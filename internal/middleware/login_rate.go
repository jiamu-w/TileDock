package middleware

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	loginMaxFailures  = 5
	loginWindow       = 10 * time.Minute
	loginLockDuration = 15 * time.Minute
	loginMinInterval  = 2 * time.Second
)

type loginAttempt struct {
	count       int
	windowEnds  time.Time
	lockedUntil time.Time
	lastAttempt time.Time
}

// LoginRateLimiter guards the login endpoint against brute-force attempts.
type LoginRateLimiter struct {
	mu       sync.Mutex
	attempts map[string]loginAttempt
}

// NewLoginRateLimiter creates a rate limiter.
func NewLoginRateLimiter() *LoginRateLimiter {
	return &LoginRateLimiter{
		attempts: make(map[string]loginAttempt),
	}
}

// Allow reports whether the current login attempt is allowed.
func (l *LoginRateLimiter) Allow(ip, username string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	for _, key := range []string{loginIPKey(ip), loginUserKey(username)} {
		attempt := l.attempts[key]
		if attempt.lockedUntil.After(now) {
			return false, time.Until(attempt.lockedUntil)
		}
		if !attempt.lastAttempt.IsZero() && now.Sub(attempt.lastAttempt) < loginMinInterval {
			return false, loginMinInterval - now.Sub(attempt.lastAttempt)
		}
		if !attempt.windowEnds.IsZero() && now.After(attempt.windowEnds) {
			delete(l.attempts, key)
		}
	}

	return true, 0
}

// RegisterAttempt records that a login attempt was made.
func (l *LoginRateLimiter) RegisterAttempt(ip, username string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	for _, key := range []string{loginIPKey(ip), loginUserKey(username)} {
		attempt := l.attempts[key]
		attempt.lastAttempt = now
		if attempt.windowEnds.IsZero() || now.After(attempt.windowEnds) {
			attempt.windowEnds = now.Add(loginWindow)
		}
		l.attempts[key] = attempt
	}
}

// RegisterFailure records a failed login attempt.
func (l *LoginRateLimiter) RegisterFailure(ip, username string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	for _, key := range []string{loginIPKey(ip), loginUserKey(username)} {
		attempt := l.attempts[key]
		if attempt.windowEnds.IsZero() || now.After(attempt.windowEnds) {
			attempt = loginAttempt{
				count:      0,
				windowEnds: now.Add(loginWindow),
			}
		}

		attempt.count++
		attempt.lastAttempt = now
		if attempt.count >= loginMaxFailures {
			attempt.lockedUntil = now.Add(loginLockDuration)
		}
		l.attempts[key] = attempt
	}
}

// RegisterSuccess clears counters after a successful login.
func (l *LoginRateLimiter) RegisterSuccess(ip, username string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	for _, key := range []string{loginIPKey(ip), loginUserKey(username)} {
		attempt := l.attempts[key]
		attempt.count = 0
		attempt.windowEnds = time.Time{}
		attempt.lockedUntil = time.Time{}
		attempt.lastAttempt = now
		l.attempts[key] = attempt
	}
}

func loginIPKey(ip string) string {
	return fmt.Sprintf("ip:%s", strings.TrimSpace(ip))
}

func loginUserKey(username string) string {
	return fmt.Sprintf("user:%s", strings.ToLower(strings.TrimSpace(username)))
}
