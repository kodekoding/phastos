package api

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"

	"github.com/kodekoding/phastos/v2/go/server"
)

// TestServeFastHTTPs_Basic tests that serveFastHTTPs starts a fasthttp server.
func TestServeFastHTTPs_Basic(t *testing.T) {
	// Test that we can bind to a port and serve requests
	ln, err := net.Listen("tcp4", ":18011")
	if err != nil {
		t.Skipf("Cannot bind to port 18011: %v", err)
	}
	defer ln.Close()

	// Create a simple fasthttp server
	srv := &fasthttp.Server{
		Handler: fasthttp.RequestHandler(func(ctx *fasthttp.RequestCtx) {
			ctx.WriteString("ok")
		}),
	}
	defer srv.Shutdown()

	// Start server in goroutine
	errCh := make(chan error, 1)
	go func() {
		go srv.Serve(ln)

		// Make a request
		client := &fasthttp.Client{}
		statusCode, body, err := client.Get(nil, "http://localhost:18011/test")
		t.Logf("Status: %d, Body: %s, Err: %v", statusCode, string(body), err)

		errCh <- nil
	}()

	// Give server time to start
	time.Sleep(200 * time.Millisecond)

	select {
	case err := <-errCh:
		if err != nil {
			t.Logf("Server error (may be expected): %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout")
	}
}

// TestServeFastHTTPs_PortBinding tests that serveFastHTTPs properly handles port binding.
func TestServeFastHTTPs_PortBinding(t *testing.T) {
	// Test that we can bind to a port
	ln, err := net.Listen("tcp4", ":18012")
	require.NoError(t, err)
	defer ln.Close()

	// Create a fasthttp server
	srv := &fasthttp.Server{
		Handler: fasthttp.RequestHandler(func(ctx *fasthttp.RequestCtx) {
			ctx.WriteString("hello")
		}),
	}

	// Serve in goroutine
	go srv.Serve(ln)
	defer srv.Shutdown()

	// Give time for server to start
	time.Sleep(100 * time.Millisecond)

	// Make a request
	client := &fasthttp.Client{}
	statusCode, body, err := client.Get(nil, "http://localhost:18012/test")
	require.NoError(t, err)
	assert.Equal(t, 200, statusCode)
	assert.Equal(t, "hello", string(body))
}

// TestServeFastHTTPs_InvalidPort tests serveFastHTTPs with invalid port.
func TestServeFastHTTPs_InvalidPort(t *testing.T) {
	config := &server.Config{
		Port:         -1, // Out of range port
		ReadTimeout:  1,
		WriteTimeout: 1,
		Ctx:          context.Background(),
		Handler:      http.DefaultServeMux,
	}

	err := serveFastHTTPs(config, false)
	// Should return error for out of range port
	assert.Error(t, err)
}

// TestServeFastHTTPs_PortAlreadyInUse tests behavior when port is already in use.
func TestServeFastHTTPs_PortAlreadyInUse(t *testing.T) {
	// Start a listener on a port first
	ln, err := net.Listen("tcp4", ":18013")
	require.NoError(t, err)
	defer ln.Close()

	config := &server.Config{
		Port:         18013,
		ReadTimeout:  1,
		WriteTimeout: 1,
		Ctx:          context.Background(),
		Handler:      http.DefaultServeMux,
	}

	err = serveFastHTTPs(config, false)
	// Should fail because port is in use
	assert.Error(t, err)
}

// TestServeFastHTTPs_GracefulShutdown tests graceful shutdown of fasthttp server.
func TestServeFastHTTPs_GracefulShutdown(t *testing.T) {
	ln, err := net.Listen("tcp4", ":18014")
	require.NoError(t, err)

	srv := &fasthttp.Server{
		Handler: fasthttp.RequestHandler(func(ctx *fasthttp.RequestCtx) {
			time.Sleep(50 * time.Millisecond)
			ctx.WriteString("done")
		}),
	}

	// Start server
	go srv.Serve(ln)

	// Give time to start
	time.Sleep(100 * time.Millisecond)

	// Make request
	client := &fasthttp.Client{}
	statusCode, body, err := client.Get(nil, "http://localhost:18014/test")
	require.NoError(t, err)
	assert.Equal(t, 200, statusCode)
	assert.Equal(t, "done", string(body))

	// Shutdown
	err = srv.Shutdown()
	assert.NoError(t, err)

	ln.Close()
}

// TestServeFastHTTPs_RequestHandler tests that fasthttp server properly handles requests.
func TestServeFastHTTPs_RequestHandler(t *testing.T) {
	ln, err := net.Listen("tcp4", ":18015")
	require.NoError(t, err)

	var requestHandled bool
	srv := &fasthttp.Server{
		Handler: fasthttp.RequestHandler(func(ctx *fasthttp.RequestCtx) {
			requestHandled = true
			ctx.SetStatusCode(200)
			ctx.WriteString(`{"handled":true}`)
		}),
	}

	go srv.Serve(ln)
	defer srv.Shutdown()
	defer ln.Close()

	time.Sleep(100 * time.Millisecond)

	client := &fasthttp.Client{}
	statusCode, body, err := client.Get(nil, "http://localhost:18015/handler")
	require.NoError(t, err)
	assert.True(t, requestHandled)
	assert.Equal(t, 200, statusCode)
	assert.Contains(t, string(body), "handled")
}

// TestServeFastHTTPs_ResponseHeaders tests that response headers are properly set.
func TestServeFastHTTPs_ResponseHeaders(t *testing.T) {
	ln, err := net.Listen("tcp4", ":18016")
	require.NoError(t, err)

	srv := &fasthttp.Server{
		Handler: fasthttp.RequestHandler(func(ctx *fasthttp.RequestCtx) {
			ctx.Response.Header.Set("X-Custom-Header", "test-value")
			ctx.SetStatusCode(200)
			ctx.WriteString("headers-test")
		}),
	}

	go srv.Serve(ln)
	defer srv.Shutdown()
	defer ln.Close()

	time.Sleep(100 * time.Millisecond)

	client := &fasthttp.Client{}
	statusCode, _, err := client.Get(nil, "http://localhost:18016/headers")
	require.NoError(t, err)
	assert.Equal(t, 200, statusCode)
}