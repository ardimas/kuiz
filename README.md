# Quiz Realtime Go
Run: `go mod tidy && go run .`
Env: `PORT`, `DATABASE_URL`, `GEMINI_API_KEY`, `ADMIN_PASSWORD`, `AUTH_SECRET`.
Run schema.sql in Supabase SQL Editor. Pages: `/`, `/admin.html`, `/host.html`.
Prototype note: room/player state is in RAM, so Render restart/spin-down loses active rooms. For production multi-instance use Redis/pubsub/shared state. WebSocket CheckOrigin is permissive for prototype and should be restricted to your domain. Gemini inline document input is limited to 50MB; larger/repeated files should use Files API.
