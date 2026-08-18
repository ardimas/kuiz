package main

import (
	"encoding/json"
	"math/rand"
	"sort"
	"sync"
	"time"
)

type Player struct {
	Name  string `json:"name"`
	Score int    `json:"score"`
}

type Room struct {
	Code            string             `json:"code"`
	Host            *Client            `json:"-"`
	Players         map[string]*Client `json:"-"`
	PlayerStats     map[string]*Player `json:"players"`
	SoalList        []Soal             `json:"-"`
	CurrentIndex    int                `json:"current_index"`
	State           string             `json:"state"` // "LOBBY", "QUESTION", "LEADERBOARD", "FINISHED"
	QuestionStartMs int64              `json:"-"`
	AnswersReceived map[string]bool    `json:"-"`
	mu              sync.RWMutex
}

func generatePIN() string {
	rand.Seed(time.Now().UnixNano())
	const digits = "0123456789"
	b := make([]byte, 6)
	for i := range b {
		b[i] = digits[rand.Intn(len(digits))]
	}
	return string(b)
}

func NewRoom(host *Client, soal []Soal) *Room {
	return &Room{
		Code:            generatePIN(),
		Host:            host,
		Players:         make(map[string]*Client),
		PlayerStats:     make(map[string]*Player),
		SoalList:        soal,
		CurrentIndex:    0,
		State:           "LOBBY",
		AnswersReceived: make(map[string]bool),
	}
}

func (r *Room) Broadcast(msg interface{}) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.Host != nil {
		r.Host.send <- data
	}
	for _, p := range r.Players {
		p.send <- data
	}
}

func (r *Room) CalculateScore(elapsedMs int64, timeLimitSec int) int {
	maxMs := int64(timeLimitSec * 1000)
	if elapsedMs > maxMs {
		elapsedMs = maxMs
	}

	// Base score 500, Speed bonus up to 500
	speedFactor := float64(maxMs-elapsedMs) / float64(maxMs)
	if speedFactor < 0 {
		speedFactor = 0
	}

	return 500 + int(500.0*speedFactor)
}

type LeaderboardEntry struct {
	Name  string `json:"name"`
	Score int    `json:"score"`
}

func (r *Room) GetLeaderboard() []LeaderboardEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var list []LeaderboardEntry
	for _, p := range r.PlayerStats {
		list = append(list, LeaderboardEntry{Name: p.Name, Score: p.Score})
	}

	sort.Slice(list, func(i, j int) bool {
		return list[i].Score > list[j].Score
	})

	if len(list) > 5 {
		return list[:5]
	}
	return list
}
