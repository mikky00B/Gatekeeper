package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	defaultAddress  = ":8080"
	defaultUpstream = "http://localhost:3000"
)

type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Proxy     ProxyConfig     `yaml:"proxy"`
	Database  DatabaseConfig  `yaml:"database"`
	Redis     RedisConfig     `yaml:"redis"`
	JWT       JWTConfig       `yaml:"jwt"`
	Analytics AnalyticsConfig `yaml:"analytics"`
	Admin     AdminConfig     `yaml:"admin"`
	HotReload HotReloadConfig `yaml:"hot_reload"`
	Routes    []RouteConfig   `yaml:"routes"`
	APIKeys   []APIKeyConfig  `yaml:"api_keys"`
}

type ServerConfig struct {
	Address         string        `yaml:"address"`
	ReadTimeout     time.Duration `yaml:"read_timeout"`
	WriteTimeout    time.Duration `yaml:"write_timeout"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
	TLS             TLSConfig     `yaml:"tls"`
}

type TLSConfig struct {
	Enabled  bool   `yaml:"enabled"`
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
}

type ProxyConfig struct {
	Upstream  string   `yaml:"upstream"`
	Upstreams []string `yaml:"upstreams"`
}

type DatabaseConfig struct {
	Enabled bool   `yaml:"enabled"`
	Driver  string `yaml:"driver"`
	DSN     string `yaml:"dsn"`
	DSNEnv  string `yaml:"dsn_env"`
	DSNFile string `yaml:"dsn_file"`
}

type RedisConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Address  string `yaml:"address"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

type JWTConfig struct {
	Secret     string `yaml:"secret"`
	SecretEnv  string `yaml:"secret_env"`
	SecretFile string `yaml:"secret_file"`
	Issuer     string `yaml:"issuer"`
	Audience   string `yaml:"audience"`
}

type AnalyticsConfig struct {
	Enabled    bool          `yaml:"enabled"`
	Persistent bool          `yaml:"persistent"`
	Window     time.Duration `yaml:"window"`
}

type AdminConfig struct {
	Enabled   bool   `yaml:"enabled"`
	Token     string `yaml:"token"`
	TokenEnv  string `yaml:"token_env"`
	TokenFile string `yaml:"token_file"`
}

type HotReloadConfig struct {
	Enabled  bool          `yaml:"enabled"`
	Interval time.Duration `yaml:"interval"`
}

type HealthCheckConfig struct {
	Enabled  bool          `yaml:"enabled"`
	Path     string        `yaml:"path"`
	Interval time.Duration `yaml:"interval"`
	Timeout  time.Duration `yaml:"timeout"`
}

type RouteConfig struct {
	ID                    string            `yaml:"id"`
	PathPrefix            string            `yaml:"path_prefix"`
	Methods               []string          `yaml:"methods"`
	Upstream              string            `yaml:"upstream"`
	UpstreamEnv           string            `yaml:"upstream_env"`
	Upstreams             []string          `yaml:"upstreams"`
	StripPrefix           bool              `yaml:"strip_prefix"`
	RequireAuth           bool              `yaml:"require_auth"`
	RequireJWT            bool              `yaml:"require_jwt"`
	RateLimitCapacity     int               `yaml:"rate_limit_capacity"`
	RateLimitRefillPerSec float64           `yaml:"rate_limit_refill_per_second"`
	HealthCheck           HealthCheckConfig `yaml:"health_check"`
}

type APIKeyConfig struct {
	ID        string `yaml:"id"`
	Key       string `yaml:"key"`
	KeyEnv    string `yaml:"key_env"`
	KeyFile   string `yaml:"key_file"`
	Tenant    string `yaml:"tenant"`
	PlainText string `yaml:"-"`
}

func Default() Config {
	return Config{
		Server: ServerConfig{
			Address:         defaultAddress,
			ReadTimeout:     10 * time.Second,
			WriteTimeout:    30 * time.Second,
			ShutdownTimeout: 10 * time.Second,
		},
		Proxy: ProxyConfig{Upstream: defaultUpstream},
		Redis: RedisConfig{Address: "localhost:6379"},
		Analytics: AnalyticsConfig{
			Window: time.Hour,
		},
		HotReload: HotReloadConfig{
			Interval: 2 * time.Second,
		},
	}
}

func Load(path string) (Config, error) {
	cfg := Default()

	if path != "" {
		if err := loadFile(path, &cfg); err != nil {
			return Config{}, err
		}
	}

	applyEnvOverrides(&cfg)
	if err := resolveSecrets(&cfg); err != nil {
		return Config{}, err
	}
	if err := normalize(&cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func loadFile(path string, cfg *Config) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("open config: %w", err)
	}
	if err := yaml.Unmarshal(content, cfg); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}
	return nil
}

