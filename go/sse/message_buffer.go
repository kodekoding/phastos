package sse

import (
	"sync"
	"time"
)

// MessageBuffer stores SSE messages with ID tracking for reliable delivery
type MessageBuffer struct {
	messages map[string]*BufferedMessage // Keyed by message ID
	mu       sync.RWMutex
	maxSize  int           // Maximum messages to keep in memory
	ttl      time.Duration // Time to keep messages before cleanup
}

// BufferedMessage represents a message with delivery metadata
type BufferedMessage struct {
	ID        string                 `json:"id"`
	Event     string                 `json:"event"`
	Data      string                 `json:"data"`
	Timestamp time.Time              `json:"timestamp"`
	CreatedAt time.Time              `json:"-"`
	ExpiresAt time.Time              `json:"-"`
	Retry     int                    `json:"retry,omitempty"`
	Extra     map[string]interface{} `json:"extra,omitempty"`
}

// ClientDeliveryTracker tracks which messages have been delivered to which clients
type ClientDeliveryTracker struct {
	clientID       string
	lastMessageID  string
	acknowledgedAt time.Time
	mu             sync.RWMutex
}

// NewMessageBuffer creates a new message buffer
func NewMessageBuffer(maxSize int, ttl time.Duration) *MessageBuffer {
	mb := &MessageBuffer{
		messages: make(map[string]*BufferedMessage),
		maxSize:  maxSize,
		ttl:      ttl,
	}

	// Start cleanup goroutine
	go mb.cleanupExpired()

	return mb
}

// AddMessage adds a message to the buffer
func (mb *MessageBuffer) AddMessage(msg *BufferedMessage) {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	msg.CreatedAt = time.Now()
	msg.ExpiresAt = time.Now().Add(mb.ttl)

	mb.messages[msg.ID] = msg

	// Cleanup if buffer is too large
	if len(mb.messages) > mb.maxSize {
		mb.evictOldest()
	}
}

// GetMessagesSince returns all messages since a given message ID
func (mb *MessageBuffer) GetMessagesSince(messageID string) []*BufferedMessage {
	mb.mu.RLock()
	defer mb.mu.RUnlock()

	var result []*BufferedMessage
	foundStart := messageID == ""

	for _, msg := range mb.messages {
		if foundStart {
			result = append(result, msg)
		} else if msg.ID == messageID {
			foundStart = true
		}
	}

	return result
}

// GetUndeliveredMessages returns messages that haven't been delivered to a client
func (mb *MessageBuffer) GetUndeliveredMessages(clientID string, lastAcknowledgedID string) []*BufferedMessage {
	return mb.GetMessagesSince(lastAcknowledgedID)
}

// evictOldest removes the oldest message from buffer
func (mb *MessageBuffer) evictOldest() {
	var oldestID string
	var oldestTime time.Time

	for id, msg := range mb.messages {
		if oldestTime.IsZero() || msg.CreatedAt.Before(oldestTime) {
			oldestTime = msg.CreatedAt
			oldestID = id
		}
	}

	if oldestID != "" {
		delete(mb.messages, oldestID)
	}
}

// cleanupExpired removes expired messages periodically
func (mb *MessageBuffer) cleanupExpired() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		mb.mu.Lock()
		now := time.Now()
		for id, msg := range mb.messages {
			if now.After(msg.ExpiresAt) {
				delete(mb.messages, id)
			}
		}
		mb.mu.Unlock()
	}
}

// GetSize returns the current number of buffered messages
func (mb *MessageBuffer) GetSize() int {
	mb.mu.RLock()
	defer mb.mu.RUnlock()
	return len(mb.messages)
}

// Clear removes all messages from buffer
func (mb *MessageBuffer) Clear() {
	mb.mu.Lock()
	defer mb.mu.Unlock()
	mb.messages = make(map[string]*BufferedMessage)
}

// NewClientDeliveryTracker creates a tracker for a client
func NewClientDeliveryTracker(clientID string) *ClientDeliveryTracker {
	return &ClientDeliveryTracker{
		clientID:       clientID,
		lastMessageID:  "",
		acknowledgedAt: time.Now(),
	}
}

// UpdateLastMessageID updates the last acknowledged message ID
func (cdt *ClientDeliveryTracker) UpdateLastMessageID(messageID string) {
	cdt.mu.Lock()
	defer cdt.mu.Unlock()
	cdt.lastMessageID = messageID
	cdt.acknowledgedAt = time.Now()
}

// GetLastMessageID returns the last acknowledged message ID
func (cdt *ClientDeliveryTracker) GetLastMessageID() string {
	cdt.mu.RLock()
	defer cdt.mu.RUnlock()
	return cdt.lastMessageID
}

// ClientDeliveryManager manages delivery tracking for multiple clients
type ClientDeliveryManager struct {
	trackers map[string]*ClientDeliveryTracker
	mu       sync.RWMutex
}

// NewClientDeliveryManager creates a new delivery manager
func NewClientDeliveryManager() *ClientDeliveryManager {
	return &ClientDeliveryManager{
		trackers: make(map[string]*ClientDeliveryTracker),
	}
}

// RegisterClient registers a new client
func (cdm *ClientDeliveryManager) RegisterClient(clientID string) *ClientDeliveryTracker {
	cdm.mu.Lock()
	defer cdm.mu.Unlock()

	tracker := NewClientDeliveryTracker(clientID)
	cdm.trackers[clientID] = tracker
	return tracker
}

// UnregisterClient removes a client tracker
func (cdm *ClientDeliveryManager) UnregisterClient(clientID string) {
	cdm.mu.Lock()
	defer cdm.mu.Unlock()
	delete(cdm.trackers, clientID)
}

// UpdateClientAcknowledgment updates the last acknowledged message for a client
func (cdm *ClientDeliveryManager) UpdateClientAcknowledgment(clientID string, messageID string) {
	cdm.mu.RLock()
	tracker, exists := cdm.trackers[clientID]
	cdm.mu.RUnlock()

	if exists {
		tracker.UpdateLastMessageID(messageID)
	}
}

// GetClientTracker returns the tracker for a client
func (cdm *ClientDeliveryManager) GetClientTracker(clientID string) *ClientDeliveryTracker {
	cdm.mu.RLock()
	defer cdm.mu.RUnlock()
	return cdm.trackers[clientID]
}

// GetAllClients returns all registered client IDs
func (cdm *ClientDeliveryManager) GetAllClients() []string {
	cdm.mu.RLock()
	defer cdm.mu.RUnlock()

	var clients []string
	for clientID := range cdm.trackers {
		clients = append(clients, clientID)
	}
	return clients
}
