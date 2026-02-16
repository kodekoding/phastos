# SSE Encrypted Token Authentication Guide

## Overview

This guide explains how to implement encrypted API key authentication for SSE (Server-Sent Events) endpoints. The system uses AES-256-GCM encryption to securely transmit API keys, with clients storing only a public key hash (safe to expose).

## Architecture

### Backend Flow
1. Generate API keys and encrypt them with a server-side encryption key
2. Generate public keys (SHA256 hash) of API keys
3. Store public keys in database (never store actual API keys)
4. Validate incoming encrypted tokens by decrypting and verifying public key

### Frontend Flow
1. Store encrypted API key (obtained from server)
2. Store public key (obtained from server)
3. Send encrypted API key with SSE connection request
4. Server decrypts and validates the token

## Backend Implementation

### 1. Initialize CryptoManager

```go
package main

import (
	"os"
	"github.com/kodekoding/phastos/v2/go/api"
	"your-app/loader"
)

func main() {
	// Load encryption key from environment variable
	cryptoManager, err := api.NewCryptoManagerFromEnv("SSE_ENCRYPTION_KEY")
	if err != nil {
		panic(err)
	}

	// Create app with encrypted token validation
	app := loader.NewApp(
		loader.WithSSE(),
		loader.WithSSECryptoManager(cryptoManager),
		loader.WithSSEEncryptedTokenValidator(
			loader.ValidateEncryptedSSEToken,
		),
	)

	// ... rest of initialization
}
```

### 2. Generate API Keys and Public Keys

Create an endpoint to generate and issue API keys to clients:

```go
package controller

import (
	"encoding/json"
	"net/http"
	"github.com/kodekoding/phastos/v2/go/api"
	"your-app/loader"
)

type GenerateAPIKeyResponse struct {
	APIKey    string `json:"api_key"`    // Encrypted API key (send to client)
	PublicKey string `json:"public_key"` // Public key hash (for verification)
}

func GenerateAPIKey(w http.ResponseWriter, r *http.Request) {
	cryptoManager, _ := api.NewCryptoManagerFromEnv("SSE_ENCRYPTION_KEY")

	// Generate a random API key
	apiKey := generateRandomToken() // Your token generation logic

	// Encrypt it
	encryptedKey, err := cryptoManager.Encrypt(apiKey)
	if err != nil {
		http.Error(w, "Failed to encrypt key", http.StatusInternalServerError)
		return
	}

	// Generate public key (safe to store)
	publicKey := api.GeneratePublicKey(apiKey)

	// Store public key in database
	// db.SavePublicKey(publicKey, clientID)

	response := GenerateAPIKeyResponse{
		APIKey:    encryptedKey,
		PublicKey: publicKey,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func generateRandomToken() string {
	// Generate a secure random token (32+ chars)
	// Implementation depends on your needs
	return ""
}
```

### 3. Store Public Keys in Database

```go
// Example: Store public key in PostgreSQL
type APIKeyRecord struct {
	ID        string `db:"id"`
	PublicKey string `db:"public_key"`
	ClientID  string `db:"client_id"`
	CreatedAt time.Time `db:"created_at"`
}

func SavePublicKey(db database.ISQL, publicKey, clientID string) error {
	query := `INSERT INTO api_keys (id, public_key, client_id, created_at) 
	          VALUES ($1, $2, $3, $4)`
	_, err := db.Exec(query, uuid.New().String(), publicKey, clientID, time.Now())
	return err
}
```

### 4. Implement Token Validator with Database

```go
package loader

import (
	"github.com/kodekoding/phastos/v2/go/api"
	"your-app/database"
)

// CreateSSEEncryptedValidatorFromDB creates a validator that checks database
func CreateSSEEncryptedValidatorFromDB(db database.ISQL) api.EncryptedTokenValidator {
	return func(decryptedToken string) (bool, error) {
		publicKey := api.GeneratePublicKey(decryptedToken)

		// Query database for public key
		query := `SELECT COUNT(*) FROM api_keys WHERE public_key = $1 AND deleted_at IS NULL`
		var count int
		err := db.QueryRow(query, publicKey).Scan(&count)
		if err != nil {
			return false, err
		}

		return count > 0, nil
	}
}
```

## Frontend Implementation

### 1. JavaScript/React Example

```javascript
// Step 1: Request API key from backend
async function getAPIKey() {
	const response = await fetch('/api/generate-api-key', {
		method: 'POST',
		headers: { 'Authorization': 'Bearer ' + userToken }
	});

	const data = await response.json();
	
	// Store both encrypted API key and public key
	localStorage.setItem('sseEncryptedToken', data.api_key);
	localStorage.setItem('ssePublicKey', data.public_key);

	return data;
}

// Step 2: Connect to SSE with encrypted token
async function connectToSSE() {
	const encryptedToken = localStorage.getItem('sseEncryptedToken');

	const eventSource = new EventSource(
		`/v1/sse?encrypted_token=${encodeURIComponent(encryptedToken)}`
	);

	eventSource.addEventListener('connected', (event) => {
		const data = JSON.parse(event.data);
		console.log('SSE Connected:', data);
	});

	eventSource.addEventListener('heartbeat', (event) => {
		console.log('Heartbeat received');
	});

	eventSource.addEventListener('message', (event) => {
		const data = JSON.parse(event.data);
		console.log('Received message:', data);
	});

	eventSource.onerror = (error) => {
		console.error('SSE Error:', error);
		eventSource.close();
	};

	return eventSource;
}

// Step 3: Initialize on app startup
document.addEventListener('DOMContentLoaded', async () => {
	// Check if token exists, if not generate new one
	if (!localStorage.getItem('sseEncryptedToken')) {
		await getAPIKey();
	}

	await connectToSSE();
});
```

