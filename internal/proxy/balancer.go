package proxy

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"
)

type HealthCheckConfig struct {
	Enabled  bool
	Path     string
	Interval time.Duration
	Timeout  time.Duration
}

type BalancedProxy struct {
	upstreams []*upstream
	next      uint64
	client    *http.Client
	health    HealthCheckConfig
}

type upstream struct {
	target  string
	health  string
	proxy   *Proxy
	healthy atomic.Bool
}

func NewBalancedProxy(targets []string, health HealthCheckConfig) (*BalancedProxy, error) {
	if len(targets) == 0 {
		return nil, fmt.Errorf("at least one upstream is required")
	}

	upstreams := make([]*upstream, 0, len(targets))
	seen := make(map[string]struct{}, len(targets))
	for _, target := range targets {
		if _, ok := seen[target]; ok {
			continue
		}
		seen[target] = struct{}{}

		p, err := New(target)
		if err != nil {
			return nil, err
		}
		healthURL, err := healthURL(target, health.Path)
		if err != nil {
			return nil, err
		}
		item := &upstream{
			target: target,
			health: healthURL,
			proxy:  p,
		}
		item.healthy.Store(true)
		upstreams = append(upstreams, item)
	}
	if len(upstreams) == 0 {
		return nil, fmt.Errorf("at least one upstream is required")
	}

	timeout := health.Timeout
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	return &BalancedProxy{
		upstreams: upstreams,
		client: &http.Client{
			Timeout: timeout,
		},
		health: health,
	}, nil
}

func (p *BalancedProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	upstream := p.pick()
	if upstream == nil {
		http.Error(w, "no healthy upstreams", http.StatusBadGateway)
		return
	}
	upstream.proxy.ServeHTTP(w, r)
}

func (p *BalancedProxy) StartHealthChecks(ctx context.Context) {
	if p == nil || !p.health.Enabled {
		return
	}
	interval := p.health.Interval
	if interval <= 0 {
		interval = 5 * time.Second
	}

	for _, upstream := range p.upstreams {
		go p.checkLoop(ctx, upstream, interval)
	}
}

func (p *BalancedProxy) Upstreams() []UpstreamStatus {
	if p == nil {
		return nil
	}
	status := make([]UpstreamStatus, 0, len(p.upstreams))
	for _, upstream := range p.upstreams {
		status = append(status, UpstreamStatus{
			Target:  upstream.target,
			Healthy: upstream.healthy.Load(),
		})
	}
	return status
}

type UpstreamStatus struct {
	Target  string `json:"target"`
	Healthy bool   `json:"healthy"`
}

func (p *BalancedProxy) pick() *upstream {
	if p == nil || len(p.upstreams) == 0 {
		return nil
	}

	start := atomic.AddUint64(&p.next, 1)
	for i := 0; i < len(p.upstreams); i++ {
		idx := int((start + uint64(i)) % uint64(len(p.upstreams)))
		candidate := p.upstreams[idx]
		if candidate.healthy.Load() {
			return candidate
		}
	}
	return nil
}

func (p *BalancedProxy) checkLoop(ctx context.Context, upstream *upstream, interval time.Duration) {
	p.check(ctx, upstream)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.check(ctx, upstream)
		}
	}
}

func (p *BalancedProxy) check(ctx context.Context, upstream *upstream) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, upstream.health, nil)
	if err != nil {
		upstream.healthy.Store(false)
		return
	}
	res, err := p.client.Do(req)
	if err != nil {
		upstream.healthy.Store(false)
		return
	}
	defer res.Body.Close()
	upstream.healthy.Store(res.StatusCode >= 200 && res.StatusCode < 500)
}

func healthURL(target string, path string) (string, error) {
	if path == "" {
		path = "/healthz"
	}
	parsed, err := url.Parse(target)
	if err != nil {
		return "", err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", &url.Error{Op: "parse", URL: target, Err: errInvalidTarget}
	}
	parsed.Path = path
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

type UpstreamRegistry struct {
	mu     sync.RWMutex
	routes map[string]*BalancedProxy
}

func NewUpstreamRegistry() *UpstreamRegistry {
	return &UpstreamRegistry{routes: make(map[string]*BalancedProxy)}
}

func (r *UpstreamRegistry) Set(routeID string, proxy *BalancedProxy) {
	if r == nil || proxy == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.routes[routeID] = proxy
}

func (r *UpstreamRegistry) Snapshot() map[string][]UpstreamStatus {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()

	snapshot := make(map[string][]UpstreamStatus, len(r.routes))
	for routeID, proxy := range r.routes {
		snapshot[routeID] = proxy.Upstreams()
	}
	return snapshot
}
