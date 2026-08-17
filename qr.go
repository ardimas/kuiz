package main

import (
	"github.com/skip2/go-qrcode"
	"net/http"
)

func (a *App) qrHandler(w http.ResponseWriter, r *http.Request) {
	if !a.requireSite(r) {
		http.Error(w, "unauthorized", 401)
		return
	}
	code := r.URL.Query().Get("room")
	if a.Hub.Get(code) == nil {
		http.Error(w, "room not found", 404)
		return
	}
	png, e := qrcode.Encode(r.Host+"/?room="+code, qrcode.Medium, 256)
	if e != nil {
		http.Error(w, e.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Write(png)
}