### 2. React Hook Example

```typescript
import { useEffect, useState } from 'react';

interface APIKeyData {
	api_key: string;
	public_key: string;
}

export const useSSE = () => {
	const [isConnected, setIsConnected] = useState(false);
	const [eventSource, setEventSource] = useState<EventSource | null>(null);

	useEffect(() => {
		const connectSSE = async () => {
			try {
				// Get encrypted token
				let encryptedToken = localStorage.getItem('sseEncryptedToken');

				if (!encryptedToken) {
					// Request new token
					const response = await fetch('/api/generate-api-key', {
						method: 'POST',
					});
					const data: APIKeyData = await response.json();
					encryptedToken = data.api_key;

					localStorage.setItem('sseEncryptedToken', data.api_key);
					localStorage.setItem('ssePublicKey', data.public_key);
				}

				// Connect with encrypted token
				const sse = new EventSource(
					`/v1/sse?encrypted_token=${encodeURIComponent(encryptedToken)}`
				);

				sse.addEventListener('connected', (event) => {
					setIsConnected(true);
					console.log('SSE Connected:', JSON.parse(event.data));
				});

				sse.addEventListener('message', (event) => {
					const message = JSON.parse(event.data);
					console.log('Message received:', message);
				});

				sse.onerror = () => {
					setIsConnected(false);
					sse.close();
				};

				setEventSource(sse);

				return () => {
					sse.close();
				};
			} catch (error) {
				console.error('Failed to connect to SSE:', error);
			}
		};

		connectSSE();
	}, []);

	return { isConnected, eventSource };
};
```

## Security Best Practices

### 1. Encryption Key Management
```go
// NEVER hardcode the encryption key!
// Load from environment variable or secret manager
encryptionKey := os.Getenv("SSE_ENCRYPTION_KEY")

// Recommended: Use AWS Secrets Manager, HashiCorp Vault, etc.
// encryptionKey := secretsManager.GetSecret("sse-encryption-key")
```

### 2. Token Expiration
```go
type APIKeyRecord struct {
	ID        string    `db:"id"`
	PublicKey string    `db:"public_key"`
	ClientID  string    `db:"client_id"`
	CreatedAt time.Time `db:"created_at"`
	ExpiresAt time.Time `db:"expires_at"` // Add expiration
	RevokedAt *time.Time `db:"revoked_at"` // Soft delete
}

// In validator:
func CreateSSEEncryptedValidatorFromDB(db database.ISQL) api.EncryptedTokenValidator {
	return func(decryptedToken string) (bool, error) {
		publicKey := api.GeneratePublicKey(decryptedToken)
		
		query := `SELECT COUNT(*) FROM api_keys 
		          WHERE public_key = $1 
		          AND deleted_at IS NULL 
		          AND revoked_at IS NULL
		          AND expires_at > NOW()`
		
		var count int
		err := db.QueryRow(query, publicKey).Scan(&count)
		return count > 0, err
	}
}
```

### 3. Rate Limiting
```go
// Add rate limiting to the SSE endpoint
func (hub *SSEHub) HandleSSE(w http.ResponseWriter, r *http.Request) {
	clientIP := r.Header.Get("X-Forwarded-For")
	if clientIP == "" {
		clientIP = r.RemoteAddr
	}

	// Check rate limit
	if !rateLimiter.Allow(clientIP) {
		http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
		return
	}

	// ... rest of SSE handling
}
```

### 4. HTTPS Only
```go
// Ensure all SSE connections use HTTPS
func (hub *SSEHub) HandleSSE(w http.ResponseWriter, r *http.Request) {
	// Enforce HTTPS
	if r.Header.Get("X-Forwarded-Proto") != "https" && !isLocalhost(r) {
		http.Error(w, "HTTPS required", http.StatusBadRequest)
		return
	}

	// ... rest of SSE handling
}
```

## API Endpoints

### 1. Generate API Key
**POST /api/generate-api-key**

**Request Headers:**
```
Authorization: Bearer <user_token>
Content-Type: application/json
```

**Response:**
```json
{
	"api_key": "encrypted_token_base64_string",
	"public_key": "public_key_hash"
}
```

### 2. Revoke API Key
**POST /api/revoke-api-key**

**Request:**
```json
{
	"public_key": "public_key_hash"
}
```

### 3. List API Keys
**GET /api/api-keys**

**Response:**
```json
{
	"keys": [
		{
			"public_key": "public_key_hash",
			"created_at": "2024-01-15T10:00:00Z",
			"expires_at": "2025-01-15T10:00:00Z",
			"active": true
		}
	]
}
```

## Troubleshooting

### 1. "Invalid encrypted token" Error
- Verify the encrypted token is correctly encoded in base64
- Ensure the encryption key matches on both client and server
- Check that the token hasn't been modified in transit

### 2. "Token validation failed" Error
- Verify the public key is stored in database
- Check that the token hasn't expired
- Ensure the token hasn't been revoked

### 3. Decryption Failures
- Verify SSE_ENCRYPTION_KEY environment variable is set
- Ensure the key is consistent across all server instances
- Check that the ciphertext format is correct (base64 encoded)

## Complete Example

See `/examples/sse_encrypted_example.go` for a complete working example including:
- API key generation
- Encryption/decryption
- Token validation
- Client connection
