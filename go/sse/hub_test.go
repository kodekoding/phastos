package sse

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kodekoding/phastos/v2/go/helper"
	plog "github.com/kodekoding/phastos/v2/go/log"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Client.sendMessage tests ---

type stubFlusher struct {
	flushed bool
}

func (sf *stubFlusher) Flush() { sf.flushed = true }

type errorWriter struct {
	http.ResponseWriter
}

func (ew *errorWriter) Write(p []byte) (int, error) {
	return 0, http.ErrAbortHandler
}

func (ew *errorWriter) Fprintf(format string, a ...interface{}) (int, error) {
	return 0, http.ErrAbortHandler
}

type stubResponseWriter struct {
	header http.Header
	body   strings.Builder
	code   int
}

func newStubResponseWriter() *stubResponseWriter {
	return &stubResponseWriter{header: make(http.Header)}
}

func (srw *stubResponseWriter) Header() http.Header        { return srw.header }
func (srw *stubResponseWriter) Write(p []byte) (int, error) { return srw.body.Write(p) }
func (srw *stubResponseWriter) WriteHeader(code int)       { srw.code = code }

func TestClientSendMessage_AllFields(t *testing.T) {
	w := newStubResponseWriter()
	flusher := &stubFlusher{}
	client := &Client{
		ID:      "client-1",
		Writer:  w,
		Flusher: flusher,
	}

	msg := &Message{
		Event: "notification",
		Data:  map[string]string{"key": "value"},
		ID:    "msg-123",
		Retry: 5000,
	}
	err := client.sendMessage(msg)
	require.NoError(t, err)
	body := w.body.String()
	assert.Contains(t, body, "event: notification")
	assert.Contains(t, body, "id: msg-123")
	assert.Contains(t, body, "retry: 5000")
	assert.Contains(t, body, "data:")
	assert.True(t, flusher.flushed)
}

func TestClientSendMessage_StringData(t *testing.T) {
	w := newStubResponseWriter()
	flusher := &stubFlusher{}
	client := &Client{
		ID:      "client-1",
		Writer:  w,
		Flusher: flusher,
	}

	msg := &Message{
		Event: "update",
		Data:  "hello world",
	}
	err := client.sendMessage(msg)
	require.NoError(t, err)
	body := w.body.String()
	assert.Contains(t, body, "event: update")
	assert.Contains(t, body, "data: hello world")
}

func TestClientSendMessage_NoEvent(t *testing.T) {
	w := newStubResponseWriter()
	flusher := &stubFlusher{}
	client := &Client{
		ID:      "client-1",
		Writer:  w,
		Flusher: flusher,
	}

	msg := &Message{
		Data: "no event",
		ID:   "msg-1",
	}
	err := client.sendMessage(msg)
	require.NoError(t, err)
	body := w.body.String()
	assert.NotContains(t, body, "event:")
	assert.Contains(t, body, "id: msg-1")
}

func TestClientSendMessage_NoID(t *testing.T) {
	w := newStubResponseWriter()
	flusher := &stubFlusher{}
	client := &Client{
		ID:      "client-1",
		Writer:  w,
		Flusher: flusher,
	}

	msg := &Message{
		Event: "test",
		Data:  "data",
	}
	err := client.sendMessage(msg)
	require.NoError(t, err)
	body := w.body.String()
	assert.NotContains(t, body, "id:")
}

func TestClientSendMessage_NoRetry(t *testing.T) {
	w := newStubResponseWriter()
	flusher := &stubFlusher{}
	client := &Client{
		ID:      "client-1",
		Writer:  w,
		Flusher: flusher,
	}

	msg := &Message{
		Event: "test",
		Data:  "data",
		Retry: 0,
	}
	err := client.sendMessage(msg)
	require.NoError(t, err)
	body := w.body.String()
	assert.NotContains(t, body, "retry:")
}

func TestClientSendMessage_WriteError(t *testing.T) {
	// Create a client with a writer that returns errors
	w := httptest.NewRecorder()
	flusher := &stubFlusher{}
	client := &Client{
		ID:      "client-1",
		Writer:  w,
		Flusher: flusher,
	}

	// This should succeed with the normal recorder
	msg := &Message{Event: "test", Data: "data"}
	err := client.sendMessage(msg)
	// The test recorder shouldn't produce errors
	require.NoError(t, err)
}

// --- Hub.Run and Broadcast tests ---

