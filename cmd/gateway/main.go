package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"goproxy/config"
	"goproxy/internal/admin"
	"goproxy/internal/analytics"
	"goproxy/internal/auth"
	"goproxy/internal/logger"
	"goproxy/internal/proxy"
	"goproxy/internal/ratelimit"
	"goproxy/internal/router"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
)

func main() {
	configPath := "config/config.yaml"
	if value := os.Getenv("GATEWAY_CONFIG"); value != "" {
		configPath = value
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatal("config load failed:", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	app := newGatewayApp(configPath)
	if err := app.Reload(ctx); err != nil {
		log.Fatal("gateway init failed:", err)
	}
	defer app.Close()

	if cfg.HotReload.Enabled {
		app.StartHotReload(ctx, cfg.HotReload.Interval)
	}

	srv := &http.Server{
		Addr:         cfg.Server.Address,
		Handler:      app,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	go func() {
		log.Printf("gateway listening on %s", cfg.Server.Address)
		var serveErr error
		if cfg.Server.TLS.Enabled {
			serveErr = srv.ListenAndServeTLS(cfg.Server.TLS.CertFile, cfg.Server.TLS.KeyFile)
		} else {
			serveErr = srv.ListenAndServe()
		}
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			log.Printf("gateway serve failed: %v", serveErr)
			stop()
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("gateway shutdown failed: %v", err)
	}
}

type gatewayApp struct {
	configPath string
	handler    atomic.Value

	mu           sync.Mutex
	cfg          config.Config
	db           *sql.DB
	redis        *redis.Client
	healthCancel context.CancelFunc
}

func newGatewayApp(configPath string) *gatewayApp {
	return &gatewayApp{configPath: configPath}
}

func (a *gatewayApp) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	handler, ok := a.handler.Load().(http.Handler)
	if !ok || handler == nil {
		http.Error(w, "gateway unavailable", http.StatusServiceUnavailable)
		return
	}
	handler.ServeHTTP(w, r)
}

func (a *gatewayApp) Reload(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	cfg, err := config.Load(a.configPath)
	if err != nil {
		return err
	}

	db, err := openDB(ctx, cfg)
	if err != nil {
		return err
	}
	redisClient := openRedis(cfg)

	if a.healthCancel != nil {
		a.healthCancel()
	}
	healthCtx, cancel := context.WithCancel(context.Background())

	state := newGatewayState(cfg)
	handler, err := buildHandler(healthCtx, cfg, runtimeDeps{
		db:        db,
		redis:     redisClient,
		state:     state,
		reload:    func() error { return a.Reload(context.Background()) },
		adminMode: true,
	})
	if err != nil {
		cancel()
		if db != nil {
			_ = db.Close()
		}
		if redisClient != nil {
			_ = redisClient.Close()
		}
		return err
	}

	oldDB := a.db
	oldRedis := a.redis
	a.cfg = cfg
	a.db = db
	a.redis = redisClient
	a.healthCancel = cancel
	a.handler.Store(handler)

	if oldDB != nil {
		_ = oldDB.Close()
	}
	if oldRedis != nil {
		_ = oldRedis.Close()
	}
	log.Printf("gateway config reloaded from %s", a.configPath)
	return nil
}

func (a *gatewayApp) StartHotReload(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 2 * time.Second
	}
	info, err := os.Stat(a.configPath)
	if err != nil {
		log.Printf("hot reload disabled: %v", err)
		return
	}
	lastMod := info.ModTime()

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				info, err := os.Stat(a.configPath)
				if err != nil {
					log.Printf("hot reload stat failed: %v", err)
					continue
				}
				if !info.ModTime().After(lastMod) {
					continue
				}
				if err := a.Reload(ctx); err != nil {
					log.Printf("hot reload failed: %v", err)
					continue
				}
				lastMod = info.ModTime()
			}
		}
	}()
}

func (a *gatewayApp) Close() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.healthCancel != nil {
		a.healthCancel()
	}
	if a.db != nil {
		_ = a.db.Close()
	}
	if a.redis != nil {
		_ = a.redis.Close()
	}
}

func newHandler(cfg config.Config) (http.Handler, error) {
	return buildHandler(context.Background(), cfg, runtimeDeps{
		state: newGatewayState(cfg),
	})
}

