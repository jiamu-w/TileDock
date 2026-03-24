package middleware

import (
	"testing"
	"time"
)

func TestLoginRateLimiterBlocksAfterFailures(t *testing.T) {
	limiter := NewLoginRateLimiter()
	ip := "127.0.0.1"
	username := "admin"

	for i := 0; i < loginMaxFailures; i++ {
		allowed, _ := limiter.Allow(ip, username)
		if !allowed {
			t.Fatalf("attempt %d unexpectedly blocked", i+1)
		}
		limiter.RegisterFailure(ip, username)
		clearAttemptInterval(limiter, ip, username)
	}

	allowed, _ := limiter.Allow(ip, username)
	if allowed {
		t.Fatal("expected login to be blocked after repeated failures")
	}
}

func TestLoginRateLimiterClearsOnSuccess(t *testing.T) {
	limiter := NewLoginRateLimiter()
	ip := "127.0.0.1"
	username := "admin"

	limiter.RegisterFailure(ip, username)
	limiter.RegisterSuccess(ip, username)
	clearAttemptInterval(limiter, ip, username)

	allowed, _ := limiter.Allow(ip, username)
	if !allowed {
		t.Fatal("expected login to be allowed after success reset")
	}
}

func TestLoginRateLimiterBlocksRapidRetry(t *testing.T) {
	limiter := NewLoginRateLimiter()
	ip := "127.0.0.1"
	username := "admin"

	allowed, _ := limiter.Allow(ip, username)
	if !allowed {
		t.Fatal("expected first login attempt to be allowed")
	}

	limiter.RegisterAttempt(ip, username)

	allowed, wait := limiter.Allow(ip, username)
	if allowed {
		t.Fatal("expected rapid retry to be blocked")
	}
	if wait <= 0 {
		t.Fatal("expected positive wait duration for rapid retry")
	}
}

func clearAttemptInterval(limiter *LoginRateLimiter, ip, username string) {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	for _, key := range []string{loginIPKey(ip), loginUserKey(username)} {
		attempt := limiter.attempts[key]
		attempt.lastAttempt = time.Time{}
		limiter.attempts[key] = attempt
	}
}
