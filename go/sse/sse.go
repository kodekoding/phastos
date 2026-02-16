package sse

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/kodekoding/phastos/v2/go/helper"
	plog "github.com/kodekoding/phastos/v2/go/log"
	"github.com/rs/zerolog"
)

type Servers interface {
	Broadcast(message *Message)
}

// SSEClient represents a single SSE connection
type Client struct {
	ID         string
	Channel    chan *Message
	Request    *http.Request
	Writer     http.ResponseWriter
	Flusher    http.Flusher
	disconnect chan bool
	mu         sync.Mutex
}

// SSEMessage represents a message to be sent via SSE
type Message struct {
	Event string
	Data  interface{}
	ID    string
	Retry int
}

// TokenValidator is a function type for validating tokens/api-keys
type TokenValidator func(token string) (bool, error)

// EncryptedTokenValidator is a function type for validating encrypted tokens
type EncryptedTokenValidator func(encryptedToken string) (bool, error)

// SSEHub manages all SSE connections
type Hub struct {
	clients                 map[string]*Client
	broadcast               chan *Message
	register                chan *Client
	unregister              chan *Client
	mu                      sync.RWMutex
	ctx                     context.Context
	cancel                  context.CancelFunc
	tokenValidator          TokenValidator
	encryptedTokenValidator EncryptedTokenValidator
	cryptoManager           *helper.CryptoManager
	messageBuffer           *MessageBuffer
	deliveryManager         *ClientDeliveryManager
}

// NewHub creates a new SSE hub
func NewHub(ctx context.Context) *Hub {
	hubCtx, cancel := context.WithCancel(ctx)
	return &Hub{
		clients:                 make(map[string]*Client),
		broadcast:               make(chan *Message, 100),
		register:                make(chan *Client),
		unregister:              make(chan *Client),
		ctx:                     hubCtx,
		cancel:                  cancel,
		tokenValidator:          nil, // Default: no validation
		encryptedTokenValidator: nil,
		cryptoManager:           nil,
		messageBuffer:           NewMessageBuffer(1000, 24*time.Hour), // Keep 1000 messages for 24 hours
		deliveryManager:         NewClientDeliveryManager(),
	}
}

// SetTokenValidator sets the token validation function
func (hub *Hub) SetTokenValidator(validator TokenValidator) {
	hub.tokenValidator = validator
}

// SetEncryptedTokenValidator sets the encrypted token validation function
func (hub *Hub) SetEncryptedTokenValidator(validator EncryptedTokenValidator) {
	hub.encryptedTokenValidator = validator
}

// SetCryptoManager sets the crypto manager for decrypting tokens
func (hub *Hub) SetCryptoManager(cm *helper.CryptoManager) {
	hub.cryptoManager = cm
}

// SetMessageBuffer sets a custom message buffer
func (hub *Hub) SetMessageBuffer(mb *MessageBuffer) {
	hub.messageBuffer = mb
}

// GetMessageBuffer returns the message buffer
func (hub *Hub) GetMessageBuffer() *MessageBuffer {
	return hub.messageBuffer
}

// GetDeliveryManager returns the delivery manager
func (hub *Hub) GetDeliveryManager() *ClientDeliveryManager {
	return hub.deliveryManager
}

