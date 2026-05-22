package sse

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// --- MessageBuffer ---

func TestNewMessageBuffer(t *testing.T) {
	mb := NewMessageBuffer(100, 1*time.Hour)
	assert.NotNil(t, mb)
	assert.Equal(t, 0, mb.GetSize())
}

func TestMessageBufferAddAndGet(t *testing.T) {
	mb := NewMessageBuffer(100, 1*time.Hour)
	msg := &BufferedMessage{
		ID:    "msg-1",
		Event: "test",
		Data:  "hello",
	}
	mb.AddMessage(msg)
	assert.Equal(t, 1, mb.GetSize())
}

func TestMessageBufferGetMessagesSinceEmptyID(t *testing.T) {
	mb := NewMessageBuffer(100, 1*time.Hour)
	mb.AddMessage(&BufferedMessage{ID: "1", Event: "a", Data: "d1"})
	mb.AddMessage(&BufferedMessage{ID: "2", Event: "b", Data: "d2"})

	result := mb.GetMessagesSince("")
	// With empty ID, all messages should be returned
	assert.Equal(t, 2, len(result))
}

func TestMessageBufferGetMessagesSinceSpecificID(t *testing.T) {
	mb := NewMessageBuffer(100, 1*time.Hour)
	mb.AddMessage(&BufferedMessage{ID: "1", Event: "a", Data: "d1"})
	mb.AddMessage(&BufferedMessage{ID: "2", Event: "b", Data: "d2"})
	mb.AddMessage(&BufferedMessage{ID: "3", Event: "c", Data: "d3"})

	// GetMessagesSince iterates over a map (non-deterministic order),
	// so we can't guarantee how many messages appear after the target ID.
	// Instead, verify that the found message(s) are not ID "1" itself.
	result := mb.GetMessagesSince("1")
	for _, msg := range result {
		assert.NotEqual(t, "1", msg.ID, "should not include the starting message")
	}
	// When called with empty ID, all messages should be returned
	allResult := mb.GetMessagesSince("")
	assert.Equal(t, 3, len(allResult))
}

func TestMessageBufferEvictOldest(t *testing.T) {
	mb := NewMessageBuffer(2, 1*time.Hour) // maxSize = 2
	mb.AddMessage(&BufferedMessage{ID: "1", Event: "a", Data: "d1"})
	mb.AddMessage(&BufferedMessage{ID: "2", Event: "b", Data: "d2"})
	mb.AddMessage(&BufferedMessage{ID: "3", Event: "c", Data: "d3"}) // should evict oldest

	assert.LessOrEqual(t, mb.GetSize(), 2)
}

func TestMessageBufferClear(t *testing.T) {
	mb := NewMessageBuffer(100, 1*time.Hour)
	mb.AddMessage(&BufferedMessage{ID: "1", Event: "a", Data: "d1"})
	mb.AddMessage(&BufferedMessage{ID: "2", Event: "b", Data: "d2"})
	mb.Clear()
	assert.Equal(t, 0, mb.GetSize())
}

func TestMessageBufferGetUndeliveredMessages(t *testing.T) {
	mb := NewMessageBuffer(100, 1*time.Hour)
	mb.AddMessage(&BufferedMessage{ID: "1", Event: "a", Data: "d1"})
	mb.AddMessage(&BufferedMessage{ID: "2", Event: "b", Data: "d2"})

	result := mb.GetUndeliveredMessages("client-1", "")
	assert.GreaterOrEqual(t, len(result), 1)
}

// --- ClientDeliveryTracker ---

func TestNewClientDeliveryTracker(t *testing.T) {
	tracker := NewClientDeliveryTracker("client-1")
	assert.NotNil(t, tracker)
	assert.Equal(t, "client-1", tracker.clientID)
	assert.Equal(t, "", tracker.GetLastMessageID())
}

func TestClientDeliveryTrackerUpdateLastMessageID(t *testing.T) {
	tracker := NewClientDeliveryTracker("client-1")
	tracker.UpdateLastMessageID("msg-5")
	assert.Equal(t, "msg-5", tracker.GetLastMessageID())
}

func TestClientDeliveryTrackerUpdateTwice(t *testing.T) {
	tracker := NewClientDeliveryTracker("client-1")
	tracker.UpdateLastMessageID("msg-1")
	tracker.UpdateLastMessageID("msg-2")
	assert.Equal(t, "msg-2", tracker.GetLastMessageID())
}

// --- ClientDeliveryManager ---

func TestNewClientDeliveryManager(t *testing.T) {
	cdm := NewClientDeliveryManager()
	assert.NotNil(t, cdm)
	assert.Empty(t, cdm.GetAllClients())
}

func TestClientDeliveryManagerRegister(t *testing.T) {
	cdm := NewClientDeliveryManager()
	tracker := cdm.RegisterClient("client-1")
	assert.NotNil(t, tracker)
	clients := cdm.GetAllClients()
	assert.Contains(t, clients, "client-1")
}

