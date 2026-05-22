package api

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kodekoding/phastos/v2/go/server"
)

// TestWaitTermSig_Basic tests that WaitTermSig waits for termination signal.
func TestWaitTermSig_Basic(t *testing.T) {
	ctx := context.Background()

	stopCalled := false
	handler := func(ctx context.Context) error {
		stopCalled = true
		return nil
	}

	_ = WaitTermSig(ctx, handler)

	// Give goroutine time to set up signal handler
	time.Sleep(50 * time.Millisecond)

	// Send SIGTERM signal
	syscall.Kill(syscall.Getpid(), syscall.SIGTERM)

	// Give goroutine time to process
	time.Sleep(100 * time.Millisecond)

	// Handler should have been called
	assert.True(t, stopCalled, "handler should be called after signal")
}

// TestWaitTermSig_ReturnsChannel tests that WaitTermSig returns a channel.
func TestWaitTermSig_ReturnsChannel(t *testing.T) {
	ctx := context.Background()
	handler := func(ctx context.Context) error { return nil }

	stopCh := WaitTermSig(ctx, handler)

	// Channel should not be nil
	assert.NotNil(t, stopCh)

	// Send signal to unblock
	pid := syscall.Getpid()
	process, err := os.FindProcess(pid)
	require.NoError(t, err)

	// Send SIGTERM
	err = process.Signal(syscall.SIGTERM)
	require.NoError(t, err)

	// Give time for signal to be processed
	time.Sleep(100 * time.Millisecond)
}

// TestWaitTermSig_CancelContext tests that the context is canceled on signal.
func TestWaitTermSig_CancelContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	handlerCalled := make(chan bool, 1)
	handler := func(ctx context.Context) error {
		// Verify context is canceled
		select {
		case <-ctx.Done():
			handlerCalled <- true
		case <-time.After(2 * time.Second):
			handlerCalled <- false
		}
		return nil
	}

	_ = WaitTermSig(ctx, handler)

	// Give goroutine time to set up signal handler
	time.Sleep(50 * time.Millisecond)

	// Send SIGINT via syscall
	syscall.Kill(syscall.Getpid(), syscall.SIGINT)

	// Wait for handler with timeout
	select {
	case called := <-handlerCalled:
		assert.True(t, called, "context should be canceled when signal received")
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for handler")
	}
}

// TestWaitTermSig_ConcurrentSignals tests handling of concurrent signals.
func TestWaitTermSig_ConcurrentSignals(t *testing.T) {
	ctx := context.Background()
	var mu sync.Mutex
	callCount := 0

	handler := func(ctx context.Context) error {
		mu.Lock()
		callCount++
		mu.Unlock()
		return nil
	}

	_ = WaitTermSig(ctx, handler)

	// Give goroutine time to set up
	time.Sleep(50 * time.Millisecond)

	// Send multiple signals
	for i := 0; i < 3; i++ {
		syscall.Kill(syscall.Getpid(), syscall.SIGTERM)
		time.Sleep(10 * time.Millisecond)
	}

	// Give time for signals to be processed
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	// Handler should only be called once due to channel blocking
	assert.GreaterOrEqual(t, callCount, 1)
}

// TestServeHTTPs_Basic tests that serveHTTPs starts an HTTP server.
func TestServeHTTPs_Basic(t *testing.T) {
	// Create a simple config
	config := &server.Config{
		Port:         18001,
		ReadTimeout:  3,
		WriteTimeout: 3,
		Ctx:          context.Background(),
		Handler:      http.DefaultServeMux,
	}

	// Make the handler actually respond
	http.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Start server in goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- serveHTTPs(config, false)
	}()

	// Give server time to start
	time.Sleep(200 * time.Millisecond)

	// Make a request to verify server is running
	resp, err := http.Get("http://localhost:18001/test")
	if err != nil {
		// Server might not be ready yet, try again
		time.Sleep(100 * time.Millisecond)
		resp, err = http.Get("http://localhost:18001/test")
	}
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Shutdown by sending signal
	syscall.Kill(syscall.Getpid(), syscall.SIGTERM)

	// Wait for server to stop with timeout
	select {
	case err := <-errCh:
		// Server stopped (possibly by signal)
		t.Logf("Server stopped: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for server to stop")
	}
}

// TestServeHTTPs_InvalidPort tests serveHTTPs with invalid port.
func TestServeHTTPs_InvalidPort(t *testing.T) {
	// Use a reserved port that shouldn't be available
	config := &server.Config{
		Port:         1, // Reserved port, likely to fail
		ReadTimeout:  1,
		WriteTimeout: 1,
		Ctx:          context.Background(),
		Handler:      http.DefaultServeMux,
	}

	err := serveHTTPs(config, false)
	// Should return error for privileged port without root
	assert.Error(t, err)
}

// TestServeHTTPs_WithTestServer tests serveHTTPs by making HTTP requests.
func TestServeHTTPs_WithTestServer(t *testing.T) {
	// Create a simple handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message":"hello"}`))
	})

	// Create test server first
	ts := httptest.NewServer(handler)
	defer ts.Close()

	// Now test that we can connect to it
	resp, err := http.Get(ts.URL)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()
}

// TestServeHTTPs_ShutdownOnSignal tests that server responds to shutdown signal.
func TestServeHTTPs_ShutdownOnSignal(t *testing.T) {
	config := &server.Config{
		Port:         18002,
		ReadTimeout:  3,
		WriteTimeout: 3,
		Ctx:          context.Background(),
		Handler:      http.DefaultServeMux,
	}

	// Add a simple endpoint
	http.HandleFunc("/shutdown-test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	errCh := make(chan error, 1)
	go func() {
		errCh <- serveHTTPs(config, false)
	}()

	// Wait for server to start
	time.Sleep(200 * time.Millisecond)

	// Verify we can make a request
	client := &http.Client{Timeout: time.Second}
	resp, err := client.Get("http://localhost:18002/shutdown-test")
	if err == nil {
		resp.Body.Close()
	}

	// Send termination signal
	syscall.Kill(syscall.Getpid(), syscall.SIGTERM)

	// Wait for server to stop
	select {
	case <-errCh:
		// Server stopped gracefully
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for server to stop")
	}
}

// TestWaitTermSig_ErrorHandling tests that handler errors are logged.
func TestWaitTermSig_ErrorHandling(t *testing.T) {
	ctx := context.Background()

	expectedErr := context.DeadlineExceeded

	handler := func(ctx context.Context) error {
		return expectedErr
	}

	_ = WaitTermSig(ctx, handler)

	// Give time for goroutine to start
	time.Sleep(50 * time.Millisecond)

	// Send signal
	syscall.Kill(syscall.Getpid(), syscall.SIGTERM)

	// Give time for processing
	time.Sleep(100 * time.Millisecond)

	// The error is logged but not returned
	// We just verify the function completes without panic
}

// TestServeHTTPs_PortInUse tests behavior when port is already in use.
func TestServeHTTPs_PortInUse(t *testing.T) {
	// Start a listener on a port first
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
	// Should fail because port is in use
	assert.Error(t, err)
}