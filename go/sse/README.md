# SSE Token Authentication Guide

## Overview

The SSE endpoint (`/sse`) now supports token/API-key authentication to prevent unauthorized access to real-time events. Clients must provide a valid token to establish an SSE connection.

## How It Works

### 1. Token Validation Flow

```
Client Request → SSE Endpoint → Check for Token → Validate Token → Accept/Reject
                                                   ↓
                                    401 (Missing Token)
                                    403 (Invalid Token)
                                    200 (Valid Token)
```

### 2. Token Sources

The endpoint checks for tokens in the following order:

1. **Authorization Header** - `Authorization: Bearer YOUR_TOKEN`
2. **Query Parameter** - `GET /v1/sse?token=YOUR_TOKEN`

The system automatically strips the "Bearer " prefix if present.

## Implementation

### Basic Setup

#### 1. Define Your Token Validator Function

```go
// In loader/app.go or a separate auth package
func MyTokenValidator(token string) (bool, error) {
    // Database lookup example
    user, err := db.GetUserByToken(token)
    if err != nil {
        return false, err
    }
    return user != nil && !user.IsExpired(), nil
}
```

#### 2. Enable SSE with Token Validation

```go
import "loader"

// In your main.go or initialization code
app := loader.NewApp(
    loader.WithSSE(),
    loader.WithSSETokenValidator(MyTokenValidator),
)
```

### Advanced Examples

#### JWT Token Validation

```go
import "github.com/golang-jwt/jwt/v4"

func ValidateJWTToken(tokenString string) (bool, error) {
    token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
        // Verify signing method
        if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
            return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
        }
        // Return your secret key
        return []byte(os.Getenv("JWT_SECRET")), nil
    })
    
    if err != nil {
        return false, err
    }
    
    if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
        // Verify expiration
        if exp, ok := claims["exp"].(float64); ok {
            if int64(exp) < time.Now().Unix() {
                return false, nil // Token expired
            }
        }
        return true, nil
    }
    
    return false, nil
}
```

#### Database Lookup with Caching

```go
import "time"

type APIKeyCache struct {
    keys  map[string]time.Time
    mu    sync.RWMutex
    ttl   time.Duration
}

func (c *APIKeyCache) ValidateToken(token string) (bool, error) {
    c.mu.RLock()
    if expiry, exists := c.keys[token]; exists {
        c.mu.RUnlock()
        if expiry.After(time.Now()) {
            return true, nil
        }
        // Token expired, remove from cache
        c.mu.Lock()
        delete(c.keys, token)
        c.mu.Unlock()
        return false, nil
    }
    c.mu.RUnlock()
    
    // Not in cache, check database
    dbKey, err := db.GetAPIKey(token)
    if err != nil {
        return false, err
    }
    
    if dbKey != nil {
        c.mu.Lock()
        c.keys[token] = time.Now().Add(c.ttl)
        c.mu.Unlock()
        return true, nil
    }
    
    return false, nil
}
```

## Client Usage

### JavaScript/Browser Example

#### Using Authorization Header

```javascript
// Connect with Authorization header
const eventSource = new EventSource('/v1/sse', {
    headers: {
        'Authorization': 'Bearer your-secret-token'
    }
});

eventSource.onopen = () => {
    console.log('SSE Connected');
};

eventSource.addEventListener('message', (event) => {
    console.log('Received:', event.data);
});

eventSource.addEventListener('heartbeat', (event) => {
    console.log('Heartbeat:', event.data);
});

eventSource.onerror = (error) => {
    if (error.status === 401) {
        console.error('Unauthorized: Missing or invalid token');
    } else if (error.status === 403) {
        console.error('Forbidden: Invalid or expired token');
    }
    eventSource.close();
};
```

#### Using Query Parameter

```javascript
const token = 'your-secret-token';
const eventSource = new EventSource(`/v1/sse?token=${encodeURIComponent(token)}`);

// Same event handlers as above
```

#### React Hook Example

```javascript
import { useEffect, useState } from 'react';

function useSSE(token) {
    const [message, setMessage] = useState(null);
    const [error, setError] = useState(null);
    const [isConnected, setIsConnected] = useState(false);

    useEffect(() => {
        const eventSource = new EventSource(`/v1/sse?token=${encodeURIComponent(token)}`);

        eventSource.onopen = () => {
            setIsConnected(true);
            setError(null);
        };

        eventSource.onmessage = (event) => {
            setMessage(JSON.parse(event.data));
        };

        eventSource.addEventListener('heartbeat', () => {
            // Heartbeat received, connection is alive
        });

        eventSource.onerror = () => {
            setIsConnected(false);
            if (eventSource.readyState === EventSource.CLOSED) {
                setError('Connection closed');
            }
            eventSource.close();
        };

        return () => eventSource.close();
    }, [token]);

    return { message, error, isConnected };
}

// Usage
function App() {
    const { message, error, isConnected } = useSSE('your-token');
    
    return (
        <div>
            <p>Status: {isConnected ? 'Connected' : 'Disconnected'}</p>
            {error && <p style={{ color: 'red' }}>{error}</p>}
            {message && <pre>{JSON.stringify(message, null, 2)}</pre>}
        </div>
    );
}
```

