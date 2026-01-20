package sse

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// Client represents a single  connection
type Client struct {
	ID         string
	Channel    chan *Message
	Request    *http.Request
	Writer     http.ResponseWriter
	Flusher    http.Flusher
	disconnect chan bool
	mu         sync.Mutex
}

// Message represents a message to be sent via
type Message struct {
	Event string
	Data  interface{}
	ID    string
	Retry int
}

// Hub manages all  connections
type Hub struct {
	clients    map[string]*Client
	broadcast  chan *Message
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
}

type Iface interface {
	Broadcast(msg *Message)
}

// NewHub creates a new  hub
func NewHub(ctx context.Context) *Hub {
	hubCtx, cancel := context.WithCancel(ctx)
	return &Hub{
		clients:    make(map[string]*Client),
		broadcast:  make(chan *Message, 100),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		ctx:        hubCtx,
		cancel:     cancel,
	}
}

// Run starts the  hub
func (hub *Hub) Run() {
	log.Info().Msg(" Hub started")

	for {
		select {
		case <-hub.ctx.Done():
			log.Info().Msg(" Hub stopped")
			return
		case client := <-hub.register:
			hub.mu.Lock()
			hub.clients[client.ID] = client
			hub.mu.Unlock()
			log.Info().Str("client_id", client.ID).Int("total_clients", len(hub.clients)).Msg(" client registered")
		case client := <-hub.unregister:
			hub.mu.Lock()
			if _, ok := hub.clients[client.ID]; ok {
				close(client.Channel)
				delete(hub.clients, client.ID)
			}
			hub.mu.Unlock()
			log.Info().Str("client_id", client.ID).Int("total_clients", len(hub.clients)).Msg(" client unregistered")
		case message := <-hub.broadcast:
			hub.mu.RLock()
			for _, client := range hub.clients {
				select {
				case client.Channel <- message:
				default:
					// Channel is full, skip this message for this client
					log.Warn().Str("client_id", client.ID).Msg(" message dropped for client")
				}
			}
			hub.mu.RUnlock()
		}
	}
}

// Stop gracefully stops the  hub
func (hub *Hub) Stop() {
	log.Info().Msg("Stopping  Hub")

	hub.cancel()

	hub.mu.Lock()
	for _, client := range hub.clients {
		close(client.Channel)
	}
	hub.clients = make(map[string]*Client)
	hub.mu.Unlock()

	close(hub.broadcast)
	close(hub.register)
	close(hub.unregister)
}

// Broadcast sends a message to all connected clients
func (hub *Hub) Broadcast(message *Message) {
	select {
	case hub.broadcast <- message:
	case <-hub.ctx.Done():
		log.Warn().Msg("Cannot broadcast,  hub is stopped")
	default:
		log.Warn().Msg("Broadcast channel is full, message dropped")
	}
}

// SendToClient sends a message to a specific client
func (hub *Hub) SendToClient(clientID string, message *Message) error {
	hub.mu.RLock()
	client, exists := hub.clients[clientID]
	hub.mu.RUnlock()

	if !exists {
		return fmt.Errorf("client %s not found", clientID)
	}

	select {
	case client.Channel <- message:
		return nil
	default:
		return fmt.Errorf("client %s channel is full", clientID)
	}
}

// GetClientCount returns the number of connected clients
func (hub *Hub) GetClientCount() int {
	hub.mu.RLock()
	defer hub.mu.RUnlock()
	return len(hub.clients)
}

// Handle is an HTTP handler for  connections
func (hub *Hub) Handle(w http.ResponseWriter, r *http.Request) {
	// Set  headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		log.Error().Msg("Streaming unsupported")
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Create client
	clientID := r.Header.Get("X-Client-ID")
	if clientID == "" {
		clientID = fmt.Sprintf("client-%d", time.Now().UnixNano())
	}

	client := &Client{
		ID:         clientID,
		Channel:    make(chan *Message, 10),
		Request:    r,
		Writer:     w,
		Flusher:    flusher,
		disconnect: make(chan bool),
	}

	// Register client
	hub.register <- client

	// Send initial connection message
	initialMsg := &Message{
		Event: "connected",
		Data:  map[string]string{"client_id": clientID, "timestamp": time.Now().Format(time.RFC3339)},
	}
	client.sendMessage(initialMsg)

	// Handle client disconnect
	defer func() {
		hub.unregister <- client
	}()

	// Keep connection alive with heartbeat
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			log.Info().Str("client_id", clientID).Msg(" client disconnected")
			return
		case <-hub.ctx.Done():
			log.Info().Str("client_id", clientID).Msg(" hub stopped, disconnecting client")
			return
		case message := <-client.Channel:
			if err := client.sendMessage(message); err != nil {
				log.Err(err).Str("client_id", clientID).Msg("Failed to send  message")
				return
			}
		case <-ticker.C:
			// Send heartbeat
			heartbeat := &Message{
				Event: "heartbeat",
				Data:  map[string]string{"timestamp": time.Now().Format(time.RFC3339)},
			}
			if err := client.sendMessage(heartbeat); err != nil {
				log.Err(err).Str("client_id", clientID).Msg("Failed to send heartbeat")
				return
			}
		}
	}
}

// sendMessage sends a message to the client
func (client *Client) sendMessage(message *Message) error {
	client.mu.Lock()
	defer client.mu.Unlock()

	// Write event type
	if message.Event != "" {
		if _, err := fmt.Fprintf(client.Writer, "event: %s\n", message.Event); err != nil {
			return err
		}
	}

	// Write message ID
	if message.ID != "" {
		if _, err := fmt.Fprintf(client.Writer, "id: %s\n", message.ID); err != nil {
			return err
		}
	}

	// Write retry interval
	if message.Retry > 0 {
		if _, err := fmt.Fprintf(client.Writer, "retry: %d\n", message.Retry); err != nil {
			return err
		}
	}

	// Write data
	var dataStr string
	switch v := message.Data.(type) {
	case string:
		dataStr = v
	default:
		dataBytes, err := json.Marshal(v)
		if err != nil {
			return err
		}
		dataStr = string(dataBytes)
	}

	if _, err := fmt.Fprintf(client.Writer, "data: %s\n\n", dataStr); err != nil {
		return err
	}

	client.Flusher.Flush()
	return nil
}

// NewMessage creates a new  message
func NewMessage(event string, data interface{}) *Message {
	return &Message{
		Event: event,
		Data:  data,
		ID:    fmt.Sprintf("%d", time.Now().UnixNano()),
	}
}