// Run starts the SSE hub
func (hub *Hub) Run() {
	log := plog.Get()
	log.Info().Msg("SSE Hub started")

	for {
		select {
		case <-hub.ctx.Done():
			log.Info().Msg("SSE Hub stopped")
			return
		case client := <-hub.register:
			hub.mu.Lock()
			hub.clients[client.ID] = client
			hub.mu.Unlock()
			// Register client with delivery manager for message tracking
			hub.deliveryManager.RegisterClient(client.ID)
			log.Info().Str("client_id", client.ID).Int("total_clients", len(hub.clients)).Msg("SSE client registered")
		case client := <-hub.unregister:
			hub.mu.Lock()
			if _, ok := hub.clients[client.ID]; ok {
				close(client.Channel)
				delete(hub.clients, client.ID)
			}
			hub.mu.Unlock()
			// Unregister client from delivery manager
			hub.deliveryManager.UnregisterClient(client.ID)
			log.Info().Str("client_id", client.ID).Int("total_clients", len(hub.clients)).Msg("SSE client unregistered")
		case message := <-hub.broadcast:
			hub.mu.RLock()
			for _, client := range hub.clients {
				select {
				case client.Channel <- message:
				default:
					// Channel is full, skip this message for this clientz
					log.Warn().Str("client_id", client.ID).Msg("SSE message dropped for client")
				}
			}
			hub.mu.RUnlock()
		}
	}
}