### Go Client Example

```go
import "net/http"

func main() {
    token := "your-secret-token"
    client := &http.Client{}
    
    req, err := http.NewRequest("GET", "http://localhost:8000/v1/sse", nil)
    if err != nil {
        log.Fatal(err)
    }
    
    // Set authorization header
    req.Header.Set("Authorization", "Bearer "+token)
    
    resp, err := client.Do(req)
    if err != nil {
        log.Fatal(err)
    }
    
    if resp.StatusCode == http.StatusUnauthorized {
        log.Fatal("Missing or invalid token")
    }
    if resp.StatusCode == http.StatusForbidden {
        log.Fatal("Token is invalid or expired")
    }
    
    defer resp.Body.Close()
    
    // Read SSE messages
    scanner := bufio.NewScanner(resp.Body)
    for scanner.Scan() {
        line := scanner.Text()
        fmt.Println(line)
    }
}
```

## Error Responses

### 401 Unauthorized - Missing Token

```json
{
  "code": "UNAUTHORIZED",
  "message": "Missing token/api-key"
}
```

**Cause**: No token provided in Authorization header or query parameter

**Solution**: Add token to request:
- Header: `Authorization: Bearer YOUR_TOKEN`
- Query: `/v1/sse?token=YOUR_TOKEN`

### 403 Forbidden - Invalid Token

```json
{
  "code": "FORBIDDEN",
  "message": "Invalid or expired token/api-key"
}
```

**Cause**: Token failed validation (invalid, expired, revoked)

**Solution**:
- Verify token is correct
- Check if token has expired
- Refresh token if applicable

### 500 Internal Server Error - Validation Failure

```json
{
  "code": "SERVER_ERROR",
  "message": "Token validation failed"
}
```

**Cause**: Unexpected error during token validation (e.g., database connection error)

**Solution**: Check server logs for detailed error information

## Disabling Token Validation

If you don't provide a token validator, the SSE endpoint will accept connections without authentication:

```go
// SSE without authentication
app := loader.NewApp(
    loader.WithSSE(),
    // No WithSSETokenValidator provided
)
```

To require authentication, always provide a validator:

```go
app := loader.NewApp(
    loader.WithSSE(),
    loader.WithSSETokenValidator(MyValidator), // Required for auth
)
```

## Best Practices

### 1. Token Generation
- Use cryptographically secure random generation
- Store tokens hashed (never plain text)
- Include expiration times

### 2. Token Validation
- Cache valid tokens with TTL for performance
- Log validation failures for security auditing
- Don't log actual token values in error messages

### 3. Transport Security
- Always use HTTPS in production
- Never transmit tokens over unencrypted connections
- Consider short-lived tokens with refresh mechanisms

### 4. Token Rotation
- Implement token expiration
- Provide refresh endpoints for valid users
- Revoke tokens on logout

### 5. Rate Limiting
- Consider rate limiting token validation attempts
- Implement exponential backoff for failed attempts

## Testing

```go
func TestSSEWithValidToken(t *testing.T) {
    validator := func(token string) (bool, error) {
        return token == "valid-token", nil
    }
    
    req := httptest.NewRequest("GET", "/v1/sse", nil)
    req.Header.Set("Authorization", "Bearer valid-token")
    
    // Test your endpoint
}

func TestSSEWithInvalidToken(t *testing.T) {
    validator := func(token string) (bool, error) {
        return token == "valid-token", nil
    }
    
    req := httptest.NewRequest("GET", "/v1/sse", nil)
    req.Header.Set("Authorization", "Bearer invalid-token")
    
    // Should return 403
}

func TestSSEWithoutToken(t *testing.T) {
    validator := func(token string) (bool, error) {
        return token == "valid-token", nil
    }
    
    req := httptest.NewRequest("GET", "/v1/sse", nil)
    // No token header
    
    // Should return 401
}
```

## Troubleshooting

### Connection Refused with 401

- **Issue**: Token not provided
- **Solution**: Add `Authorization: Bearer YOUR_TOKEN` header or `?token=YOUR_TOKEN` query param

### Connection Refused with 403

- **Issue**: Token validation failed
- **Cause**: Invalid token, expired token, or revoked token
- **Solution**: Verify token and refresh if necessary

### Connection Closes Immediately

- **Issue**: Token validation error in validator function
- **Solution**: Check server logs for validator function errors and database connectivity

### No Heartbeat After Connect

- **Issue**: Client connected successfully but receives no heartbeat
- **Solution**: Browser SSE has a 30-second heartbeat; check network connectivity
