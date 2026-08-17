package main

import (
	"github.com/gorilla/websocket"
	"net/http"
	"strings"
)

var upgrader = websocket.Upgrader{ReadBufferSize: 1024, WriteBufferSize: 4096, CheckOrigin: func(r *http.Request) bool { return true }}

func (a *App) websocket(w http.ResponseWriter, r *http.Request) {
	if !a.requireSite(r) {
		http.Error(w, "unauthorized", 401)
		return
	}
	role := r.URL.Query().Get("role")
	if role != "host" {
		role = "player"
	}
	code := strings.TrimSpace(r.URL.Query().Get("room"))
	room := a.Hub.Get(code)
	if room == nil {
		http.Error(w, "room not found", 404)
		return
	}
	conn, e := upgrader.Upgrade(w, r, nil)
	if e != nil {
		return
	}
	c := &Client{Conn: conn, SendQ: make(chan any, 32), Room: room, Role: role, App: a}
	go c.writePump()
	go c.readPump()
}
