package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func (a *App) siteVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, 405, "method not allowed")
		return
	}
	var x struct {
		Passcode string `json:"passcode"`
	}
	if json.NewDecoder(r.Body).Decode(&x) != nil {
		jsonError(w, 400, "invalid json")
		return
	}
	v, e := a.DB.GetSetting(r.Context(), "site_passcode")
	if e != nil {
		jsonError(w, 500, e.Error())
		return
	}
	if x.Passcode != v {
		jsonError(w, 401, "invalid site passcode")
		return
	}
	writeJSON(w, map[string]any{"token": a.SiteAuth.Sign("site")})
}
func (a *App) adminLogin(w http.ResponseWriter, r *http.Request) {
	var x struct {
		Password string `json:"password"`
	}
	json.NewDecoder(r.Body).Decode(&x)
	if os.Getenv("ADMIN_PASSWORD") == "" || x.Password != os.Getenv("ADMIN_PASSWORD") {
		jsonError(w, 401, "invalid admin password")
		return
	}
	writeJSON(w, map[string]any{"token": a.AdminAuth.Sign("admin")})
}
func (a *App) adminSettings(w http.ResponseWriter, r *http.Request) {
	if !a.requireAdmin(r) {
		jsonError(w, 401, "admin authentication required")
		return
	}
	if r.Method == http.MethodGet {
		v, e := a.DB.GetSetting(r.Context(), "site_passcode")
		if e != nil {
			jsonError(w, 500, e.Error())
			return
		}
		writeJSON(w, map[string]string{"site_passcode": v})
		return
	}
	if r.Method == http.MethodPut {
		var x struct {
			Passcode string `json:"passcode"`
		}
		json.NewDecoder(r.Body).Decode(&x)
		if len(x.Passcode) < 4 {
			jsonError(w, 400, "passcode too short")
			return
		}
		if e := a.DB.SetSetting(r.Context(), "site_passcode", x.Passcode); e != nil {
			jsonError(w, 500, e.Error())
			return
		}
		writeJSON(w, map[string]any{"ok": true})
		return
	}
	jsonError(w, 405, "method not allowed")
}
func (a *App) meta(w http.ResponseWriter, r *http.Request) {
	if !a.requireSite(r) {
		jsonError(w, 401, "site authentication required")
		return
	}
	m, e := a.DB.GetMeta(r.Context())
	if e != nil {
		jsonError(w, 500, e.Error())
		return
	}
	writeJSON(w, m)
}
func (a *App) generateQuestions(w http.ResponseWriter, r *http.Request) {
	if !a.requireAdmin(r) {
		jsonError(w, 401, "admin authentication required")
		return
	}
	if r.ParseMultipartForm(55<<20) != nil {
		jsonError(w, 400, "invalid upload")
		return
	}
	g, s := r.FormValue("grade"), r.FormValue("subject")
	sem, _ := strconv.Atoi(r.FormValue("semester"))
	ch, _ := strconv.Atoi(r.FormValue("chapter_no"))
	title := strings.TrimSpace(r.FormValue("chapter_title"))
	f, h, e := r.FormFile("file")
	if e != nil {
		jsonError(w, 400, "file required")
		return
	}
	defer f.Close()
	if h.Size > 50<<20 {
		jsonError(w, 400, "file too large")
		return
	}
	data, e := io.ReadAll(io.LimitReader(f, 50<<20+1))
	if e != nil {
		jsonError(w, 400, "read failed")
		return
	}
	ext := strings.ToLower(filepath.Ext(h.Filename))
	if ext != ".pdf" && ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
		jsonError(w, 400, "unsupported file")
		return
	}
	qs, e := a.AI.Generate(r.Context(), data, h.Filename, g, s, sem, ch, title)
	if e != nil {
		jsonError(w, 502, fmt.Sprintf("AI: %v", e))
		return
	}
	if e = a.DB.InsertQuestions(r.Context(), g, s, sem, ch, title, h.Filename, qs); e != nil {
		jsonError(w, 500, e.Error())
		return
	}
	writeJSON(w, map[string]any{"ok": true, "count": len(qs)})
}