type runtimeDeps struct {
	db        *sql.DB
	redis     *redis.Client
	state     *gatewayState
	reload    func() error
	adminMode bool
}

type gatewayState struct {
	cfg       config.Config
	upstreams *proxy.UpstreamRegistry
}

func newGatewayState(cfg config.Config) *gatewayState {
	return &gatewayState{
		cfg:       cfg,
		upstreams: proxy.NewUpstreamRegistry(),
	}
}

func buildHandler(ctx context.Context, cfg config.Config, deps runtimeDeps) (http.Handler, error) {
	if deps.state == nil {
		deps.state = newGatewayState(cfg)
	}

	apiKeyStore := auth.APIKeyStore(auth.NewStaticAPIKeyStore(toAPIKeys(cfg.APIKeys)))
	if cfg.Database.Enabled && deps.db != nil {
		apiKeyStore = auth.MultiAPIKeyStore{
			auth.NewPostgresAPIKeyStore(deps.db),
			apiKeyStore,
		}
	}

	metrics := analytics.NewAggregator(10000)
	sinks := logger.MultiSink{
		logger.NewChannelSink(1024),
		metrics,
	}
	if cfg.Database.Enabled && deps.db != nil {
		dbSink := logger.NewChannelSink(4096)
		sinks = append(sinks, dbSink)
		writer := logger.NewPostgresWriter(deps.db, 100, time.Second)
		go func() {
			if err := writer.Run(ctx, dbSink.Entries()); err != nil && !errors.Is(err, context.Canceled) {
				log.Printf("postgres log writer stopped: %v", err)
			}
		}()
	}

	routeConfigs := normalizedRoutes(cfg)
	routes := make([]router.Route, 0, len(routeConfigs))
	for _, routeCfg := range routeConfigs {
		routeHandler, balanced, err := newRouteHandler(ctx, routeCfg, apiKeyStore, cfg.JWT, deps.redis)
		if err != nil {
			return nil, err
		}
		if balanced != nil {
			deps.state.upstreams.Set(routeCfg.ID, balanced)
		}

		routes = append(routes, router.Route{
			ID:          routeCfg.ID,
			PathPrefix:  routeCfg.PathPrefix,
			Methods:     routeCfg.Methods,
			StripPrefix: routeCfg.StripPrefix,
			Handler:     routeHandler,
		})
	}

	gatewayRouter := router.New(routes)
	analyticsHandler := analytics.NewHandler(metrics)
	if cfg.Analytics.Persistent && deps.db != nil {
		analyticsHandler = analytics.NewPersistentHandler(analytics.NewPostgresStore(deps.db), metrics, cfg.Analytics.Window)
	}
	adminHandler := admin.NewHandler(
		cfg.Admin.Token,
		func() []config.RouteConfig { return deps.state.cfg.Routes },
		func() map[string][]proxy.UpstreamStatus { return deps.state.upstreams.Snapshot() },
		deps.reload,
	)

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})
	if cfg.Analytics.Enabled {
		mux.Handle("/analytics/routes", analyticsHandler)
	}
	if cfg.Admin.Enabled {
		mux.Handle("/admin/routes", adminHandler)
		mux.Handle("/admin/upstreams", adminHandler)
		mux.Handle("/admin/reload", adminHandler)
	}
	mux.Handle("/dashboard", http.RedirectHandler("/dashboard/", http.StatusMovedPermanently))
	mux.Handle("/dashboard/", http.StripPrefix("/dashboard/", http.FileServer(http.Dir("web"))))
	mux.Handle("/", gatewayRouter)

	return proxy.Chain(
		mux,
		logger.Middleware(sinks),
	), nil
}

