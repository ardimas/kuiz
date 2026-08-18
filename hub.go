package main

import (
	"encoding/json"
	"sync"
	"time"
)

type Hub struct {
	rooms      map[string]*Room
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
}

func newHub() *Hub {
	return &Hub{
		rooms:      make(map[string]*Room),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			_ = client
		case client := <-h.unregister:
			h.mu.Lock()
			if room, exists := h.rooms[client.roomCode]; exists {
				room.mu.Lock()
				if client.isHost {
					room.Broadcast(map[string]interface{}{"type": "HOST_DISCONNECTED"})
					delete(h.rooms, client.roomCode)
				} else {
					delete(room.Players, client.name)
					delete(room.PlayerStats, client.name)
					room.Broadcast(map[string]interface{}{
						"type":    "PLAYER_LEFT",
						"payload": map[string]interface{}{"name": client.name, "players": room.PlayerStats},
					})
				}
				room.mu.Unlock()
			}
			h.mu.Unlock()
		}
	}
}

func (h *Hub) handleMessage(client *Client, msg *WSMessage) {
	switch msg.Type {
	case "CREATE_ROOM":
		var payload struct {
			Grade    string `json:"grade"`
			Subject  string `json:"subject"`
			Mode     string `json:"mode"`
			Semester int    `json:"semester"`
			Chapter  int    `json:"chapter"`
		}
		json.Unmarshal(msg.Payload, &payload)

		soalList, err := fetchSoalFiltered(payload.Grade, payload.Subject, payload.Mode, payload.Semester, payload.Chapter)
		if err != nil || len(soalList) == 0 {
			client.send <- []byte(`{"type":"ERROR","payload":"Soal tidak ditemukan untuk filter ini"}`)
			return
		}

		room := NewRoom(client, soalList)
		client.roomCode = room.Code
		client.isHost = true

		h.mu.Lock()
		h.rooms[room.Code] = room
		h.mu.Unlock()

		client.send <- []byte(mustJSON(map[string]interface{}{
			"type":    "ROOM_CREATED",
			"payload": map[string]interface{}{"pin": room.Code},
		}))

	case "JOIN_ROOM":
		var payload struct {
			PIN  string `json:"pin"`
			Name string `json:"name"`
		}
		json.Unmarshal(msg.Payload, &payload)

		h.mu.RLock()
		room, exists := h.rooms[payload.PIN]
		h.mu.RUnlock()

		if !exists {
			client.send <- []byte(`{"type":"ERROR","payload":"Kode PIN Room tidak ditemukan"}`)
			return
		}

		room.mu.Lock()
		if room.State != "LOBBY" {
			room.mu.Unlock()
			client.send <- []byte(`{"type":"ERROR","payload":"Game sudah dimulai"}`)
			return
		}

		if _, taken := room.Players[payload.Name]; taken {
			room.mu.Unlock()
			client.send <- []byte(`{"type":"ERROR","payload":"Nama sudah dipakai"}`)
			return
		}

		client.roomCode = room.Code
		client.name = payload.Name
		client.isHost = false

		room.Players[payload.Name] = client
		room.PlayerStats[payload.Name] = &Player{Name: payload.Name, Score: 0}

		room.mu.Unlock()

		client.send <- []byte(`{"type":"JOIN_SUCCESS"}`)

		room.Broadcast(map[string]interface{}{
			"type":    "PLAYER_JOINED",
			"payload": map[string]interface{}{"players": room.PlayerStats},
		})

	case "START_GAME":
		if !client.isHost {
			return
		}

		h.mu.RLock()
		room, exists := h.rooms[client.roomCode]
		h.mu.RUnlock()

		if exists {
			sendNextQuestion(room)
		}

	case "SUBMIT_ANSWER":
		var payload struct {
			OptionIndex int `json:"option_index"`
		}
		json.Unmarshal(msg.Payload, &payload)

		h.mu.RLock()
		room, exists := h.rooms[client.roomCode]
		h.mu.RUnlock()

		if !exists {
			return
		}

		room.mu.Lock()
		if room.State != "QUESTION" || room.AnswersReceived[client.name] {
			room.mu.Unlock()
			return
		}

		room.AnswersReceived[client.name] = true
		elapsedMs := time.Now().UnixMilli() - room.QuestionStartMs
		currentSoal := room.SoalList[room.CurrentIndex]

		isCorrect := payload.OptionIndex == currentSoal.CorrectAnswerIdx
		addedScore := 0

		if isCorrect {
			addedScore = room.CalculateScore(elapsedMs, currentSoal.TimeLimit)
			room.PlayerStats[client.name].Score += addedScore
		}

		room.mu.Unlock()

		client.send <- []byte(mustJSON(map[string]interface{}{
			"type": "ANSWER_RESULT",
			"payload": map[string]interface{}{
				"correct":     isCorrect,
				"score_added": addedScore,
				"total_score": room.PlayerStats[client.name].Score,
			},
		}))

		// Cek jika semua pemain sudah menjawab
		room.mu.RLock()
		allAnswered := len(room.AnswersReceived) >= len(room.Players)
		room.mu.RUnlock()

		if allAnswered {
			showQuestionEnd(room)
		}
	}
}

func sendNextQuestion(room *Room) {
	room.mu.Lock()
	if room.CurrentIndex >= len(room.SoalList) {
		room.State = "FINISHED"
		room.mu.Unlock()

		room.Broadcast(map[string]interface{}{
			"type":    "GAME_OVER",
			"payload": map[string]interface{}{"leaderboard": room.GetLeaderboard()},
		})
		return
	}

	room.State = "QUESTION"
	room.AnswersReceived = make(map[string]bool)
	room.QuestionStartMs = time.Now().UnixMilli()

	soal := room.SoalList[room.CurrentIndex]
	room.mu.Unlock()

	// Kirim soal tanpa pemberitahuan correct_answer_idx ke pemain
	room.Broadcast(map[string]interface{}{
		"type": "NEW_QUESTION",
		"payload": map[string]interface{}{
			"index":      room.CurrentIndex + 1,
			"total":      len(room.SoalList),
			"question":   soal.Question,
			"options":    soal.Options,
			"time_limit": soal.TimeLimit,
		},
	})

	// Timer otomatis di server
	go func(r *Room, idx int) {
		time.Sleep(time.Duration(soal.TimeLimit) * time.Second)
		r.mu.Lock()
		if r.State == "QUESTION" && r.CurrentIndex == idx {
			r.mu.Unlock()
			showQuestionEnd(r)
		} else {
			r.mu.Unlock()
		}
	}(room, room.CurrentIndex)
}

func showQuestionEnd(room *Room) {
	room.mu.Lock()
	if room.State != "QUESTION" {
		room.mu.Unlock()
		return
	}
	room.State = "LEADERBOARD"
	soal := room.SoalList[room.CurrentIndex]
	room.CurrentIndex++
	room.mu.Unlock()

	room.Broadcast(map[string]interface{}{
		"type": "QUESTION_END",
		"payload": map[string]interface{}{
			"correct_idx": soal.CorrectAnswerIdx,
			"leaderboard": room.GetLeaderboard(),
		},
	})
}

func mustJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}
