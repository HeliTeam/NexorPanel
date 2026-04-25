package middleware

import (
	"sync"
	"time"
)

type loginAttempt struct {
	failCount   int
	lockedUntil time.Time
}

var loginMu sync.Mutex
var loginState = map[string]*loginAttempt{}

const (
	loginMaxFailures = 5
	loginLockout     = 15 * time.Minute
)

func loginKey(ip, nickname string) string {
	return ip + "|" + nickname
}

// IsLoginLocked returns true if the IP+nickname pair is temporarily blocked.
func IsLoginLocked(ip, nickname string) (locked bool, retryAfterSec int) {
	loginMu.Lock()
	defer loginMu.Unlock()
	a, ok := loginState[loginKey(ip, nickname)]
	if !ok {
		return false, 0
	}
	if time.Now().Before(a.lockedUntil) {
		return true, int(time.Until(a.lockedUntil).Round(time.Second).Seconds())
	}
	return false, 0
}

// RecordLoginFailure increments failures and may apply lockout.
func RecordLoginFailure(ip, nickname string) (locked bool, retryAfterSec int) {
	loginMu.Lock()
	defer loginMu.Unlock()
	k := loginKey(ip, nickname)
	a, ok := loginState[k]
	if !ok {
		a = &loginAttempt{}
		loginState[k] = a
	}
	now := time.Now()
	if now.Before(a.lockedUntil) {
		return true, int(time.Until(a.lockedUntil).Round(time.Second).Seconds())
	}
	if now.After(a.lockedUntil) && !a.lockedUntil.IsZero() {
		a.failCount = 0
		a.lockedUntil = time.Time{}
	}
	a.failCount++
	if a.failCount >= loginMaxFailures {
		a.lockedUntil = now.Add(loginLockout)
		a.failCount = 0
		return true, int(loginLockout.Seconds())
	}
	return false, 0
}

// ResetLoginFailures clears counters after a successful login.
func ResetLoginFailures(ip, nickname string) {
	loginMu.Lock()
	defer loginMu.Unlock()
	delete(loginState, loginKey(ip, nickname))
}