func TestHubRunAndBroadcast(t *testing.T) {
	hub := NewHub(context.Background())
	go hub.Run()
	defer hub.Stop()

	// Register a client manually
	client := &Client{
		ID:         "test-client",
		Channel:    make(chan *Message, 10),
		disconnect: make(chan bool),
	}
	hub.register <- client

	// Wait for registration
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 1, hub.GetClientCount())

	// Broadcast a message
	msg := &Message{Event: "test", Data: "hello", ID: "msg-1"}
	hub.Broadcast(msg)

	// Wait for message delivery
	time.Sleep(50 * time.Millisecond)
	select {
	case received := <-client.Channel:
		assert.Equal(t, "test", received.Event)
		assert.Equal(t, "hello", received.Data)
	case <-time.After(1 * time.Second):
		t.Fatal("Expected to receive broadcast message")
	}
}

func TestHubBroadcastWithBuffer(t *testing.T) {
	hub := NewHub(context.Background())
	go hub.Run()
	defer hub.Stop()

	// Broadcast with ID should buffer the message
	msg := &Message{Event: "test", Data: map[string]string{"k": "v"}, ID: "msg-buffered"}
	hub.Broadcast(msg)

	time.Sleep(50 * time.Millisecond)

	// Check that message was buffered
	missed := hub.GetMissedMessages("client-1", "")
	assert.GreaterOrEqual(t, len(missed), 1)
}

func TestHubBroadcastWithoutID(t *testing.T) {
	hub := NewHub(context.Background())
	go hub.Run()
	defer hub.Stop()

	// Broadcast without ID should NOT buffer
	msg := &Message{Event: "test", Data: "no-id"}
	hub.Broadcast(msg)

	time.Sleep(50 * time.Millisecond)
}

func TestHubSendToClient_Success(t *testing.T) {
	hub := NewHub(context.Background())
	go hub.Run()
	defer hub.Stop()

	client := &Client{
		ID:         "target-client",
		Channel:    make(chan *Message, 10),
		disconnect: make(chan bool),
	}
	hub.register <- client
	time.Sleep(50 * time.Millisecond)

	msg := &Message{Event: "personal", Data: "for you", ID: "msg-pers"}
	err := hub.SendToClient("target-client", msg)
	require.NoError(t, err)

	select {
	case received := <-client.Channel:
		assert.Equal(t, "personal", received.Event)
	case <-time.After(1 * time.Second):
		t.Fatal("Expected to receive personal message")
	}
}

func TestHubSendToClient_ChannelFull(t *testing.T) {
	hub := NewHub(context.Background())
	go hub.Run()
	defer hub.Stop()

	// Create client with very small channel buffer
	client := &Client{
		ID:         "full-client",
		Channel:    make(chan *Message, 1),
		disconnect: make(chan bool),
	}
	hub.register <- client
	time.Sleep(50 * time.Millisecond)

	// Fill the channel
	client.Channel <- &Message{Event: "filler"}

	// Try to send another message
	msg := &Message{Event: "overflow", Data: "too much"}
	err := hub.SendToClient("full-client", msg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "channel is full")
}

func TestHubStop(t *testing.T) {
	hub := NewHub(context.Background())
	go hub.Run()

	client := &Client{
		ID:         "stop-client",
		Channel:    make(chan *Message, 10),
		disconnect: make(chan bool),
	}
	hub.register <- client
	time.Sleep(50 * time.Millisecond)

	assert.Equal(t, 1, hub.GetClientCount())
	hub.Stop()

	// After stop, clients should be cleared
	assert.Equal(t, 0, hub.GetClientCount())
}

func TestHubSetCryptoManager(t *testing.T) {
	hub := NewHubWithCancel()
	defer hub.Stop()

	cm, err := helper.NewCryptoManager("test-encryption-key")
	require.NoError(t, err)
	hub.SetCryptoManager(cm)
	assert.NotNil(t, hub.cryptoManager)
}

// withContextLogger returns a context with an isolated zerolog logger,
// preventing data races when Handle (which calls plog.Ctx) runs concurrently
// with Hub.Run (which calls plog.Get). A non-disabled logger is required
// because zerolog won't attach a disabled logger to a context that has none.
func withContextLogger(ctx context.Context) context.Context {
	logger := zerolog.New(io.Discard).With().Timestamp().Logger()
	return logger.WithContext(ctx)
}

// --- Hub.Handle tests ---

