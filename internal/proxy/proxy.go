package proxy

import (
	"errors"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"
)

var errInvalidTarget = errors.New("target must include scheme and host")

type Proxy struct {
	rp *httputil.ReverseProxy
}

func New(target string) (*Proxy, error) {
	return NewWithTransport(target, nil)
}

func NewWithTransport(target string, transport http.RoundTripper) (*Proxy, error) {
	targetURL, err := url.Parse(target)
	if err != nil {
		return nil, err
	}
	if targetURL.Scheme == "" || targetURL.Host == "" {
		return nil, &url.Error{Op: "parse", URL: target, Err: errInvalidTarget}
	}

	rp := httputil.NewSingleHostReverseProxy(targetURL)

	// Custom transport — controls timeouts and connection pooling
	if transport == nil {
		transport = &http.Transport{
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   10,
			IdleConnTimeout:       90 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
		}
	}
	rp.Transport = transport

	// Modify the outgoing request before it hits upstream
	rp.Director = func(req *http.Request) {
		originalHost := req.Host
		req.URL.Scheme = targetURL.Scheme
		req.URL.Host = targetURL.Host
		req.Host = targetURL.Host

		// Strip internal headers before forwarding
		req.Header.Del("X-Api-Key")

		// Tell the upstream who the real client is
		req.Header.Set("X-Forwarded-For", req.RemoteAddr)
		req.Header.Set("X-Forwarded-Host", originalHost)
		if req.TLS == nil {
			req.Header.Set("X-Forwarded-Proto", "http")
		} else {
			req.Header.Set("X-Forwarded-Proto", "https")
		}
	}

	// Handle upstream errors — don't expose raw error to client
	rp.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		http.Error(w, "upstream unavailable", http.StatusBadGateway)
	}

	return &Proxy{rp: rp}, nil
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.rp.ServeHTTP(w, r)
}
