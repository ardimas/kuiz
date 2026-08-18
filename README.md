# 🎮 Kuis Interaktif Realtime (Golang + Gemini AI + Supabase)

Aplikasi web game kuis interaktif realtime berbasis web (mirip Kahoot!) yang dibuat khusus untuk membantu proses belajar anak secara menyenangkan. Aplikasi ini menggunakan **Golang** sebagai backend realtime (WebSocket), **Supabase (PostgreSQL)** sebagai database bank soal, dan **Google Gemini API** untuk secara otomatis membaca materi scan buku (Foto/PDF) dan mengubahnya menjadi soal pilihan ganda.

---

## 🚀 Fitur Utama

- **🤖 Auto-Generate Soal via Gemini AI (Multimodal):** Cukup upload foto atau file PDF scan buku pelajaran sekolah di halaman Admin, Gemini 1.5 Flash akan menganalisis isi materi dan otomatis menghasilkan soal kuis pilihan ganda.
- **📚 Hirarki Materi Terstruktur:** Soal dikelompokkan berdasarkan **Kelas (Grade)**, **Mata Pelajaran**, **Semester (1/2)**, dan **Bab (Chapter)**.
- **🎯 Mode Filter Host Fleksibel:** Host dapat membuat room berdasarkan:
  - Per Bab spesifik
  - Seluruh Bab Semester 1
  - Seluruh Bab Semester 2
  - Full Year (Semester 1 & 2)
- **⚡ Realtime Multi-Room Engine:** Menggunakan Golang Goroutines & Channels via WebSocket dengan sinkronisasi timer dan kalkulasi skor presisi (milidetik).
- **📱 Pemain Tanpa Login:** Anak-anak/peserta cukup memasukkan **Kode PIN Room 6-Digit** dan **Nama Pemain** untuk mulai bermain.
- **🔒 Keamanan Berlapis:**
  - **Admin Auth:** Halaman `/admin` dilindungi HTTP Basic Auth (`ADMIN_PASSWORD`).
  - **Site Passcode Dinamis:** Mencegah publik masuk tanpa izin. Passcode tersimpan di database dan di-cache di RAM server Go. Bisa diubah kapan saja via Dashboard Admin.

---

## 🛠️ Tech Stack

- **Backend:** Golang (`net/http`, Gorilla WebSocket, Google Generative AI SDK)
- **Database:** Supabase (PostgreSQL)
- **AI Engine:** Google Gemini 1.5 Flash API
- **Frontend:** HTML5, CSS3, JavaScript Native (No Heavy Framework)
- **Hosting & Deployment:** Render.com (Web Service)

---

## 📁 Struktur Folder Proyek

```text
kuiz/
├── main.go               # Entry point, HTTP Server & Routing
├── db.go                 # Database Connection & Query (Supabase)
├── auth.go               # Auth Middleware & Passcode Cache
├── gemini_service.go     # Integrasi Google Gemini AI SDK
├── client.go             # Websocket Client Handler & Pump
├── hub.go                # Websocket Hub Engine & Event Router
├── room.go               # Room State Manager & Scoring Logic
├── go.mod                # Dependency Go
├── go.sum                # Dependency Checksum
├── schema.sql            # Database DDL for Supabase
└── static/               # Frontend Assets
    ├── index.html        # Halaman Pemain / Main Access
    ├── host.html         # Layar Utama Host / TV
    └── admin.html        # Dashboard Admin Upload & Settings
```

---

## ⚙️ Environment Variables (Variabel Lingkungan)

Pastikan Anda telah mengatur variabel berikut di lingkungan lokal (`.env`) atau di dashboard **Render.com**:

| Variable | Deskripsi | Contoh / Default |
| :--- | :--- | :--- |
| `DATABASE_URL` | PostgreSQL Connection URI dari Supabase | `postgres://postgres.xxx:pass@aws-0-...pooler.supabase.com:6543/postgres` |
| `GEMINI_API_KEY` | API Key gratis dari Google AI Studio | `AIzaSyD...` |
| `ADMIN_PASSWORD` | Password untuk mengakses `/admin` | `admin123` |
| `PORT` | Port server (Otomatis diatur oleh Render) | `8080` |

---

## 🗄️ Setup Database (Supabase)

Jalankan perintah SQL berikut di **SQL Editor** pada Dashboard Supabase Anda:

```sql
-- 1. Tabel Settings (Passcode Akses Situs)
CREATE TABLE IF NOT EXISTS settings (
    key VARCHAR(50) PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

INSERT INTO settings (key, value) 
VALUES ('site_passcode', 'BELAJAR2026') 
ON CONFLICT (key) DO NOTHING;

-- 2. Tabel Soal
CREATE TABLE IF NOT EXISTS soal (
    id BIGSERIAL PRIMARY KEY,
    grade VARCHAR(20) NOT NULL,
    subject VARCHAR(50) NOT NULL,
    semester INT NOT NULL,
    chapter_num INT NOT NULL,
    chapter_title VARCHAR(150) NOT NULL,
    question TEXT NOT NULL,
    options JSONB NOT NULL,
    correct_answer_idx INT NOT NULL,
    time_limit INT DEFAULT 15,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_soal_filter ON soal(grade, subject, semester, chapter_num);

ALTER TABLE settings ENABLE ROW LEVEL SECURITY;
ALTER TABLE soal ENABLE ROW LEVEL SECURITY;
```

---

## 🚀 Cara Menjalankan Secara Lokal

1. **Clone repository ini:**
   ```bash
   git clone https://github.com/username-anda/kuiz.git
   cd kuiz
   ```

2. **Set Environment Variables (di terminal Linux/WSL/macOS):**
   ```bash
   export DATABASE_URL="postgres://postgres.xxx:yourpassword@xxx.pooler.supabase.com:6543/postgres"
   export GEMINI_API_KEY="AIzaSy..."
   export ADMIN_PASSWORD="rahasia-admin"
   export PORT="8080"
   ```

3. **Jalankan aplikasi:**
   ```bash
   go run .
   ```

4. **Buka di Browser:**
   - **Pemain:** `http://localhost:8080?key=BELAJAR2026`
   - **Host:** `http://localhost:8080/host`
   - **Admin:** `http://localhost:8080/admin` (User: `admin`, Pass: sesuai `ADMIN_PASSWORD`)

---

## ☁️ Deployment ke Render.com

1. Push repository ini ke GitHub.
2. Buka [Render.com](https://render.com) -> Buat **New Web Service**.
3. Pilih repository GitHub ini.
4. Atur konfigurasi berikut:
   - **Environment:** `Go`
   - **Build Command:** `go build -o main .`
   - **Start Command:** `./main`
5. Tambahkan **Environment Variables** (`DATABASE_URL`, `GEMINI_API_KEY`, `ADMIN_PASSWORD`).
6. Deploy dan jalankan! 🚀
