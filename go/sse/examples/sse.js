// Frontend: Reliable SSE with missed message recovery
class ReliableSSEClient {
    constructor(endpoint = 'http://localhost:8000/sse') {
        this.endpoint = endpoint;
        this.eventSource = null;
        this.clientID = null;
        this.lastReceivedMessageID = null;
        this.messageQueue = [];
        this.reconnectAttempts = 0;
        this.maxReconnectAttempts = 10;
        this.reconnectDelay = 1000;
        this.isManuallyDisconnected = false;
    }

    log(message, type = 'info') {
        const console = document.getElementById('console');
        const line = document.createElement('div');
        line.className = `console-line ${type}`;
        const timestamp = new Date().toLocaleTimeString();
        line.textContent = `[${timestamp}] ${message}`;
        console.appendChild(line);
        console.scrollTop = console.scrollHeight;
    }

    updateStatus(status, isConnected) {
        const statusEl = document.getElementById('status');
        statusEl.className = `status ${isConnected ? 'connected' : 'disconnected'}`;
        statusEl.textContent = `Status: ${status}`;
    }

    clearConsole() {
        document.getElementById('console').innerHTML = '';
        log('Console cleared', 'info');
    }

    connect() {
        try {
            this.eventSource = new EventSource(this.endpoint);
            this.setupEventListeners();
            console.log('[SSE] Connecting to:', this.endpoint);
        } catch (error) {
            console.error('[SSE] Connection error:', error);
            this.handleReconnection();
        }
    }

    setupEventListeners() {
        // Connection established
        this.eventSource.addEventListener('connected', (e) => {
            const data = JSON.parse(e.data);
            this.clientID = data.client_id;
            this.reconnectAttempts = 0;
            console.log('[SSE] ✓ Connected successfully');
            console.log('[SSE] Client ID:', this.clientID);

            // Request missed messages if this is a reconnection
            if (this.lastReceivedMessageID) {
                this.requestMissedMessages();
            }
        });

        // Heartbeat to keep connection alive
        this.eventSource.addEventListener('heartbeat', (e) => {
            console.log('[SSE] ♥ Heartbeat received');
        });

        // Generic message listener
        this.eventSource.addEventListener('message', (e) => {
            this.handleMessage(e);
        });

        // Custom events - add more as needed
        this.eventSource.addEventListener('update', (e) => {
            this.handleMessage(e);
        });

        this.eventSource.addEventListener('notification', (e) => {
            this.handleMessage(e);
        });

        // Handle connection errors
        this.eventSource.onerror = (error) => {
            if (this.eventSource.readyState === EventSource.CLOSED) {
                console.error('[SSE] ✗ Connection closed');
                this.handleReconnection();
            } else if (this.eventSource.readyState === EventSource.CONNECTING) {
                console.warn('[SSE] △ Attempting to reconnect...');
            }
        };
    }

    handleMessage(event) {
        try {
            const messageID = event.lastEventId;
            const data = JSON.parse(event.data);

            console.log('[SSE] ✓ Received message:', {
                id: messageID,
                event: event.type,
                data: data,
                timestamp: new Date().toLocaleTimeString(),
            });

            // Update last received message ID for reconnection recovery
            if (messageID) {
                this.lastReceivedMessageID = messageID;
            }

            // Queue the message and process
            this.messageQueue.push({
                id: messageID,
                event: event.type,
                data: data,
                receivedAt: new Date(),
            });

            // Emit custom event so app can listen
            window.dispatchEvent(
                new CustomEvent('sse-message', { detail: data })
            );
        } catch (error) {
            console.error('[SSE] Error processing message:', error);
        }
    }

    // Request missed messages from server
    requestMissedMessages() {
        console.log('[SSE] Requesting missed messages since:', this.lastReceivedMessageID);

        fetch('http://localhost:8000/sse/missed-messages', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                client_id: this.clientID,
                last_received_id: this.lastReceivedMessageID,
            }),
        })
            .then((res) => res.json())
            .then((data) => {
                if (data.messages && data.messages.length > 0) {
                    console.log(
                        '[SSE] Retrieved',
                        data.messages.length,
                        'missed messages'
                    );
                    data.messages.forEach((msg) => {
                        console.log('[SSE] Processing missed message:', msg);
                        // Process missed messages same as live messages
                        window.dispatchEvent(
                            new CustomEvent('sse-missed-message', { detail: msg })
                        );
                    });
                } else {
                    console.log('[SSE] No missed messages');
                }
            })
            .catch((err) =>
                console.error('[SSE] Error retrieving missed messages:', err)
            );
    }

    handleReconnection() {
        if (this.isManuallyDisconnected) return;

        if (this.reconnectAttempts >= this.maxReconnectAttempts) {
            console.error('[SSE] Max reconnection attempts reached');
            return;
        }

        this.reconnectAttempts++;
        const delay = this.reconnectDelay * Math.pow(2, this.reconnectAttempts - 1);

        console.log(
            '[SSE] △ Reconnecting in',
            delay,
            'ms (attempt',
            this.reconnectAttempts,
            ')'
        );

        setTimeout(() => {
            this.connect();
        }, delay);
    }

    disconnect() {
        this.isManuallyDisconnected = true;
        if (this.eventSource) {
            this.eventSource.close();
            console.log('[SSE] Disconnected');
        }
    }

    reconnect() {
        this.isManuallyDisconnected = false;
        this.reconnectAttempts = 0;
        this.connect();
    }

    getMessageQueue() {
        return this.messageQueue;
    }

    clearMessageQueue() {
        this.messageQueue = [];
    }
}

// Usage:
const client = new ReliableSSEClient('http://localhost:8000/sse');
client.connect();

// Listen for SSE messages in your app
window.addEventListener('sse-message', (e) => {
    console.log('Got SSE message:', e.detail);
    // Update your UI with the message
});

// Listen for missed messages
window.addEventListener('sse-missed-message', (e) => {
    console.log('Got missed message:', e.detail);
    // Process missed messages (e.g., update database, sync state)
});

// Manual disconnect
// client.disconnect();

// Manual reconnect
// client.reconnect();