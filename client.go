package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"github.com/gorilla/websocket"
	"strings"
	"time"
)

type Client struct {
	Conn   *websocket.Conn
	SendQ  chan any
	Room   *Room
	Player *Player
	Role   string
	App    *App
}

func (c *Client) readPump() {
	defer func() {
		c.Conn.Close()
		if c.Player != nil {
			c.Room.mu.Lock()
			delete(c.Room.Players, c.Player.ID)
			c.Room.mu.Unlock()
			c.Room.broadcast(map[string]any{"type": "lobby", "players": c.RoomPlayers()})
		}
	}()
	c.Conn.SetReadLimit(64 << 10)
	c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.Conn.SetPongHandler(func(string) error { c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second)); return nil })
	for {
		_, b, e := c.Conn.ReadMessage()
		if e != nil {
			return
		}
		var m struct {
			Type, Name, PIN string
			Index           int
		}
		if json.Unmarshal(b, &m) != nil {
			continue
		}
		switch m.Type {
		case "join":
			if m.Name == "" {
				c.sendError("name required")
				continue
			}
			p := &Player{ID: randomID(), Name: safeName(m.Name), Client: c}
			c.Player = p
			c.Room.mu.Lock()
			c.Room.Players[p.ID] = p
			c.Room.mu.Unlock()
			c.Send(map[string]any{"type": "joined", "player_id": p.ID, "room": publicRoom(c.Room)})
			c.Room.broadcast(map[string]any{"type": "lobby", "players": c.RoomPlayers()})
		case "start":
			if c.Role == "host" {
				c.App.startRoom(c.Room)
			}
		case "next":
			if c.Role == "host" {
				c.App.nextQuestion(c.Room)
			}
		case "answer":
			if c.Player != nil {
				elapsed := time.Since(c.Room.QuestionStarted).Milliseconds()
				c.App.answer(c.Room, c.Player, m.Index, elapsed)
			}
		}
	}
}
func (c *Client) writePump() {
	t := time.NewTicker(25 * time.Second)
	defer func() { t.Stop(); c.Conn.Close() }()
	for {
		select {
		case v := <-c.SendQ:
			b, _ := json.Marshal(v)
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if c.Conn.WriteMessage(websocket.TextMessage, b) != nil {
				return
			}
		case <-t.C:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if c.Conn.WriteMessage(websocket.PingMessage, nil) != nil {
				return
			}
		}
	}
}
func (c *Client) Send(v any) {
	select {
	case c.SendQ <- v:
	default:
	}
}
func (c *Client) sendError(s string) { c.Send(map[string]any{"type": "error", "message": s}) }
func (c *Client) RoomPlayers() []map[string]any {
	c.Room.mu.Lock()
	defer c.Room.mu.Unlock()
	o := []map[string]any{}
	for _, p := range c.Room.Players {
		o = append(o, map[string]any{"id": p.ID, "name": p.Name, "score": p.Score, "correct": p.Correct})
	}
	return o
}
func randomID() string { b := make([]byte, 8); rand.Read(b); return hex.EncodeToString(b) }
func safeName(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 40 {
		s = s[:40]
	}
	return s
}
