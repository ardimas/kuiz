package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

type GeminiQuizOutput struct {
	Question         string   `json:"question"`
	Options          []string `json:"options"`
	CorrectAnswerIdx int      `json:"correct_answer_idx"`
	TimeLimit        int      `json:"time_limit"`
}

func processScanWithGemini(ctx context.Context, fileBytes []byte, mimeType string, meta Soal) ([]Soal, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY belum dikonfigurasi")
	}

	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, err
	}
	defer client.Close()

	model := client.GenerativeModel("gemini-1.5-flash")
	model.SetTemperature(0.2)
	model.ResponseMIMEType = "application/json"

	prompt := fmt.Sprintf(`Analisis seluruh isi materi pelajaran dari dokumen/gambar ini untuk %s, Mata Pelajaran: %s, Semester: %d, Bab %d (%s).
Buatkan minimal 5 sampai 10 soal kuis pilihan ganda interaktif edukatif yang berkualitas berdasarkan materi tersebut.

Kembalikan HANYA array JSON dengan struktur persis seperti ini:
[
  {
    "question": "Pertanyaan kuis yang jelas?",
    "options": ["Pilihan A", "Pilihan B", "Pilihan C", "Pilihan D"],
    "correct_answer_idx": 0,
    "time_limit": 15
  }
]
Ketentuan:
- correct_answer_idx adalah angka 0 untuk A, 1 untuk B, 2 untuk C, 3 untuk D.
- Bahasa Indonesia yang ramah anak.`, meta.Grade, meta.Subject, meta.Semester, meta.ChapterNum, meta.ChapterTitle)

	dataPart := genai.Data(mimeType, fileBytes)
	resp, err := model.GenerateContent(ctx, dataPart, genai.Text(prompt))
	if err != nil {
		return nil, err
	}

	if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
		return nil, fmt.Errorf("tidak ada respon dari Gemini API")
	}

	var rawJSON string
	for _, part := range resp.Candidates[0].Content.Parts {
		if text, ok := part.(genai.Text); ok {
			rawJSON += string(text)
		}
	}

	rawJSON = strings.TrimSpace(rawJSON)

	var parsedQuizzes []GeminiQuizOutput
	err = json.Unmarshal([]byte(rawJSON), &parsedQuizzes)
	if err != nil {
		return nil, fmt.Errorf("gagal parse JSON dari Gemini: %v | Raw: %s", err, rawJSON)
	}

	var result []Soal
	for _, q := range parsedQuizzes {
		if len(q.Options) != 4 {
			continue
		}
		if q.TimeLimit <= 0 {
			q.TimeLimit = 15
		}
		result = append(result, Soal{
			Grade:            meta.Grade,
			Subject:          meta.Subject,
			Semester:         meta.Semester,
			ChapterNum:       meta.ChapterNum,
			ChapterTitle:     meta.ChapterTitle,
			Question:         q.Question,
			Options:          q.Options,
			CorrectAnswerIdx: q.CorrectAnswerIdx,
			TimeLimit:        q.TimeLimit,
		})
	}

	return result, nil
}
