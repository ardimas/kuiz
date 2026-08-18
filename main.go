package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func main() {
	initDB()
	initPasscodeCache()

	hub := newHub()
	go hub.run()

	// Static Files Frontend
	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	// Routes HTML
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./static/index.html")
	})
	http.HandleFunc("/host", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./static/host.html")
	})
	http.HandleFunc("/admin", adminAuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./static/admin.html")
	}))

	// API Verify Passcode
	http.HandleFunc("/api/verify-passcode", func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Query().Get("key")
		if validateSitePasscode(key) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"valid":true}`))
		} else {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"valid":false}`))
		}
	})

	// API Admin: Update Site Passcode
	http.HandleFunc("/api/admin/update-passcode", adminAuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Passcode string `json:"passcode"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		if req.Passcode == "" {
			http.Error(w, "Passcode tidak boleh kosong", http.StatusBadRequest)
			return
		}

		err := updateSitePasscodeInDB(req.Passcode)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		setPasscode(req.Passcode)
		w.Write([]byte(`{"status":"success"}`))
	}))

	// API Admin: Upload & Auto Generate Gemini Quiz
	http.HandleFunc("/api/admin/upload-scan", adminAuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		r.ParseMultipartForm(10 << 20) // Limit 10MB
		file, header, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "File upload error", http.StatusBadRequest)
			return
		}
		defer file.Close()

		fileBytes, err := io.ReadAll(file)
		if err != nil {
			http.Error(w, "Gagal membaca file", http.StatusInternalServerError)
			return
		}

		sem, _ := strconv.Atoi(r.FormValue("semester"))
		chap, _ := strconv.Atoi(r.FormValue("chapter_num"))

		meta := Soal{
			Grade:        r.FormValue("grade"),
			Subject:      r.FormValue("subject"),
			Semester:     sem,
			ChapterNum:   chap,
			ChapterTitle: r.FormValue("chapter_title"),
		}

		mimeType := header.Header.Get("Content-Type")
		if mimeType == "" {
			mimeType = "image/jpeg"
		}

		soalList, err := processScanWithGemini(context.Background(), fileBytes, mimeType, meta)
		if err != nil {
			http.Error(w, "Gemini AI Error: "+err.Error(), http.StatusInternalServerError)
			return
		}

		err = saveSoalBatch(soalList)
		if err != nil {
			http.Error(w, "Gagal simpan ke DB: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "success",
			"count":  len(soalList),
			"data":   soalList,
		})
	}))

	// WebSocket Endpoint
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println("Upgrade error:", err)
			return
		}
		client := &Client{hub: hub, conn: conn, send: make(chan []byte, 256)}
		hub.register <- client

		go client.writePump()
		go client.readPump()
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server berjalan di port :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
