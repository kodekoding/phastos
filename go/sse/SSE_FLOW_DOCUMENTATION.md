# SSEController Structure and Flow Documentation

## Overview
The `sseController` is an implementation of the `Controller` interface that manages Server-Sent Events (SSE) routing and request handling in your v3 API framework.

## Structure Definition

Located in **loader/app.go**, the sseController is defined as:

```go
type sseController struct {
    hub *api.SSEHub  // Reference to the SSE Hub for managing connections
}
```

## Three Core Methods

### 1. **Register() → string**
```go
func (s *sseController) Register() string {
    return "/sse"
}
```

**Purpose**: Defines the base path/prefix for all SSE routes
**Returns**: The root endpoint path for SSE operations
**Called by**: Not directly called by developers - used internally by the framework
**Usage**: When you add this controller to your app, it automatically prefixes all routes with `/sse`

---

### 2. **Routes() → []api.Route**
```go
func (s *sseController) Routes() []api.Route {
    return []api.Route{
        {
            Method:  "GET",
            Path:    "",
            Handler: s.handleSSE,
        },
    }
}
```

**Purpose**: Declares all HTTP routes exposed by this controller
**Returns**: A slice of Route objects that define:
- `Method`: HTTP method (GET, POST, etc.)
- `Path`: Route path (relative to the Register() prefix)
- `Handler`: The handler function to execute

**In this case**:
- **Method**: GET
- **Full Path**: `/v1/sse` (v1 is default version + /sse prefix + empty path)
- **Handler**: s.handleSSE function

**Called by**: The framework automatically calls this when you register the controller

---

### 3. **handleSSE() → error**
```go
func (s *sseController) handleSSE(ctx api.Context) error {
    return s.hub.HandleSSE(ctx)
}
```

**Purpose**: The actual HTTP request handler for SSE connections
**Called by**: The chi router when a client makes a GET request to `/sse`
**What it does**: Delegates the request to the SSEHub which:
1. Converts the response to SSE format
2. Upgrades the connection to streaming
3. Registers the client in the hub
4. Maintains the connection and sends events

---

## Complete Call Flow

### 1️⃣ **Initialization Phase** (in your main/loader)
```go
// Create the app with SSE enabled
apiApp := api.NewApp(
    api.WithAppPort(8000),
    api.WithNewRelic(),
)
apiApp.Init()  // This initializes the SSEHub internally

// In loader/app.go:
func WithSSE() AppOptions {
    return func(a *app) {
        api.WithSSE()(a.App)  // Activates SSEHub in api.App
    }
}
```

### 2️⃣ **Controller Registration Phase**
```go
// Create the controller with hub reference
sseCtrl := &sseController{
    hub: apiApp.SSE(),  // Get the SSEHub instance from App
}

// Register it with the app
apiApp.AddController(sseCtrl)
// OR in Controllers.Register():
// return []Controller{ sseCtrl }
```

### 3️⃣ **Framework Processing**
Inside `app.AddController()`:
```go
func (app *App) AddController(ctrl Controller) {
    config := ctrl.GetConfig()  // Implicitly calls Register() internally
    
    for _, route := range config.Routes {  // Calls Routes() 
        routePath := route.GetVersionedPath(config.Path)
        // routePath becomes: /v1/sse (version + path + route.Path)
        
        handlerFunc := app.wrapHandler(route.Handler)  // Wraps handleSSE()
        app.Http.Method(route.Method, routePath, handler)
        // Registers: GET /v1/sse → s.handleSSE
    }
}
```

### 4️⃣ **Request Processing**
When a client calls `GET /v1/sse`:
```
1. HTTP request arrives
2. Chi router matches /v1/sse
3. Calls app.wrapHandler(s.handleSSE)
4. wrapHandler creates a Request object and context
5. Executes: s.handleSSE(ctx, req)
6. Handler calls: s.hub.HandleSSE(ctx)
7. SSEHub:
   - Converts response to SSE format (text/event-stream)
   - Registers client in the hub's clients map
   - Sends heartbeat events
   - Client receives real-time events from other parts of app
```

---

## How SSE Broadcasting Works

From anywhere in your application, you can broadcast events:

```go
// In a controller or use case
func (uc *MyUsecase) DoSomething(ctx context.Context) error {
    // Do work...
    
    // Get SSE hub from app
    app.SSE().Broadcast(&api.Event{
        Event: "user_created",
        Data:  userData,
    })
    
    return nil
}
```

The SSEHub then:
1. Receives the event
2. Formats it as `event: user_created\ndata: {...}\n\n`
3. Sends to all connected clients
4. All clients receive real-time update

---

## Key Points Summary

| Aspect | Details |
|--------|---------|
| **Register()** | Defines base path prefix (/sse) |
| **Routes()** | Declares HTTP endpoints available |
| **handleSSE()** | Handles incoming SSE requests |
| **When called** | Routes() and Register() are called automatically during app setup |
| **SSE endpoint** | GET /v1/sse (accessible after app starts) |
| **Broadcasting** | Use app.SSE().Broadcast() from anywhere to send events |
| **Client Connection** | Clients connect via EventSource("GET /v1/sse") in JavaScript |

---

## Usage Example

### Server Side (Golang)
```go
// In your loader/app.go or main.go
sseCtrl := &sseController{
    hub: apiApp.SSE(),
}
apiApp.AddController(sseCtrl)

// Later, broadcast an event
apiApp.SSE().Broadcast(&api.Event{
    Event: "notification",
    Data:  "Hello Client!",
})
```

### Client Side (JavaScript)
```javascript
const eventSource = new EventSource('/v1/sse');

eventSource.addEventListener('notification', (event) => {
    console.log('Received:', event.data);
});

eventSource.addEventListener('open', () => {
    console.log('SSE connection established');
});

eventSource.onerror = (error) => {
    console.error('SSE connection error:', error);
    eventSource.close();
};
```

---

## Architecture Diagram

```
App Initialization
    ↓
WithSSE() → Creates SSEHub in api.App
    ↓
sseController created with hub reference
    ↓
AddController(sseCtrl)
    ↓
Register() called → Returns "/sse" prefix
    ↓
Routes() called → Returns [GET route with handleSSE handler]
    ↓
Chi router configured → GET /v1/sse → s.handleSSE
    ↓
Client connects to GET /v1/sse
    ↓
handleSSE() called
    ↓
hub.HandleSSE(ctx) executes
    ↓
Client registered in hub
    ↓
Connection streaming (SSE text/event-stream)
    ↓
App calls hub.Broadcast(event)
    ↓
Hub sends event to all connected clients ✓
