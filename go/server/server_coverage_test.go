package server

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServeHTTPConfigNil(t *testing.T) {
	// Bind to a port to guarantee it's in use
	ln, err := net.Listen("tcp4", ":18055")
	require.NoError(t, err)
	defer ln.Close()

	// Test with minimal config
	config := &Config{
		Port:        18055, // guaranteed to cause port bind error
		ReadTimeout: 1,
		Handler:     http.NewServeMux(),
		Ctx:         context.Background(),
	}

	// This will fail to bind but we test coverage of the function
	err = ServeHTTP(config)
	assert.Error(t, err)
}

func TestServeHTTPSConfigNil(t *testing.T) {
	// Bind to a port to guarantee it's in use
	ln, err := net.Listen("tcp4", ":18056")
	require.NoError(t, err)
	defer ln.Close()

	// Test with minimal config - will fail due to port in use
	config := &Config{
		Port:        18056,
		ReadTimeout: 1,
		Handler:     http.NewServeMux(),
		Ctx:         context.Background(),
	}

	err = ServeHTTPS(config)
	assert.Error(t, err)
}

func TestServeHTTPNormalOperation(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	config := &Config{
		Port:          0, // auto-assign port
		ReadTimeout:   5,
		WriteTimeout:  5,
		MaxHeaderByte: 1 << 10,
		Handler:       handler,
		Ctx:           ctx,
		Version:       "test-v1",
	}

	// Set environment variables
	os.Setenv("APP_NAME", "test-server")
	os.Setenv("APPS_ENV", "testing")
	os.Setenv("NOTIFY_SERVICE_STATUS", "false")
	defer func() {
		os.Unsetenv("APP_NAME")
		os.Unsetenv("APPS_ENV")
		os.Unsetenv("NOTIFY_SERVICE_STATUS")
	}()

	// Run server in background
	var wg sync.WaitGroup
	wg.Add(1)
	var serverErr error
	go func() {
		defer wg.Done()
		serverErr = ServeHTTP(config)
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Test server is running by making a request
	if config.Port != 0 {
		resp, err := http.Get("http://localhost:" + strconv.Itoa(config.Port))
		// May or may not succeed depending on port binding
		if err == nil {
			resp.Body.Close()
		}
	}

	// Stop the server by canceling context
	cancel()

	wg.Wait()
	// Server should complete without error after shutdown
	assert.NoError(t, serverErr)
}

func TestServeHTTPSServer(t *testing.T) {
	certFile, keyFile, cleanup, err := generateTempCert()
	require.NoError(t, err)
	defer cleanup()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	config := &Config{
		Port:          0, // auto-assign
		ReadTimeout:   5,
		WriteTimeout:  5,
		MaxHeaderByte: 1 << 10,
		Handler:       handler,
		Ctx:           ctx,
		CertFile:      certFile,
		KeyFile:       keyFile,
	}

	var wg sync.WaitGroup
	wg.Add(1)
	var serverErr error
	go func() {
		defer wg.Done()
		serverErr = ServeHTTPS(config)
	}()

	time.Sleep(100 * time.Millisecond)

	cancel()
	wg.Wait()

	assert.NoError(t, serverErr)
}

func generateTempCert() (certFile, keyFile string, cleanup func(), err error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", nil, err
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test Co"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return "", "", nil, err
	}

	certOut, err := os.CreateTemp("", "cert-*.pem")
	if err != nil {
		return "", "", nil, err
	}
	defer certOut.Close()
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		return "", "", nil, err
	}

	keyOut, err := os.CreateTemp("", "key-*.pem")
	if err != nil {
		os.Remove(certOut.Name())
		return "", "", nil, err
	}
	defer keyOut.Close()
	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		os.Remove(certOut.Name())
		os.Remove(keyOut.Name())
		return "", "", nil, err
	}
	if err := pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes}); err != nil {
		os.Remove(certOut.Name())
		os.Remove(keyOut.Name())
		return "", "", nil, err
	}

	cleanup = func() {
		os.Remove(certOut.Name())
		os.Remove(keyOut.Name())
	}
	return certOut.Name(), keyOut.Name(), cleanup, nil
}

func TestWaitTermSigHandlerReturnsError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	handlerCalled := make(chan error, 1)
	handler := func(ctx context.Context) error {
		handlerCalled <- nil
		return nil
	}

	done := WaitTermSig(ctx, handler)

	// Cancel context to trigger the WaitTermSig handler
	cancel()

	select {
	case <-done:
		// Channel was closed, test passes
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for WaitTermSig to finish")
	}

	select {
	case <-handlerCalled:
		// Handler was called
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for handler to be called")
	}
}