func applyEnvOverrides(cfg *Config) {
	if address := strings.TrimSpace(os.Getenv("GATEWAY_ADDR")); address != "" {
		cfg.Server.Address = address
	}
	if upstream := strings.TrimSpace(os.Getenv("GATEWAY_UPSTREAM")); upstream != "" {
		cfg.Proxy.Upstream = upstream
	}
	if driver := strings.TrimSpace(os.Getenv("GATEWAY_DB_DRIVER")); driver != "" {
		cfg.Database.Driver = driver
	}
	if dsn := strings.TrimSpace(os.Getenv("GATEWAY_DB_DSN")); dsn != "" {
		cfg.Database.DSN = dsn
	}
	if enabled, ok := boolEnv("GATEWAY_DB_ENABLED"); ok {
		cfg.Database.Enabled = enabled
	}
	if secret := strings.TrimSpace(os.Getenv("GATEWAY_JWT_SECRET")); secret != "" {
		cfg.JWT.Secret = secret
	}
	if redisAddr := strings.TrimSpace(os.Getenv("GATEWAY_REDIS_ADDR")); redisAddr != "" {
		cfg.Redis.Address = redisAddr
	}
	if enabled, ok := boolEnv("GATEWAY_REDIS_ENABLED"); ok {
		cfg.Redis.Enabled = enabled
	}
	if adminToken := strings.TrimSpace(os.Getenv("GATEWAY_ADMIN_TOKEN")); adminToken != "" {
		cfg.Admin.Token = adminToken
	}
	if enabled, ok := boolEnv("GATEWAY_ADMIN_ENABLED"); ok {
		cfg.Admin.Enabled = enabled
	}
	if enabled, ok := boolEnv("GATEWAY_ANALYTICS_PERSISTENT"); ok {
		cfg.Analytics.Persistent = enabled
	}
}

func boolEnv(name string) (bool, bool) {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(name)))
	switch value {
	case "":
		return false, false
	case "1", "true", "yes", "on":
		return true, true
	case "0", "false", "no", "off":
		return false, true
	default:
		return false, false
	}
}

func resolveSecrets(cfg *Config) error {
	var err error
	if cfg.Database.DSN, err = resolveSecret(cfg.Database.DSN, cfg.Database.DSNEnv, cfg.Database.DSNFile); err != nil {
		return fmt.Errorf("database dsn: %w", err)
	}
	if cfg.JWT.Secret, err = resolveSecret(cfg.JWT.Secret, cfg.JWT.SecretEnv, cfg.JWT.SecretFile); err != nil {
		return fmt.Errorf("jwt secret: %w", err)
	}
	if cfg.Admin.Token, err = resolveSecret(cfg.Admin.Token, cfg.Admin.TokenEnv, cfg.Admin.TokenFile); err != nil {
		return fmt.Errorf("admin token: %w", err)
	}
	for i := range cfg.APIKeys {
		if cfg.APIKeys[i].Key, err = resolveSecret(cfg.APIKeys[i].Key, cfg.APIKeys[i].KeyEnv, cfg.APIKeys[i].KeyFile); err != nil {
			return fmt.Errorf("api key %q: %w", cfg.APIKeys[i].ID, err)
		}
		cfg.APIKeys[i].PlainText = cfg.APIKeys[i].Key
	}
	return nil
}

func resolveSecret(value, envName, filePath string) (string, error) {
	if envName != "" {
		value = os.Getenv(envName)
	}
	if filePath != "" {
		content, err := os.ReadFile(filePath)
		if err != nil {
			return "", err
		}
		value = strings.TrimSpace(string(content))
	}
	return value, nil
}