func newRouteHandler(ctx context.Context, routeCfg config.RouteConfig, apiKeyStore auth.APIKeyStore, jwtCfg config.JWTConfig, redisClient *redis.Client) (http.Handler, *proxy.BalancedProxy, error) {
	balanced, err := proxy.NewBalancedProxy(routeCfg.Upstreams, proxy.HealthCheckConfig{
		Enabled:  routeCfg.HealthCheck.Enabled,
		Path:     routeCfg.HealthCheck.Path,
		Interval: routeCfg.HealthCheck.Interval,
		Timeout:  routeCfg.HealthCheck.Timeout,
	})
	if err != nil {
		return nil, nil, err
	}
	balanced.StartHealthChecks(ctx)

	middlewares := make([]proxy.Middleware, 0, 3)
	if routeCfg.RequireAuth {
		middlewares = append(middlewares, auth.APIKeyMiddleware(apiKeyStore))
	}
	if routeCfg.RequireJWT {
		if jwtCfg.Secret == "" {
			return nil, nil, fmt.Errorf("route %q requires jwt but jwt.secret is empty", routeCfg.ID)
		}
		middlewares = append(middlewares, auth.JWTMiddleware(auth.JWTVerifier{
			Secret:   []byte(jwtCfg.Secret),
			Issuer:   jwtCfg.Issuer,
			Audience: jwtCfg.Audience,
		}))
	}
	if routeCfg.RateLimitCapacity > 0 || routeCfg.RateLimitRefillPerSec > 0 {
		if redisClient != nil {
			limiter := ratelimit.NewRedisLimiter(redisEvalClient{client: redisClient}, routeCfg.RateLimitCapacity, routeCfg.RateLimitRefillPerSec)
			middlewares = append(middlewares, ratelimit.RedisMiddleware(limiter, ratelimit.APIKeyOrIPKey))
		} else {
			limiter := ratelimit.NewLimiter(routeCfg.RateLimitCapacity, routeCfg.RateLimitRefillPerSec)
			middlewares = append(middlewares, ratelimit.Middleware(limiter, ratelimit.APIKeyOrIPKey))
		}
	}

	return proxy.Chain(balanced, middlewares...), balanced, nil
}

type redisEvalClient struct {
	client *redis.Client
}

func (c redisEvalClient) Eval(ctx context.Context, script string, keys []string, args ...any) (any, error) {
	return c.client.Eval(ctx, script, keys, args...).Result()
}

func openDB(ctx context.Context, cfg config.Config) (*sql.DB, error) {
	if !cfg.Database.Enabled {
		return nil, nil
	}
	db, err := sql.Open(cfg.Database.Driver, cfg.Database.DSN)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return db, nil
}

func openRedis(cfg config.Config) *redis.Client {
	if !cfg.Redis.Enabled {
		return nil
	}
	return redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Address,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
}

func normalizedRoutes(cfg config.Config) []config.RouteConfig {
	if len(cfg.Routes) == 0 {
		upstreams := cfg.Proxy.Upstreams
		if len(upstreams) == 0 && cfg.Proxy.Upstream != "" {
			upstreams = []string{cfg.Proxy.Upstream}
		}
		return []config.RouteConfig{{
			ID:         "default",
			PathPrefix: "/",
			Upstream:   cfg.Proxy.Upstream,
			Upstreams:  upstreams,
		}}
	}

	routes := make([]config.RouteConfig, len(cfg.Routes))
	copy(routes, cfg.Routes)
	for i := range routes {
		if routes[i].PathPrefix == "" {
			routes[i].PathPrefix = "/"
		}
		if routes[i].UpstreamEnv != "" {
			if upstream := os.Getenv(routes[i].UpstreamEnv); upstream != "" {
				routes[i].Upstream = upstream
				routes[i].Upstreams = []string{upstream}
			}
		}
		if len(routes[i].Upstreams) == 0 && routes[i].Upstream != "" {
			routes[i].Upstreams = []string{routes[i].Upstream}
		}
		if len(routes[i].Upstreams) == 0 {
			routes[i].Upstream = cfg.Proxy.Upstream
			routes[i].Upstreams = []string{cfg.Proxy.Upstream}
		}
		if routes[i].Upstream == "" && len(routes[i].Upstreams) > 0 {
			routes[i].Upstream = routes[i].Upstreams[0]
		}
	}
	return routes
}

func toAPIKeys(keys []config.APIKeyConfig) []auth.APIKey {
	converted := make([]auth.APIKey, 0, len(keys))
	for _, key := range keys {
		converted = append(converted, auth.APIKey{
			ID:     key.ID,
			Key:    key.Key,
			Tenant: key.Tenant,
		})
	}
	return converted
}
