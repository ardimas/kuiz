package main

import (
	"sync"
	"time"
)

type Hub struct {
	mu    sync.RWMutex
	rooms map[string]*Room
}

func NewHub() *Hub                { return &Hub{rooms: map[string]*Room{}} }
func (h *Hub) Add(r *Room)        { h.mu.Lock(); h.rooms[r.Code] = r; h.mu.Unlock() }
func (h *Hub) Get(c string) *Room { h.mu.RLock(); defer h.mu.RUnlock(); return h.rooms[c] }
func (h *Hub) StartJanitor() {
	go func() {
		for {
			time.Sleep(30 * time.Minute)
			h.mu.Lock()
			for c, r := range h.rooms {
				if time.Since(r.CreatedAt) > 6*time.Hour {
					delete(h.rooms, c)
				}
			}
			h.mu.Unlock()
		}
	}()
}
