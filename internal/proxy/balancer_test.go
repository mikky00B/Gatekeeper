package proxy

import (
	"testing"
)

func TestBalancedProxyRoundRobinsHealthyUpstreams(t *testing.T) {
	p, err := NewBalancedProxy([]string{"http://a.example.test", "http://b.example.test"}, HealthCheckConfig{})
	if err != nil {
		t.Fatalf("NewBalancedProxy returned error: %v", err)
	}

	hits := map[string]int{}
	for i := 0; i < 4; i++ {
		hits[p.pick().target]++
	}

	if hits["http://a.example.test"] != 2 || hits["http://b.example.test"] != 2 {
		t.Fatalf("hits = %#v, want two hits per upstream", hits)
	}
}

func TestBalancedProxyRejectsInvalidUpstream(t *testing.T) {
	if _, err := NewBalancedProxy([]string{"localhost:3000"}, HealthCheckConfig{}); err == nil {
		t.Fatal("NewBalancedProxy returned nil error for invalid upstream")
	}
}
