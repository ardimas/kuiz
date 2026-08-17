package main

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"sort"
	"sync"
	"time"
)

type Player struct {
	ID      string  `json:"id"`
	Name    string  `json:"name"`
	Score   int     `json:"score"`
	Correct int     `json:"correct"`
	TotalMs int64   `json:"total_ms"`
	Client  *Client `json:"-"`
}
type Room struct {
	Code, Grade, Subject, Scope           string
	Semester, Chapter                     *int
	Questions                             []Question
	Players                               map[string]*Player
	Current                               int
	State                                 string
	CreatedAt, StartedAt, QuestionStarted time.Time
	mu                                    sync.Mutex
}

func randomPIN(h *Hub) (string, error) {
	for i := 0; i < 100; i++ {
		n, e := rand.Int(rand.Reader, big.NewInt(1000000))
		if e != nil {
			return "", e
		}
		x := fmt.Sprintf("%06d", n.Int64())
		if h.Get(x) == nil {
			return x, nil
		}
	}
	return "", fmt.Errorf("unable to create PIN")
}
func (a *App) createRoom(w http.ResponseWriter, r *http.Request) {
	if !a.requireSite(r) {
		jsonError(w, 401, "site authentication required")
		return
	}
	if r.Method != http.MethodPost {
		jsonError(w, 405, "method not allowed")
		return
	}
	var x struct {
		Grade, Subject, Scope string
		Semester, Chapter     *int
	}
	if json.NewDecoder(r.Body).Decode(&x) != nil {
		jsonError(w, 400, "invalid json")
		return
	}
	var sem, chap *int
	switch x.Scope {
	case "chapter":
		if x.Semester == nil || x.Chapter == nil {
			jsonError(w, 400, "semester/chapter required")
			return
		}
		sem = x.Semester
		chap = x.Chapter
	case "semester1":
		v := 1
		sem = &v
	case "semester2":
		v := 2
		sem = &v
	case "full_year":
	default:
		jsonError(w, 400, "invalid scope")
		return
	}
	qs, e := a.DB.GetQuestions(r.Context(), x.Grade, x.Subject, sem, chap)
	if e != nil {
		jsonError(w, 500, e.Error())
		return
	}
	if len(qs) == 0 {
		jsonError(w, 400, "no questions match filter")
		return
	}
	code, e := randomPIN(a.Hub)
	if e != nil {
		jsonError(w, 500, e.Error())
		return
	}
	room := &Room{Code: code, Grade: x.Grade, Subject: x.Subject, Scope: x.Scope, Semester: sem, Chapter: chap, Questions: qs, Players: map[string]*Player{}, State: "lobby", CreatedAt: time.Now()}
	a.Hub.Add(room)
	writeJSON(w, map[string]any{"room": publicRoom(room)})
}
func (a *App) roomAPI(w http.ResponseWriter, r *http.Request) {
	if !a.requireSite(r) {
		jsonError(w, 401, "site authentication required")
		return
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 3 {
		jsonError(w, 404, "not found")
		return
	}
	room := a.Hub.Get(parts[2])
	if room == nil {
		jsonError(w, 404, "room not found")
		return
	}
	writeJSON(w, map[string]any{"room": publicRoom(room)})
}
func publicRoom(r *Room) map[string]any {
	r.mu.Lock()
	defer r.mu.Unlock()
	ps := []map[string]any{}
	for _, p := range r.Players {
		ps = append(ps, map[string]any{"id": p.ID, "name": p.Name, "score": p.Score, "correct": p.Correct})
	}
	return map[string]any{"code": r.Code, "grade": r.Grade, "subject": r.Subject, "scope": r.Scope, "state": r.State, "current": r.Current, "total": len(r.Questions), "players": ps}
}
func (a *App) startRoom(r *Room) {
	r.mu.Lock()
	if r.State != "lobby" {
		r.mu.Unlock()
		return
	}
	r.State = "question"
	r.Current = 0
	r.QuestionStarted = time.Now()
	q := r.Questions[0]
	r.mu.Unlock()
	r.broadcast(map[string]any{"type": "question", "index": 0, "question": q, "server_time": time.Now().UnixMilli()})
}
func (a *App) nextQuestion(r *Room) {
	r.mu.Lock()
	r.Current++
	if r.Current >= len(r.Questions) {
		r.State = "finished"
		r.mu.Unlock()
		r.broadcast(map[string]any{"type": "finished", "leaderboard": leaderboard(r)})
		return
	}
	r.QuestionStarted = time.Now()
	q := r.Questions[r.Current]
	idx := r.Current
	r.mu.Unlock()
	r.broadcast(map[string]any{"type": "question", "index": idx, "question": q, "server_time": time.Now().UnixMilli()})
}
func (r *Room) broadcast(v any) {
	for _, p := range r.Players {
		if p.Client != nil {
			p.Client.Send(v)
		}
	}
}
func (a *App) answer(r *Room, p *Player, idx int, elapsed int64) {
	r.mu.Lock()
	if r.State != "question" || r.Current >= len(r.Questions) {
		r.mu.Unlock()
		return
	}
	q := r.Questions[r.Current]
	ok := idx == q.CorrectIndex
	if ok {
		p.Correct++
		p.Score++
	}
	p.TotalMs += elapsed
	r.mu.Unlock()
	p.Client.Send(map[string]any{"type": "answer_result", "correct": ok, "correct_index": q.CorrectIndex, "score": p.Score})
}
func leaderboard(r *Room) []*Player {
	r.mu.Lock()
	defer r.mu.Unlock()
	o := make([]*Player, 0, len(r.Players))
	for _, p := range r.Players {
		c := *p
		c.Client = nil
		o = append(o, &c)
	}
	sort.Slice(o, func(i, j int) bool {
		if o[i].Correct != o[j].Correct {
			return o[i].Correct > o[j].Correct
		}
		return o[i].TotalMs < o[j].TotalMs
	})
	if len(o) > 3 {
		o = o[:3]
	}
	return o
}
