package main

import (
	"net/http"
	"os"
	"sync"
)

type PasscodeCache struct {
	sync.RWMutex
	code string
}

var globalPasscode = &PasscodeCache{}

func initPasscodeCache() {
	globalPasscode.Lock()
	defer globalPasscode.Unlock()
	globalPasscode.code = getSitePasscodeFromDB()
}

func getPasscode() string {
	globalPasscode.RLock()
	defer globalPasscode.RUnlock()
	return globalPasscode.code
}

func setPasscode(newCode string) {
	globalPasscode.Lock()
	defer globalPasscode.Unlock()
	globalPasscode.code = newCode
}

func adminAuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		adminPass := os.Getenv("ADMIN_PASSWORD")
		if adminPass == "" {
			adminPass = "admin123" // Default fallback jika env var lupa diisi
		}

		user, pass, ok := r.BasicAuth()
		if !ok || pass != adminPass || user != "admin" {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted Admin Area"`)
			http.Error(w, "Akses Admin Ditolak", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func validateSitePasscode(pass string) bool {
	return pass == getPasscode()
}
