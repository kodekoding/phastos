package log

import (
	"net"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

type otelTCPWriter struct {
	endpoint string
	conn     net.Conn
	mu       sync.Mutex
}

func newOTelTCPWriter(endpoint string) *otelTCPWriter {
	return &otelTCPWriter{endpoint: endpoint}
}

func (w *otelTCPWriter) WriteLevel(level zerolog.Level, p []byte) (int, error) {
	return w.Write(p)
}

func (w *otelTCPWriter) Write(p []byte) (int, error) {
	conn := w.getConn()
	if conn == nil {
		return len(p), nil
	}

	n, err := conn.Write(p)
	if err != nil {
		w.mu.Lock()
		if w.conn != nil {
			_ = w.conn.Close()
			w.conn = nil
		}
		w.mu.Unlock()
		return len(p), nil
	}
	return n, nil
}

func (w *otelTCPWriter) getConn() net.Conn {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.conn == nil {
		conn, err := net.DialTimeout("tcp", w.endpoint, 5*time.Second)
		if err != nil {
			return nil
		}
		w.conn = conn
	}
	return w.conn
}