// Stop gracefully stops the SSE hub
func (hub *Hub) Stop() {
	log := plog.Get()
	log.Info().Msg("Stopping SSE Hub")

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

// Broadcast sends a message to all connected clients and buffers it for later retrieval
func (hub *Hub) Broadcast(message *Message) {
	// Buffer the message for offline clients

	if hub.messageBuffer != nil && message.ID != "" {
		bufferedMsg := &BufferedMessage{
			ID:        message.ID,
			Event:     message.Event,
			Data:      "",
			Timestamp: time.Now(),
			Retry:     message.Retry,
		}

		// Convert message data to JSON string for buffering
		if dataBytes, err := json.Marshal(message.Data); err == nil {
			bufferedMsg.Data = string(dataBytes)
		}

		hub.messageBuffer.AddMessage(bufferedMsg)
	}
	log := plog.Get()
	log.UpdateContext(func(c zerolog.Context) zerolog.Context {
		return c.
			Str("process_name", "[SSE][BROADCASTING_MESSAGE]").
			Str("message_id", message.ID).
			Str("event", message.Event).
			Any("data", message.Data)
	})
	select {
	case hub.broadcast <- message:
		log.Info().Msg("Successfully broadcasted message")
	case <-hub.ctx.Done():
		log.Warn().Msg("Cannot broadcast, SSE hub is stopped")
	default:
		log.Warn().Msg("Broadcast channel is full, message dropped")
	}
}

// GetMissedMessages returns messages that were sent while a client was offline
// clientID: the client requesting missed messages
// lastReceivedID: the ID of the last message the client successfully received
func (hub *Hub) GetMissedMessages(clientID string, lastReceivedID string) []*BufferedMessage {
	if hub.messageBuffer == nil {
		return nil
	}
	log := plog.Get()
	log.UpdateContext(func(c zerolog.Context) zerolog.Context {
		return c.
			Str("process_name", "[SSE][GET_MISSED_MESSAGE]").
			Str("client_id", clientID).
			Str("last_received_id", lastReceivedID)
	})
	return hub.messageBuffer.GetMessagesSince(lastReceivedID)
}

// SendToClient sends a message to a specific client
func (hub *Hub) SendToClient(clientID string, message *Message) error {
	hub.mu.RLock()
	client, exists := hub.clients[clientID]
	hub.mu.RUnlock()

	if !exists {
		return fmt.Errorf("client %s not found", clientID)
	}

	// Buffer the message for offline clients
	if hub.messageBuffer != nil && message.ID != "" {
		bufferedMsg := &BufferedMessage{
			ID:        message.ID,
			Event:     message.Event,
			Data:      "",
			Timestamp: time.Now(),
			Retry:     message.Retry,
		}

		if dataBytes, err := json.Marshal(message.Data); err == nil {
			bufferedMsg.Data = string(dataBytes)
		}

		hub.messageBuffer.AddMessage(bufferedMsg)
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

// Handle is an HTTP handler for SSE connections
func (hub *Hub) Handle(w http.ResponseWriter, r *http.Request) {

	log := plog.Ctx(r.Context())

	log.UpdateContext(func(c zerolog.Context) zerolog.Context {
		return c.Str("process_name", "[SSE][CLIENT_CONNECTED]")
	})
	// Token validation - supports both plain and encrypted tokens
	tokenValidated := false

	// Try encrypted token validation first
	if hub.encryptedTokenValidator != nil && hub.cryptoManager != nil {
		encryptedToken := r.Header.Get("X-Encrypted-Token")
		if encryptedToken == "" {
			encryptedToken = r.URL.Query().Get("encrypted_token")
		}

		if encryptedToken != "" {
			// Decrypt the token
			decryptedToken, err := hub.cryptoManager.Decrypt(encryptedToken)
			if err != nil {
				log.Warn().Err(err).Msg("Failed to decrypt token")
				http.Error(w, `{"code":"FORBIDDEN","message":"Invalid encrypted token"}`, http.StatusForbidden)
				return
			}

			isValid, err := hub.encryptedTokenValidator(decryptedToken)
			if err != nil {
				log.Error().Err(err).Msg("Encrypted token validation error")
				http.Error(w, `{"code":"SERVER_ERROR","message":"Token validation failed"}`, http.StatusInternalServerError)
				return
			}

			if !isValid {
				log.Warn().Msg("SSE connection attempt with invalid encrypted token")
				http.Error(w, `{"code":"FORBIDDEN","message":"Invalid or expired token/api-key"}`, http.StatusForbidden)
				return
			}

			tokenValidated = true
			log.Debug().Msg("SSE encrypted token validation successful")
		}
	}

	// Try plain text token validation if no encrypted token or encrypted validation is not enabled
	if !tokenValidated && hub.tokenValidator != nil {
		token := r.Header.Get("Authorization")
		if token == "" {
			token = r.URL.Query().Get("token")
		}

		if token == "" {
			log.Warn().Msg("SSE connection attempt without token")
			http.Error(w, `{"code":"UNAUTHORIZED","message":"Missing token/api-key"}`, http.StatusUnauthorized)
			return
		}

		// Validate token - remove "Bearer " prefix if present
		tokenValue := token
		if len(token) > 7 && token[:7] == "Bearer " {
			tokenValue = token[7:]
		}

		isValid, err := hub.tokenValidator(tokenValue)
		if err != nil {
			log.Error().Err(err).Msg("Token validation error")
			http.Error(w, `{"code":"SERVER_ERROR","message":"Token validation failed"}`, http.StatusInternalServerError)
			return
		}

		if !isValid {
			log.Warn().Str("token", tokenValue[:len(tokenValue)-5]+"...").Msg("SSE connection attempt with invalid token")
			http.Error(w, `{"code":"FORBIDDEN","message":"Invalid or expired token/api-key"}`, http.StatusForbidden)
			return
		}

		tokenValidated = true
		log.Debug().Msg("SSE token validation successful")
	}

	// Check if any validation is required but not passed
	if (hub.tokenValidator != nil || hub.encryptedTokenValidator != nil) && !tokenValidated {
		log.Warn().Msg("SSE connection attempt without valid token")
		http.Error(w, `{"code":"UNAUTHORIZED","message":"Missing or invalid token/api-key"}`, http.StatusUnauthorized)
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Encrypted-Token")
	w.Header().Set("X-Accel-Buffering", "no") // Disable proxy buffering

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
			log.Info().Str("client_id", clientID).Msg("SSE client disconnected")
			return
		case <-hub.ctx.Done():
			log.Info().Str("client_id", clientID).Msg("SSE hub stopped, disconnecting client")
			return
		case message := <-client.Channel:
			if err := client.sendMessage(message); err != nil {
				log.Err(err).Str("client_id", clientID).Msg("Failed to send SSE message")
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

// NewSSEMessage creates a new SSE message
func NewSSEMessage(event string, data interface{}) *Message {
	return &Message{
		Event: event,
		Data:  data,
		ID:    fmt.Sprintf("%d", time.Now().UnixNano()),
	}
}