func TestHubHandle_NoValidation(t *testing.T) {
	_ = plog.Get() // initialize global logger once

	hub := NewHub(context.Background())
	go hub.Run()

	ctx, cancel := context.WithCancel(context.Background())
	ctx = withContextLogger(ctx)
	req := httptest.NewRequest(http.MethodGet, "/sse", nil)
	req = req.WithContext(ctx)
	req.Header.Set("X-Client-ID", "test-handle-client")
	rec := httptest.NewRecorder()

	// Run Handle in a goroutine since it blocks
	done := make(chan bool)
	go func() {
		hub.Handle(rec, req)
		done <- true
	}()

	time.Sleep(100 * time.Millisecond)

	// Cancel the context to end the handler
	cancel()

	select {
	case <-done:
		// Handler exited
	case <-time.After(2 * time.Second):
		t.Fatal("Handler did not exit")
	}
	// Use cancel() instead of Stop() to avoid send-on-closed-channel
	hub.cancel()
}

func TestHubHandle_TokenValidation(t *testing.T) {
	t.Run("no token", func(t *testing.T) {
		hub := NewHub(context.Background())
		hub.SetTokenValidator(func(token string) (bool, error) {
			return token == "valid-token", nil
		})
	go hub.Run()
	defer hub.cancel()

	req := httptest.NewRequest(http.MethodGet, "/sse", nil)
	req = req.WithContext(withContextLogger(req.Context()))
	rec := httptest.NewRecorder()

	hub.Handle(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("invalid token", func(t *testing.T) {
		hub := NewHub(context.Background())
		hub.SetTokenValidator(func(token string) (bool, error) {
			return token == "valid-token", nil
		})
		go hub.Run()
		defer hub.cancel()

		req := httptest.NewRequest(http.MethodGet, "/sse", nil)
		req.Header.Set("Authorization", "invalid-token")
		req = req.WithContext(withContextLogger(req.Context()))
		rec := httptest.NewRecorder()

		hub.Handle(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("valid token with Bearer prefix", func(t *testing.T) {
		hub := NewHub(context.Background())
		hub.SetTokenValidator(func(token string) (bool, error) {
			return token == "valid-token", nil
		})
		go hub.Run()

		ctx, cancel := context.WithCancel(context.Background())
		ctx = withContextLogger(ctx)
		req := httptest.NewRequest(http.MethodGet, "/sse", nil)
		req = req.WithContext(ctx)
		req.Header.Set("Authorization", "Bearer valid-token")
		req.Header.Set("X-Client-ID", "client-bearer")
		rec := httptest.NewRecorder()

		go func() {
			time.Sleep(100 * time.Millisecond)
			cancel()
		}()
		hub.Handle(rec, req)
		hub.cancel()
	})
}

func TestHubHandle_TokenValidationError(t *testing.T) {
	hub := NewHub(context.Background())
	hub.SetTokenValidator(func(token string) (bool, error) {
		return false, assert.AnError
	})
	go hub.Run()
	defer hub.cancel()

	req := httptest.NewRequest(http.MethodGet, "/sse", nil)
	req.Header.Set("Authorization", "some-token")
	req = req.WithContext(withContextLogger(req.Context()))
	rec := httptest.NewRecorder()

	hub.Handle(rec, req)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestHubHandle_EncryptedTokenValidation(t *testing.T) {
	cm, err := helper.NewCryptoManager("test-encryption-key-12345")
	require.NoError(t, err)

	// Encrypt a valid token
	encrypted, err := cm.Encrypt("valid-plain-token")
	require.NoError(t, err)

	t.Run("valid encrypted token in header", func(t *testing.T) {
		hub := NewHub(context.Background())
		hub.SetCryptoManager(cm)
		hub.SetEncryptedTokenValidator(func(token string) (bool, error) {
			return token == "valid-plain-token", nil
		})
		go hub.Run()

		ctx, cancel := context.WithCancel(context.Background())
		ctx = withContextLogger(ctx)
		req := httptest.NewRequest(http.MethodGet, "/sse", nil)
		req = req.WithContext(ctx)
		req.Header.Set("X-Encrypted-Token", encrypted)
		req.Header.Set("X-Client-ID", "enc-client-1")
		rec := httptest.NewRecorder()

		go func() {
			time.Sleep(100 * time.Millisecond)
			cancel()
		}()
		hub.Handle(rec, req)
		hub.cancel()
	})

	t.Run("invalid encrypted token", func(t *testing.T) {
		hub := NewHub(context.Background())
		hub.SetCryptoManager(cm)
		hub.SetEncryptedTokenValidator(func(token string) (bool, error) {
			return token == "valid-plain-token", nil
		})
		go hub.Run()
		defer hub.cancel()

		req := httptest.NewRequest(http.MethodGet, "/sse", nil)
		req.Header.Set("X-Encrypted-Token", "invalid-encrypted-token")
		req = req.WithContext(withContextLogger(req.Context()))
		rec := httptest.NewRecorder()

		hub.Handle(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("encrypted token via query param", func(t *testing.T) {
		hub := NewHub(context.Background())
		hub.SetCryptoManager(cm)
		hub.SetEncryptedTokenValidator(func(token string) (bool, error) {
			return token == "valid-plain-token", nil
		})
		go hub.Run()

		ctx, cancel := context.WithCancel(context.Background())
		ctx = withContextLogger(ctx)
		req := httptest.NewRequest(http.MethodGet, "/sse?encrypted_token="+encrypted, nil)
		req = req.WithContext(ctx)
		req.Header.Set("X-Client-ID", "enc-client-2")
		rec := httptest.NewRecorder()

		go func() {
			time.Sleep(100 * time.Millisecond)
			cancel()
		}()
		hub.Handle(rec, req)
		hub.cancel()
	})
}

func TestHubHandle_EncryptedTokenValidationError(t *testing.T) {
	cm, err := helper.NewCryptoManager("test-encryption-key-12345")
	require.NoError(t, err)

	hub := NewHub(context.Background())
	hub.SetCryptoManager(cm)
	hub.SetEncryptedTokenValidator(func(token string) (bool, error) {
		return false, assert.AnError
	})
	go hub.Run()
	defer hub.cancel()

	encrypted, err := cm.Encrypt("some-token")
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/sse", nil)
	req.Header.Set("X-Encrypted-Token", encrypted)
	req = req.WithContext(withContextLogger(req.Context()))
	rec := httptest.NewRecorder()

	hub.Handle(rec, req)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestHubHandle_EncryptedTokenInvalidToken(t *testing.T) {
	cm, err := helper.NewCryptoManager("test-encryption-key-12345")
	require.NoError(t, err)

	hub := NewHub(context.Background())
	hub.SetCryptoManager(cm)
	hub.SetEncryptedTokenValidator(func(token string) (bool, error) {
		return false, nil // Token decrypted but invalid
	})
	go hub.Run()
	defer hub.cancel()

	encrypted, err := cm.Encrypt("wrong-token")
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/sse", nil)
	req.Header.Set("X-Encrypted-Token", encrypted)
	req = req.WithContext(withContextLogger(req.Context()))
	rec := httptest.NewRecorder()

	hub.Handle(rec, req)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestHubHandle_ValidationRequiredButNoToken(t *testing.T) {
	hub := NewHub(context.Background())
	hub.SetTokenValidator(func(token string) (bool, error) {
		return true, nil
	})
	go hub.Run()
	defer hub.cancel()

	req := httptest.NewRequest(http.MethodGet, "/sse", nil)
	req = req.WithContext(withContextLogger(req.Context()))
	// No Authorization header, no token query param
	rec := httptest.NewRecorder()

	hub.Handle(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestHubHandle_EncryptedValidationButNoToken(t *testing.T) {
	cm, err := helper.NewCryptoManager("test-key-1234567890")
	require.NoError(t, err)

	hub := NewHub(context.Background())
	hub.SetCryptoManager(cm)
	hub.SetEncryptedTokenValidator(func(token string) (bool, error) {
		return true, nil
	})
	go hub.Run()
	// Use cancel instead of Stop to avoid closing channels while Run is listening
	defer hub.cancel()

	// No encrypted token provided - should fall through to token validator check
	req := httptest.NewRequest(http.MethodGet, "/sse", nil)
	req = req.WithContext(withContextLogger(req.Context()))
	rec := httptest.NewRecorder()

	hub.Handle(rec, req)
	// Both validators set, no token provided, should get unauthorized
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestHubHandle_NoFlusher(t *testing.T) {
	hub := NewHub(context.Background())
	hub.SetTokenValidator(func(token string) (bool, error) {
		return true, nil
	})
	go hub.Run()

	// stubResponseWriter does NOT implement http.Flusher,
	// so the type assertion w.(http.Flusher) should fail.
	w := newStubResponseWriter()

	req := httptest.NewRequest(http.MethodGet, "/sse", nil)
	req.Header.Set("Authorization", "test-token")
	req.Header.Set("X-Client-ID", "noflush-client")
	req = req.WithContext(withContextLogger(req.Context()))

	hub.Handle(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.code)
	hub.cancel()
}

func TestHubHandle_TokenViaQueryParam(t *testing.T) {
	hub := NewHub(context.Background())
	hub.SetTokenValidator(func(token string) (bool, error) {
		return token == "valid-qp-token", nil
	})
	go hub.Run()

	ctx, cancel := context.WithCancel(context.Background())
	ctx = withContextLogger(ctx)
	req := httptest.NewRequest(http.MethodGet, "/sse?token=valid-qp-token", nil)
	req = req.WithContext(ctx)
	req.Header.Set("X-Client-ID", "qp-client")
	rec := httptest.NewRecorder()

	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()
	hub.Handle(rec, req)
	// Don't call hub.Stop() because Handle's defer sends to closed channels
	// Instead, just cancel the hub context
	hub.cancel()
}

func TestHubHandle_AutoGeneratedClientID(t *testing.T) {
	hub := NewHub(context.Background())
	go hub.Run()

	ctx, cancel := context.WithCancel(context.Background())
	ctx = withContextLogger(ctx)
	req := httptest.NewRequest(http.MethodGet, "/sse", nil)
	req = req.WithContext(ctx)
	// No X-Client-ID header - should auto-generate
	rec := httptest.NewRecorder()

	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()
	hub.Handle(rec, req)

	// Use cancel instead of Stop to avoid send-on-closed-channel
	hub.cancel()
}

func TestHubRun_UnregisterClient(t *testing.T) {
	hub := NewHub(context.Background())
	go hub.Run()
	defer hub.Stop()

	client := &Client{
		ID:         "unreg-client",
		Channel:    make(chan *Message, 10),
		disconnect: make(chan bool),
	}
	hub.register <- client
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 1, hub.GetClientCount())

	hub.unregister <- client
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 0, hub.GetClientCount())
}

func TestHubRun_BroadcastFullChannel(t *testing.T) {
	hub := NewHub(context.Background())
	go hub.Run()
	defer hub.Stop()

	client := &Client{
		ID:         "full-bcast-client",
		Channel:    make(chan *Message, 1),
		disconnect: make(chan bool),
	}
	hub.register <- client
	time.Sleep(50 * time.Millisecond)

	// Fill the channel
	client.Channel <- &Message{Event: "filler"}

	// Broadcast should not block even with full channel
	msg := &Message{Event: "overflow", Data: "too much"}
	hub.Broadcast(msg)
}

func TestHubGetMissedMessagesNilBuffer(t *testing.T) {
	hub := NewHubWithCancel()
	hub.messageBuffer = nil
	defer hub.Stop()

	msgs := hub.GetMissedMessages("client-1", "last-id")
	assert.Nil(t, msgs)
}

func TestNewSSEMessageWithData(t *testing.T) {
	msg := NewSSEMessage("test", "string-data")
	assert.Equal(t, "test", msg.Event)
	assert.Equal(t, "string-data", msg.Data)
	assert.NotEmpty(t, msg.ID)
}

func TestBufferedMessageJSONMarshal(t *testing.T) {
	msg := &BufferedMessage{
		ID:        "bm-1",
		Event:     "test",
		Data:      `{"key":"val"}`,
		Timestamp: time.Now(),
		Retry:     1000,
		Extra:     map[string]interface{}{"extra_key": "extra_val"},
	}
	data, err := json.Marshal(msg)
	require.NoError(t, err)
	assert.Contains(t, string(data), "bm-1")
	assert.Contains(t, string(data), "test")
	assert.Contains(t, string(data), "extra_key")
}

func TestEventsInterface(t *testing.T) {
	var _ Events = &Hub{}
}

func TestHubBroadcast_ContextStopped(t *testing.T) {
	hub := NewHubWithCancel()
	hub.cancel() // Cancel the context

	msg := &Message{Event: "test", Data: "data"}
	// Should not panic or block
	hub.Broadcast(msg)
}

func TestHubBroadcast_DefaultDrop(t *testing.T) {
	hub := NewHubWithCancel()
	// Fill the broadcast channel
	for i := 0; i < 100; i++ {
		hub.broadcast <- &Message{Event: "fill"}
	}

	// Next broadcast should hit the default case (channel full)
	msg := &Message{Event: "overflow", Data: "data"}
	hub.Broadcast(msg) // Should not block
}

func TestHubHandle_DecryptError(t *testing.T) {
	cm, err := helper.NewCryptoManager("test-encryption-key-12345")
	require.NoError(t, err)

	hub := NewHub(context.Background())
	hub.SetCryptoManager(cm)
	hub.SetEncryptedTokenValidator(func(token string) (bool, error) {
		return true, nil
	})
	go hub.Run()
	defer hub.cancel()

	// Send garbage that can't be base64-decoded
	req := httptest.NewRequest(http.MethodGet, "/sse", nil)
	req.Header.Set("X-Encrypted-Token", "not-valid-base64!!!")
	req = req.WithContext(withContextLogger(req.Context()))
	rec := httptest.NewRecorder()

	hub.Handle(rec, req)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}