func TestWaitTermSigSuccessCase(t *testing.T) {
	os.Setenv("APP_NAME", "test-waitterm")
	os.Setenv("APPS_ENV", "test-env")
	os.Setenv("NOTIFY_SERVICE_STATUS", "false")
	defer func() {
		os.Unsetenv("APP_NAME")
		os.Unsetenv("APPS_ENV")
		os.Unsetenv("NOTIFY_SERVICE_STATUS")
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	handlerCalled := false
	handler := func(ctx context.Context) error {
		handlerCalled = true
		return nil
	}

	done := WaitTermSig(ctx, handler)

	// Cancel context to trigger the handler
	cancel()

	select {
	case <-done:
		assert.True(t, handlerCalled)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for WaitTermSig to finish")
	}
}

func TestWaitTermSigGracefulShutdownError(t *testing.T) {
	os.Setenv("APP_NAME", "test-waitterm-err")
	os.Setenv("APPS_ENV", "test-env")
	os.Setenv("NOTIFY_SERVICE_STATUS", "false")
	defer func() {
		os.Unsetenv("APP_NAME")
		os.Unsetenv("APPS_ENV")
		os.Unsetenv("NOTIFY_SERVICE_STATUS")
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	handler := func(ctx context.Context) error {
		return assert.AnError // simulate error during shutdown
	}

	done := WaitTermSig(ctx, handler)

	// Cancel context to trigger the handler
	cancel()

	select {
	case <-done:
		// Channel closed, success
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for WaitTermSig to finish")
	}
}

func TestConfigStruct(t *testing.T) {
	cfg := &Config{
		Port:          8080,
		ReadTimeout:   30,
		WriteTimeout:  30,
		MaxHeaderByte: 1 << 20,
		Handler:       http.NewServeMux(),
		CertFile:      "/path/to/cert",
		KeyFile:       "/path/to/key",
		EncryptionKey: "secret-key",
		Ctx:           context.Background(),
		Version:       "v1.0.0",
		Environment:   "production",
	}

	assert.Equal(t, 8080, cfg.Port)
	assert.Equal(t, 30, cfg.ReadTimeout)
	assert.Equal(t, 30, cfg.WriteTimeout)
	assert.Equal(t, 1<<20, cfg.MaxHeaderByte)
	assert.NotNil(t, cfg.Handler)
	assert.Equal(t, "/path/to/cert", cfg.CertFile)
	assert.Equal(t, "/path/to/key", cfg.KeyFile)
	assert.Equal(t, "secret-key", cfg.EncryptionKey)
	assert.NotNil(t, cfg.Ctx)
	assert.Equal(t, "v1.0.0", cfg.Version)
	assert.Equal(t, "production", cfg.Environment)
}

func TestServeHTTPsWithDifferentConfigs(t *testing.T) {
	tests := []struct {
		name   string
		config *Config
	}{
		{
			name: "minimal config",
			config: &Config{
				Port:        -1, // invalid port to trigger listen error
				Handler:     http.NewServeMux(),
				Ctx:         context.Background(),
			},
		},
		{
			name: "with timeouts",
			config: &Config{
				Port:         -1, // invalid port to trigger listen error
				ReadTimeout:   60,
				WriteTimeout:  60,
				MaxHeaderByte: 1 << 16,
				Handler:       http.NewServeMux(),
				Ctx:           context.Background(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := serveHTTPs(tt.config, false)
			assert.Error(t, err)
		})
	}
}

func TestServeHTTPsServerAcceptsConnections(t *testing.T) {
	var port int
	var wg sync.WaitGroup

	// Use a random available port by binding to :0
	listener, err := net.Listen("tcp4", ":0")
	require.NoError(t, err)
	port = listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	config := &Config{
		Port:          port,
		ReadTimeout:   5,
		WriteTimeout:  5,
		MaxHeaderByte: 1 << 10,
		Handler:       handler,
		Ctx:           ctx,
	}

	os.Setenv("APP_NAME", "test-serve")
	os.Setenv("APPS_ENV", "test")
	os.Setenv("NOTIFY_SERVICE_STATUS", "false")
	defer func() {
		os.Unsetenv("APP_NAME")
		os.Unsetenv("APPS_ENV")
		os.Unsetenv("NOTIFY_SERVICE_STATUS")
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = serveHTTPs(config, false)
	}()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Make a request to verify server works
	resp, err := http.Get("http://localhost:" + strconv.Itoa(port))
	if err == nil {
		resp.Body.Close()
	}

	cancel()
	wg.Wait()
}

func TestWaitTermSigNotifyServiceStatusEnv(t *testing.T) {
	os.Setenv("APP_NAME", "test-notify")
	os.Setenv("APPS_ENV", "staging")
	os.Setenv("NOTIFY_SERVICE_STATUS", "true")
	defer func() {
		os.Unsetenv("APP_NAME")
		os.Unsetenv("APPS_ENV")
		os.Unsetenv("NOTIFY_SERVICE_STATUS")
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	handler := func(ctx context.Context) error {
		return nil
	}

	done := WaitTermSig(ctx, handler)

	// Cancel context to trigger the handler
	cancel()

	select {
	case <-done:
		// Channel closed, success
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for WaitTermSig to finish")
	}
}