package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

type TokenAuth struct {
	secret []byte
	ttl    time.Duration
}
type tokenPayload struct {
	Exp  int64  `json:"exp"`
	Role string `json:"role"`
}

func NewTokenAuth(s []byte, t time.Duration) *TokenAuth { return &TokenAuth{s, t} }
func (a *TokenAuth) Sign(role string) string {
	p := tokenPayload{time.Now().Add(a.ttl).Unix(), role}
	b, _ := json.Marshal(p)
	x := base64.RawURLEncoding.EncodeToString(b)
	m := hmac.New(sha256.New, a.secret)
	m.Write([]byte(x))
	return x + "." + base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}
func (a *TokenAuth) Verify(tok, role string) bool {
	p := strings.Split(tok, ".")
	if len(p) != 2 {
		return false
	}
	m := hmac.New(sha256.New, a.secret)
	m.Write([]byte(p[0]))
	sig, e := base64.RawURLEncoding.DecodeString(p[1])
	if e != nil || !hmac.Equal(sig, m.Sum(nil)) {
		return false
	}
	b, e := base64.RawURLEncoding.DecodeString(p[0])
	if e != nil {
		return false
	}
	var x tokenPayload
	if json.Unmarshal(b, &x) != nil {
		return false
	}
	return x.Role == role && x.Exp > time.Now().Unix()
}
func bearer(r *http.Request) string {
	if q := r.URL.Query().Get("token"); q != "" {
		return q
	}
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(h, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
	}
	return ""
}
func (a *App) requireSite(r *http.Request) bool  { return a.SiteAuth.Verify(bearer(r), "site") }
func (a *App) requireAdmin(r *http.Request) bool { return a.AdminAuth.Verify(bearer(r), "admin") }
func jsonError(w http.ResponseWriter, s int, m string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(s)
	json.NewEncoder(w).Encode(map[string]any{"error": m})
}
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