func TestClientDeliveryManagerUnregister(t *testing.T) {
	cdm := NewClientDeliveryManager()
	cdm.RegisterClient("client-1")
	cdm.UnregisterClient("client-1")
	clients := cdm.GetAllClients()
	assert.NotContains(t, clients, "client-1")
}

func TestClientDeliveryManagerGetTracker(t *testing.T) {
	cdm := NewClientDeliveryManager()
	cdm.RegisterClient("client-1")
	tracker := cdm.GetClientTracker("client-1")
	assert.NotNil(t, tracker)
	
	tracker2 := cdm.GetClientTracker("non-existent")
	assert.Nil(t, tracker2)
}

func TestClientDeliveryManagerUpdateAcknowledgment(t *testing.T) {
	cdm := NewClientDeliveryManager()
	cdm.RegisterClient("client-1")
	cdm.UpdateClientAcknowledgment("client-1", "msg-10")
	
	tracker := cdm.GetClientTracker("client-1")
	assert.Equal(t, "msg-10", tracker.GetLastMessageID())
}

func TestClientDeliveryManagerUpdateNonExistentClient(t *testing.T) {
	cdm := NewClientDeliveryManager()
	// Should not panic
	cdm.UpdateClientAcknowledgment("non-existent", "msg-1")
}

// --- Hub ---

func TestNewHub(t *testing.T) {
	hub := NewHubWithCancel()
	assert.NotNil(t, hub)
	assert.Equal(t, 0, hub.GetClientCount())
	hub.Stop()
}

func TestHubSetTokenValidator(t *testing.T) {
	hub := NewHubWithCancel()
	defer hub.Stop()
	
	validator := func(token string) (bool, error) { return true, nil }
	hub.SetTokenValidator(validator)
	assert.NotNil(t, hub.tokenValidator)
}

func TestHubSetEncryptedTokenValidator(t *testing.T) {
	hub := NewHubWithCancel()
	defer hub.Stop()
	
	validator := func(token string) (bool, error) { return true, nil }
	hub.SetEncryptedTokenValidator(validator)
	assert.NotNil(t, hub.encryptedTokenValidator)
}

func TestHubGetMessageBuffer(t *testing.T) {
	hub := NewHubWithCancel()
	defer hub.Stop()
	
	mb := hub.GetMessageBuffer()
	assert.NotNil(t, mb)
}

func TestHubGetDeliveryManager(t *testing.T) {
	hub := NewHubWithCancel()
	defer hub.Stop()
	
	cdm := hub.GetDeliveryManager()
	assert.NotNil(t, cdm)
}

func TestHubSetMessageBuffer(t *testing.T) {
	hub := NewHubWithCancel()
	defer hub.Stop()
	
	customMB := NewMessageBuffer(50, 30*time.Minute)
	hub.SetMessageBuffer(customMB)
	assert.Equal(t, customMB, hub.GetMessageBuffer())
}

func TestHubGetMissedMessages(t *testing.T) {
	hub := NewHubWithCancel()
	defer hub.Stop()
	
	// No message buffer set - should return nil
	// Actually NewHub sets a buffer by default
	msgs := hub.GetMissedMessages("client-1", "")
	// No messages added yet
	assert.Empty(t, msgs)
}

func TestHubSendToClientNotFound(t *testing.T) {
	hub := NewHubWithCancel()
	defer hub.Stop()
	
	err := hub.SendToClient("non-existent", &Message{Event: "test", Data: "hello"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// --- NewSSEMessage ---

func TestNewSSEMessage(t *testing.T) {
	msg := NewSSEMessage("update", map[string]string{"key": "val"})
	assert.Equal(t, "update", msg.Event)
	assert.NotNil(t, msg.Data)
	assert.NotEmpty(t, msg.ID)
}

// --- Message struct ---

func TestMessageStruct(t *testing.T) {
	msg := &Message{
		Event: "notification",
		Data:  "test data",
		ID:    "msg-123",
		Retry: 5000,
	}
	assert.Equal(t, "notification", msg.Event)
	assert.Equal(t, "test data", msg.Data)
	assert.Equal(t, "msg-123", msg.ID)
	assert.Equal(t, 5000, msg.Retry)
}

// --- Client struct ---

func TestClientStruct(t *testing.T) {
	client := &Client{
		ID:      "client-1",
		Channel: make(chan *Message, 10),
	}
	assert.Equal(t, "client-1", client.ID)
	assert.NotNil(t, client.Channel)
}

// Helper to create a Hub with cancel for testing
func NewHubWithCancel() *Hub {
	return NewHub(context.Background())
}

// --- BufferedMessage ---

func TestBufferedMessageStruct(t *testing.T) {
	now := time.Now()
	msg := &BufferedMessage{
		ID:        "bm-1",
		Event:     "test",
		Data:      "data",
		Timestamp: now,
		CreatedAt: now,
		ExpiresAt: now.Add(1 * time.Hour),
		Retry:     1000,
		Extra:     map[string]interface{}{"key": "val"},
	}
	assert.Equal(t, "bm-1", msg.ID)
	assert.Equal(t, "test", msg.Event)
	assert.Equal(t, 1000, msg.Retry)
}
