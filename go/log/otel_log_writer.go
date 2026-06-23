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
	conn, err := w.getConn()
	if err != nil {
		return 0, err
	}
	msg := append(p, '\n')
	return conn.Write(msg)
}

func (w *otelTCPWriter) getConn() (net.Conn, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.conn != nil {
		_ = w.conn.SetReadDeadline(time.Now())
		if _, err := w.conn.Read([]byte{}); err != nil {
			_ = w.conn.Close()
			w.conn = nil
		}
	}

	if w.conn == nil {
		conn, err := net.DialTimeout("tcp", w.endpoint, 5*time.Second)
		if err != nil {
			return nil, err
		}
		w.conn = conn
	}

	return w.conn, nil
}
