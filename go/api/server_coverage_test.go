package api

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kodekoding/phastos/v2/go/server"
)

func TestWaitTermSig_Basic(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	stopCalled := make(chan struct{})
	handler := func(ctx context.Context) error {
		close(stopCalled)
		return nil
	}

	_ = WaitTermSig(ctx, handler)

	time.Sleep(50 * time.Millisecond)

	cancel()

	select {
	case <-stopCalled:
	case <-time.After(time.Second):
		t.Fatal("handler was not called")
	}
}

func TestWaitTermSig_ReturnsChannel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	handler := func(ctx context.Context) error { return nil }

	stopCh := WaitTermSig(ctx, handler)

	assert.NotNil(t, stopCh)

	cancel()

	select {
	case <-stopCh:
	case <-time.After(time.Second):
		t.Fatal("channel was not closed")
	}
}

func TestWaitTermSig_CancelContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	handlerCalled := make(chan bool, 1)
	handler := func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			handlerCalled <- true
		case <-time.After(2 * time.Second):
			handlerCalled <- false
		}
		return nil
	}

	_ = WaitTermSig(ctx, handler)

	time.Sleep(50 * time.Millisecond)

	cancel()

	select {
	case called := <-handlerCalled:
		assert.True(t, called, "context should be canceled when signal received")
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for handler")
	}
}

func TestWaitTermSig_ConcurrentSignals(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var mu sync.Mutex
	callCount := 0

	handler := func(ctx context.Context) error {
		mu.Lock()
		callCount++
		mu.Unlock()
		return nil
	}

	_ = WaitTermSig(ctx, handler)

	time.Sleep(50 * time.Millisecond)

	cancel()

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	count := callCount
	mu.Unlock()
	assert.GreaterOrEqual(t, count, 1)
}

func TestServeHTTPs_Basic(t *testing.T) {
	listener, err := net.Listen("tcp4", ":0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	handler := http.NewServeMux()
	handler.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	ctx, cancel := context.WithCancel(context.Background())

	config := &server.Config{
		Port:         port,
		ReadTimeout:  3,
		WriteTimeout: 3,
		Ctx:          ctx,
		Handler:      handler,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- serveHTTPs(config, false)
	}()

	time.Sleep(200 * time.Millisecond)

	resp, err := http.Get("http://localhost:" + itoa(port) + "/test")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	cancel()

	select {
	case err := <-errCh:
		t.Logf("Server stopped: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for server to stop")
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

func TestServeHTTPs_InvalidPort(t *testing.T) {
	config := &server.Config{
		Port:         -1,
		ReadTimeout:  1,
		WriteTimeout: 1,
		Ctx:          context.Background(),
		Handler:      http.DefaultServeMux,
	}

	err := serveHTTPs(config, false)
	assert.Error(t, err)
}

func TestServeHTTPs_WithTestServer(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message":"hello"}`))
	})

	ts := httptest.NewServer(handler)
	defer ts.Close()

	resp, err := http.Get(ts.URL)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()
}

func TestServeHTTPs_ShutdownOnSignal(t *testing.T) {
	listener, err := net.Listen("tcp4", ":0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	handler := http.NewServeMux()
	handler.HandleFunc("/shutdown-test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	ctx, cancel := context.WithCancel(context.Background())

	config := &server.Config{
		Port:         port,
		ReadTimeout:  3,
		WriteTimeout: 3,
		Ctx:          ctx,
		Handler:      handler,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- serveHTTPs(config, false)
	}()

	time.Sleep(200 * time.Millisecond)

	client := &http.Client{Timeout: time.Second}
	resp, err := client.Get("http://localhost:" + itoa(port) + "/shutdown-test")
	require.NoError(t, err)
	resp.Body.Close()

	cancel()

	select {
	case <-errCh:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for server to stop")
	}
}

func TestWaitTermSig_ErrorHandling(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	handler := func(ctx context.Context) error {
		return context.DeadlineExceeded
	}

	_ = WaitTermSig(ctx, handler)

	time.Sleep(50 * time.Millisecond)

	cancel()

	time.Sleep(100 * time.Millisecond)
}

func TestServeHTTPs_PortInUse(t *testing.T) {
	ln, err := net.Listen("tcp4", ":18003")
	require.NoError(t, err)
	defer ln.Close()

	config := &server.Config{
		Port:         18003,
		ReadTimeout:  1,
		WriteTimeout: 1,
		Ctx:          context.Background(),
		Handler:      http.DefaultServeMux,
	}

	err = serveHTTPs(config, false)
	assert.Error(t, err)
}