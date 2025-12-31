package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimiter_Allow(t *testing.T) {
	rl := NewRateLimiter(5, time.Second, 5)

	// First 5 requests should be allowed
	for i := 0; i < 5; i++ {
		if !rl.Allow("192.168.1.1") {
			t.Errorf("request %d should be allowed", i+1)
		}
	}

	// 6th request should be denied
	if rl.Allow("192.168.1.1") {
		t.Error("6th request should be denied")
	}
}

func TestRateLimiter_DifferentIPs(t *testing.T) {
	rl := NewRateLimiter(2, time.Second, 2)

	// IP1 uses up its limit
	rl.Allow("192.168.1.1")
	rl.Allow("192.168.1.1")
	if rl.Allow("192.168.1.1") {
		t.Error("IP1 should be rate limited")
	}

	// IP2 should still be allowed
	if !rl.Allow("192.168.1.2") {
		t.Error("IP2 should be allowed")
	}
}

func TestRateLimiter_Refill(t *testing.T) {
	rl := NewRateLimiter(10, 50*time.Millisecond, 2)

	// Use up the bucket
	rl.Allow("192.168.1.1")
	rl.Allow("192.168.1.1")
	if rl.Allow("192.168.1.1") {
		t.Error("should be rate limited")
	}

	// Wait for refill
	time.Sleep(100 * time.Millisecond)

	// Should be allowed again
	if !rl.Allow("192.168.1.1") {
		t.Error("should be allowed after refill")
	}
}

func TestRateLimiter_Middleware(t *testing.T) {
	rl := NewRateLimiter(2, time.Second, 2)

	handler := rl.Middleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// First 2 requests should succeed
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/api/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		w := httptest.NewRecorder()

		handler(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("request %d: expected 200, got %d", i+1, w.Code)
		}
	}

	// 3rd request should be rate limited
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", w.Code)
	}

	retryAfter := w.Header().Get("Retry-After")
	if retryAfter == "" {
		t.Error("expected Retry-After header")
	}
}

func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		xff        string
		xri        string
		expected   string
	}{
		{
			name:       "Remote addr only",
			remoteAddr: "192.168.1.1:12345",
			expected:   "192.168.1.1",
		},
		{
			name:       "X-Forwarded-For single",
			remoteAddr: "127.0.0.1:12345",
			xff:        "203.0.113.195",
			expected:   "203.0.113.195",
		},
		{
			name:       "X-Forwarded-For chain",
			remoteAddr: "127.0.0.1:12345",
			xff:        "203.0.113.195, 70.41.3.18, 150.172.238.178",
			expected:   "203.0.113.195",
		},
		{
			name:       "X-Real-IP",
			remoteAddr: "127.0.0.1:12345",
			xri:        "203.0.113.195",
			expected:   "203.0.113.195",
		},
		{
			name:       "X-Forwarded-For takes precedence",
			remoteAddr: "127.0.0.1:12345",
			xff:        "203.0.113.195",
			xri:        "70.41.3.18",
			expected:   "203.0.113.195",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.xff != "" {
				req.Header.Set("X-Forwarded-For", tt.xff)
			}
			if tt.xri != "" {
				req.Header.Set("X-Real-IP", tt.xri)
			}

			ip := getClientIP(req)
			if ip != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, ip)
			}
		})
	}
}
