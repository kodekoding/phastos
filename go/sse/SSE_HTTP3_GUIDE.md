# SSE and HTTP/3 Compatibility Guide

## Problem

HTTP/3 (QUIC-based) can cause issues with Server-Sent Events due to:
1. Different connection multiplexing model compared to HTTP/1.1/HTTP/2
2. UDP-based protocol not optimized for persistent streaming
3. Increased connection drops and timeouts on long-lived streams

## Solutions

### Option 1: Force HTTP/1.1 or HTTP/2 for SSE (Recommended)

The cleanest solution is to force HTTP/1.1 or HTTP/2 for the SSE endpoint while keeping HTTP/3 for other endpoints.

**Backend Configuration (Go with chi router):**

```go
// In your middleware or route setup
func preventHTTP3ForSSE(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Force HTTP/1.1 or HTTP/2 for SSE
        if r.URL.Path == "/sse" {
            w.Header().Set("Alt-Svc", "")  // Disable QUIC/HTTP3 for this response
        }
        next.ServeHTTP(w, r)
    })
}
```

**Server Configuration (when starting HTTPS server):**

```go
// In your server.go or main.go
server := &http.Server{
    Addr: ":8000",
    // ... other config
}

// If using http2 package
h2s := &http2.Server{}
http2.ConfigureServer(server, h2s)

// Start server
server.ListenAndServeTLS(certFile, keyFile)
```

### Option 2: Disable HTTP/3 Globally (Simple but affects all endpoints)

Add this to your server configuration:

```go
server.Addr = ":8000"
// Don't configure QUIC listener - only HTTP/1.1 and HTTP/2 over TLS
server.ListenAndServeTLS(certFile, keyFile)
```

**In Nginx (if using as reverse proxy):**

```nginx
server {
    listen 443 ssl http2;  # Use HTTP/2, NOT http3
    listen [::]:443 ssl http2;
    
    # Don't add: quic_listen
    
    location /sse {
        proxy_pass http://backend;
        proxy_http_version 1.1;
        proxy_set_header Connection "";
        proxy_set_header Cache-Control "no-cache";
    }
}
```

### Option 3: Implement Polling Fallback for HTTP/3 Clients

Detect HTTP/3 and fall back to polling:

```javascript
// Client-side detection
class AdaptiveSSEClient {
    constructor(endpoint) {
        this.endpoint = endpoint;
        this.usePolling = false;
    }

    async connect() {
        try {
            // Try SSE first
            const response = await fetch(this.endpoint);
            
            // Check if HTTP/3 (via Alt-Svc header or connection info)
            if (response.headers.get('Alt-Svc')?.includes('h3')) {
                console.warn('HTTP/3 detected, using polling instead of SSE');
                this.usePolling = true;
                this.startPolling();
            } else {
                this.startSSE();
            }
        } catch (error) {
            console.error('Connection failed:', error);
            this.usePolling = true;
            this.startPolling();
        }
    }

    startSSE() {
        this.eventSource = new EventSource(this.endpoint);
        this.eventSource.onmessage = (e) => this.handleMessage(e.data);
    }

    startPolling() {
        setInterval(async () => {
            const response = await fetch(this.endpoint + '?poll=true');
            const data = await response.json();
            if (data.events) {
                data.events.forEach(event => this.handleMessage(event));
            }
        }, 5000); // Poll every 5 seconds
    }

    handleMessage(data) {
        // Process message
    }
}
```

### Option 4: Implement Long-Polling with Message Queue

Better reliability than SSE over HTTP/3:

```go
// Backend endpoint for polling
func (s *sseController) poll(ctx api.Context) error {
    clientID := ctx.Query("client_id")
    lastMsgID := ctx.Query("last_msg_id")
    
    // Get buffered messages since last ID
    messages := s.hub.GetMissedMessages(clientID, lastMsgID)
    
    return ctx.JSON(http.StatusOK, map[string]interface{}{
        "events": messages,
        "timestamp": time.Now(),
    })
}
```

## Recommended Approach

1. **For production**: Use Option 1 (Force HTTP/1.1/HTTP/2 for SSE)
2. **For maximum reliability**: Use Option 4 (Long-polling) with message buffering
3. **For development**: Disable HTTP/3 entirely until it's more stable

## Testing HTTP Protocol Version

**Check which HTTP version your client is using:**

```javascript
// In browser console
fetch('/sse').then(r => {
    console.log('HTTP Version:', r.httpVersion || 'Check Network tab');
});
```

**In browser DevTools:**
1. Open Network tab
2. Check the "Protocol" column
3. Should show "h1" (HTTP/1.1), "h2" (HTTP/2), or "h3" (HTTP/3)

## Environment Detection

Add this to your server to disable HTTP/3 conditionally:

```go
// In app.go or server.go
func (app *App) configureHTTP3() {
    if os.Getenv("DISABLE_HTTP3") == "true" {
        // Skip QUIC configuration
        return
    }
    // Configure HTTP/3...
}
```

Then set `DISABLE_HTTP3=true` for environments with HTTP/3 compatibility issues.