func normalize(cfg *Config) error {
	if strings.TrimSpace(cfg.Server.Address) == "" {
		return errors.New("server address is required")
	}
	if cfg.Server.ShutdownTimeout <= 0 {
		cfg.Server.ShutdownTimeout = 10 * time.Second
	}
	if cfg.Server.TLS.Enabled && (cfg.Server.TLS.CertFile == "" || cfg.Server.TLS.KeyFile == "") {
		return errors.New("tls cert_file and key_file are required when tls is enabled")
	}

	if strings.TrimSpace(cfg.Proxy.Upstream) == "" && len(cfg.Proxy.Upstreams) == 0 {
		cfg.Proxy.Upstream = defaultUpstream
	}
	if strings.TrimSpace(cfg.Proxy.Upstream) == "" && len(cfg.Proxy.Upstreams) > 0 {
		cfg.Proxy.Upstream = cfg.Proxy.Upstreams[0]
	}

	if cfg.Database.Enabled {
		if cfg.Database.Driver == "" {
			cfg.Database.Driver = "postgres"
		}
		if cfg.Database.Driver != "postgres" {
			return fmt.Errorf("unsupported database driver %q", cfg.Database.Driver)
		}
		if cfg.Database.DSN == "" {
			return errors.New("database dsn is required when database is enabled")
		}
	}
	if cfg.Redis.Enabled && cfg.Redis.Address == "" {
		return errors.New("redis address is required when redis is enabled")
	}
	if cfg.Analytics.Persistent && !cfg.Database.Enabled {
		return errors.New("persistent analytics require database.enabled")
	}
	if cfg.Admin.Enabled && cfg.Admin.Token == "" {
		return errors.New("admin token, token_env, or token_file is required when admin is enabled")
	}
	if cfg.HotReload.Enabled && cfg.HotReload.Interval <= 0 {
		cfg.HotReload.Interval = 2 * time.Second
	}

	if len(cfg.Routes) == 0 {
		cfg.Routes = []RouteConfig{{
			ID:         "default",
			PathPrefix: "/",
			Upstream:   cfg.Proxy.Upstream,
			Upstreams:  cfg.Proxy.Upstreams,
		}}
	}
	for i := range cfg.Routes {
		route := &cfg.Routes[i]
		if strings.TrimSpace(route.ID) == "" {
			route.ID = fmt.Sprintf("route-%d", i+1)
		}
		if strings.TrimSpace(route.PathPrefix) == "" {
			route.PathPrefix = "/"
		}
		route.Methods = normalizeMethods(route.Methods)
		if route.UpstreamEnv != "" {
			if upstream := strings.TrimSpace(os.Getenv(route.UpstreamEnv)); upstream != "" {
				route.Upstream = upstream
				route.Upstreams = []string{upstream}
			}
		}
		if len(route.Upstreams) == 0 && strings.TrimSpace(route.Upstream) != "" {
			route.Upstreams = []string{route.Upstream}
		}
		if len(route.Upstreams) == 0 {
			route.Upstream = cfg.Proxy.Upstream
			route.Upstreams = append(route.Upstreams, cfg.Proxy.Upstream)
			route.Upstreams = append(route.Upstreams, cfg.Proxy.Upstreams...)
		}
		if route.Upstream == "" && len(route.Upstreams) > 0 {
			route.Upstream = route.Upstreams[0]
		}
		route.Upstreams = compactStrings(route.Upstreams)
		if len(route.Upstreams) == 0 {
			return fmt.Errorf("route %q requires at least one upstream", route.ID)
		}
		if route.HealthCheck.Enabled {
			if route.HealthCheck.Path == "" {
				route.HealthCheck.Path = "/healthz"
			}
			if route.HealthCheck.Interval <= 0 {
				route.HealthCheck.Interval = 5 * time.Second
			}
			if route.HealthCheck.Timeout <= 0 {
				route.HealthCheck.Timeout = 2 * time.Second
			}
		}
	}
	return nil
}

func normalizeMethods(methods []string) []string {
	out := make([]string, 0, len(methods))
	for _, method := range methods {
		for _, part := range strings.Split(method, ",") {
			part = strings.ToUpper(strings.TrimSpace(part))
			if part != "" {
				out = append(out, part)
			}
		}
	}
	return out
}

func compactStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func (r *RouteConfig) UnmarshalYAML(value *yaml.Node) error {
	var aux struct {
		ID                    string            `yaml:"id"`
		PathPrefix            string            `yaml:"path_prefix"`
		Methods               any               `yaml:"methods"`
		Upstream              string            `yaml:"upstream"`
		UpstreamEnv           string            `yaml:"upstream_env"`
		Upstreams             []string          `yaml:"upstreams"`
		StripPrefix           bool              `yaml:"strip_prefix"`
		RequireAuth           bool              `yaml:"require_auth"`
		RequireJWT            bool              `yaml:"require_jwt"`
		RateLimitCapacity     int               `yaml:"rate_limit_capacity"`
		RateLimitRefillPerSec float64           `yaml:"rate_limit_refill_per_second"`
		HealthCheck           HealthCheckConfig `yaml:"health_check"`
	}
	if err := value.Decode(&aux); err != nil {
		return err
	}
	*r = RouteConfig{
		ID:                    aux.ID,
		PathPrefix:            aux.PathPrefix,
		Upstream:              aux.Upstream,
		UpstreamEnv:           aux.UpstreamEnv,
		Upstreams:             aux.Upstreams,
		StripPrefix:           aux.StripPrefix,
		RequireAuth:           aux.RequireAuth,
		RequireJWT:            aux.RequireJWT,
		RateLimitCapacity:     aux.RateLimitCapacity,
		RateLimitRefillPerSec: aux.RateLimitRefillPerSec,
		HealthCheck:           aux.HealthCheck,
	}
	switch methods := aux.Methods.(type) {
	case nil:
	case string:
		r.Methods = []string{methods}
	case []any:
		r.Methods = make([]string, 0, len(methods))
		for _, method := range methods {
			r.Methods = append(r.Methods, fmt.Sprint(method))
		}
	default:
		return fmt.Errorf("methods must be a string or list")
	}
	return nil
}
