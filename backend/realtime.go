package main

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

type Hub struct {
	Redis      *redis.Client
	Register   chan *Client
	Unregister chan *Client
	Clients    map[string]map[*Client]bool
	mu         sync.Mutex
}

type Client struct {
	Conn   *websocket.Conn
	PollID string
	Send   chan []byte
}

func NewHub(rdb *redis.Client) *Hub {
	return &Hub{
		Redis:      rdb,
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
		Clients:    make(map[string]map[*Client]bool),
	}
}

func (h *Hub) Run() {
	ctx := context.Background()
	pubsub := h.Redis.PSubscribe(ctx, "poll:*")
	defer pubsub.Close()

	go func() {
		for message := range pubsub.Channel() {
			var payload map[string]interface{}
			if json.Unmarshal([]byte(message.Payload), &payload) != nil {
				continue
			}
			data, _ := json.Marshal(payload)
			pollID := payload["pollId"].(string)
			h.broadcast(pollID, data)
		}
	}()

	for {
		select {
		case client := <-h.Register:
			h.mu.Lock()
			if h.Clients[client.PollID] == nil {
				h.Clients[client.PollID] = make(map[*Client]bool)
			}
			h.Clients[client.PollID][client] = true
			h.mu.Unlock()

		case client := <-h.Unregister:
			h.remove(client)
		}
	}
}

func (h *Hub) broadcast(pollID string, data []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for client := range h.Clients[pollID] {
		select {
		case client.Send <- data:
		default:
			close(client.Send)
			delete(h.Clients[pollID], client)
		}
	}
}

func (h *Hub) remove(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if clients, ok := h.Clients[client.PollID]; ok {
		if _, exists := clients[client]; exists {
			delete(clients, client)
			close(client.Send)
		}
		if len(clients) == 0 {
			delete(h.Clients, client.PollID)
		}
	}
}

func (a *App) WebSocket(c *gin.Context) {
	pollID := c.Param("id")
	if _, err := a.findPoll(c.Request.Context(), pollID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "poll not found"})
		return
	}

	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	client := &Client{
		Conn:   conn,
		PollID: pollID,
		Send:   make(chan []byte, 16),
	}

	a.Hub.Register <- client

	go writePump(a.Hub, client)
	readPump(a.Hub, client)
}

func writePump(h *Hub, client *Client) {
	defer client.Conn.Close()
	for message := range client.Send {
		if err := client.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
			h.remove(client)
			return
		}
	}
}

func readPump(h *Hub, client *Client) {
	defer func() {
		h.remove(client)
		client.Conn.Close()
	}()

	for {
		if _, _, err := client.Conn.ReadMessage(); err != nil {
			return
		}
	}
}
