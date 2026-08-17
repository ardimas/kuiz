package main

import (
	"context"
	"encoding/json"
	"fmt"
	"google.golang.org/genai"
	"mime"
	"path/filepath"
	"strings"
	"time"
)

type AIService struct{ APIKey string }

func NewAIService(k string) *AIService { return &AIService{k} }
func (s *AIService) Generate(ctx context.Context, data []byte, fn, g, sub string, sem, ch int, title string) ([]GeneratedQuestion, error) {
	if s.APIKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY is not configured")
	}
	if len(data) > 50*1024*1024 {
		return nil, fmt.Errorf("file exceeds 50 MB")
	}
	c, e := genai.NewClient(ctx, &genai.ClientConfig{APIKey: s.APIKey, Backend: genai.BackendGeminiAPI})
	if e != nil {
		return nil, e
	}
	mt := mime.TypeByExtension(strings.ToLower(filepath.Ext(fn)))
	if mt == "" {
		mt = "application/octet-stream"
	}
	prompt := fmt.Sprintf(`Analisis SELURUH materi pada file. Konteks: Kelas=%s; Mata Pelajaran=%s; Semester=%d; Bab=%d - %s. Buat 10-20 soal pilihan ganda yang bersumber dari materi, tepat 4 opsi A/B/C/D, satu jawaban benar, waktu 10000-30000 ms. Bahasa Indonesia. Kembalikan HANYA JSON valid: {"questions":[{"question":"...","options":["A","B","C","D"],"correct_index":0,"time_limit_ms":15000}]}`, g, sub, sem, ch, title)
	parts := []*genai.Part{{InlineData: &genai.Blob{MIMEType: mt, Data: data}}, genai.NewPartFromText(prompt)}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	res, e := c.Models.GenerateContent(ctx, "gemini-3.6-flash", []*genai.Content{genai.NewContentFromParts(parts, genai.RoleUser)}, nil)
	if e != nil {
		return nil, e
	}
	raw := strings.TrimSpace(res.Text())
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	var w struct {
		Questions []GeneratedQuestion `json:"questions"`
	}
	if e = json.Unmarshal([]byte(strings.TrimSpace(raw)), &w); e != nil {
		return nil, fmt.Errorf("invalid AI JSON: %w", e)
	}
	if len(w.Questions) == 0 {
		return nil, fmt.Errorf("AI returned zero questions")
	}
	for i := range w.Questions {
		if len(w.Questions[i].Options) != 4 || w.Questions[i].CorrectIndex < 0 || w.Questions[i].CorrectIndex > 3 {
			return nil, fmt.Errorf("invalid question %d", i)
		}
		if w.Questions[i].TimeLimitMs < 1000 || w.Questions[i].TimeLimitMs > 120000 {
			w.Questions[i].TimeLimitMs = 15000
		}
	}
	return w.Questions, nil
}
