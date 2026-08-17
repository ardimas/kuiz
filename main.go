package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

type App struct {
	DB        *DB
	AI        *AIService
	Hub       *Hub
	SiteAuth  *TokenAuth
	AdminAuth *TokenAuth
}

func env(k, f string) string {
	v := strings.TrimSpace(os.Getenv(k))
	if v == "" {
		return f
	}
	return v
}
func main() {
	ctx := context.Background()
	db, e := NewDB(ctx, env("DATABASE_URL", ""))
	if e != nil {
		log.Fatal(e)
	}
	defer db.Close()
	sec := os.Getenv("AUTH_SECRET")
	if sec == "" {
		b := make([]byte, 32)
		rand.Read(b)
		sec = hex.EncodeToString(b)
		log.Println("WARNING AUTH_SECRET not set; ephemeral secret")
	}
	a := &App{DB: db, AI: NewAIService(os.Getenv("GEMINI_API_KEY")), Hub: NewHub(), SiteAuth: NewTokenAuth([]byte(sec), 24*time.Hour), AdminAuth: NewTokenAuth([]byte(sec), 8*time.Hour)}
	a.Hub.StartJanitor()
	m := http.NewServeMux()
	m.HandleFunc("/healthz", a.health)
	m.HandleFunc("/api/site/verify", a.siteVerify)
	m.HandleFunc("/api/admin/login", a.adminLogin)
	m.HandleFunc("/api/admin/settings", a.adminSettings)
	m.HandleFunc("/api/admin/generate", a.generateQuestions)
	m.HandleFunc("/api/meta", a.meta)
	m.HandleFunc("/api/host/rooms", a.createRoom)
	m.HandleFunc("/api/host/rooms/", a.roomAPI)
	m.HandleFunc("/api/room/qr", a.qrHandler)
	m.HandleFunc("/ws", a.websocket)
	m.Handle("/", http.FileServer(http.Dir("./web")))
	p := env("PORT", "8080")
	log.Printf("listening on :%s", p)
	log.Fatal(http.ListenAndServe(":"+p, m))
}
func (a *App) health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"ok": true, "time": time.Now().UTC()})
}
