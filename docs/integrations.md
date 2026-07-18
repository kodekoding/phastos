# Integrations

Phastos provides built-in integration support for cron scheduling, Server-Sent Events (SSE), multi-platform notifications, email (Mandrill & SMTP), and Slack Socket Mode.

## Cron Scheduler

The cron package wraps [robfig/cron](https://github.com/robfig/cron) with lifecycle notifications, timeout handling, and Slack alerts on job start/finish/error.

```go
import "github.com/kodekoding/phastos/v2/go/cron"
```

### Standalone Engine

```go
engine := cron.New(cron.WithTimeZone("Asia/Jakarta"))
engine.RegisterScheduler("*/5 * * * *", func(ctx context.Context) *cron.Response {
    return cron.NewResponse().SetProcessName("my-job")
})
engine.Start()
defer engine.Stop()
```

### Via api.NewApp (Recommended)

```go
app := api.NewApp(
    api.WithCronJob("Asia/Jakarta"), // default timezone if omitted
)
app.Init()
app.AddScheduler("0 0 * * *", func(ctx context.Context) *cron.Response {
    return cron.NewResponse().SetProcessName("midnight-cleanup")
})
```

| Method | Description |
|--------|-------------|
| `app.AddScheduler(pattern, handler)` | Register a cron job |
| `app.WrapScheduler(wrapper)` | Add context wrapper to all jobs |

### HandlerFunc

```go
type HandlerFunc func(ctx context.Context) *cron.Response
```

Return `cron.NewResponse().SetError(err).SetProcessName("job-name")` to signal failure. The cron engine sends auto Slack notifications on job lifecycle (start/finish/error).

### Timeout

Set `CRON_JOB_TIMEOUT_PROCESS` env var in minutes (default: `1`). Jobs exceeding the timeout are stopped and a warning Slack notification is sent.

### Wrapper (Context Injection)

Use `cron.Wrapper` to inject values into the cron job context (e.g., DB connection, dependencies):

```go
app.WrapScheduler(myWrapper)
```

---

## SSE (Server-Sent Events)

The SSE package provides an in-memory hub for real-time server-to-client streaming with optional token validation, message buffering, and delivery tracking.

```go
import "github.com/kodekoding/phastos/v2/go/sse"
```

### Enabling SSE

```go
app := api.NewApp(api.WithSSE())
app.Init()
```

This automatically registers two endpoints:
- `GET /events` — SSE event stream
- `GET /events/missed-msg?client_id=<id>&last_received_id=<id>` — missed message recovery

### Broadcasting

```go
hub := app.SSE()
msg := sse.NewSSEMessage("order-updated", map[string]interface{}{
    "order_id": 123,
    "status":   "shipped",
})
hub.Broadcast(msg)
```

### Core Types

```go
type Message struct {
    Event string      // event type (e.g. "connected", "heartbeat", custom)
    Data  interface{} // payload (string, struct, map — auto-serialized to JSON)
    ID    string      // unique message ID
    Retry int         // reconnection delay in ms
}

type Client struct {
    ID      string
    Channel chan *Message
}
```

### Message Buffer

Messages with an `ID` are automatically buffered (1000 messages, 24h TTL) so disconnected clients can retrieve them via `/events/missed-msg`:

```json
// GET /events/missed-msg?client_id=client-123&last_received_id=msg-456
{
    "code": "OK",
    "data": {
        "client_id": "client-123",
        "messages": [...],
        "count": 5
    }
}
```

### Token Validation

```go
hub := app.SSE()
hub.SetTokenValidator(func(token string) (bool, error) {
    return token == "my-secret-key", nil
})

hub.SetEncryptedTokenValidator(func(decryptedToken string) (bool, error) {
    return validateJWT(decryptedToken), nil
})
hub.SetCryptoManager(myCryptoManager)
```

Token sources (checked in order):
1. `X-Encrypted-Token` header → decrypt → validate
2. `Authorization` header (strips `Bearer ` prefix) → validate
3. `token` or `encrypted_token` query param

### Heartbeat & Lifecycle

- Clients receive a `heartbeat` event every 30 seconds
- On connect: `connected` event with `client_id` and `timestamp`
- `hub.GetClientCount()` returns connected clients count
- Hub starts/stops automatically with `app.Start()` / app shutdown

---

## Notifications

Multi-platform notification system supporting Slack (webhook), Telegram (bot), and Firebase Cloud Messaging (FCM).

```go
import "github.com/kodekoding/phastos/v2/go/notifications"
```

### Initialization

```go
notif := notifications.New(
    notifications.ActivateSlack("https://hooks.slack.com/services/..."),
    notifications.ActivateTelegram("123456:ABC-DEF1234ghIkl"),
    notifications.ActivateFirebase("/path/to/service-account.json"),
)
```

### Platforms Interface

```go
type Platforms interface {
    Telegram() Action
    Slack() Action
    FCM() Action
    GetAllPlatform() []Action
    WrapToHandler(next http.Handler) http.Handler
    WrapToContext(ctx context.Context) context.Context
}

type Action interface {
    Send(ctx context.Context, text string, attachment interface{}) error
    IsActive() bool
    Type() string
    SetTraceId(traceId string)
    SetDestination(destination interface{})
}
```

### Using Individual Platforms

```go
// Slack
notif.Slack().SetTraceId(traceID)
notif.Slack().Send(ctx, "Deploy completed", &sgw.Attachment{
    // slack-go-webhook attachment
})

// Telegram
notif.Telegram().SetDestination(chatID) // int64
notif.Telegram().SetTraceId(traceID)
notif.Telegram().Send(ctx, "Server restarted", nil)

// FCM
notif.FCM().SetDestination("device-token")
notif.FCM().SetTraceId(traceID)
notif.FCM().Send(ctx, "New message", &fcm.FCMAttachment{
    Title: "Phastos",
    Data:  map[string]string{"type": "alert"},
})
```

### Auto 500 Notification

Use `WrapToHandler` middleware to inject the notification platform into the request context. The `response.SetError` method automatically sends 500 error notifications via all active platforms:

```go
app.AddGlobalMiddleware(notif.WrapToHandler)
```

### Slack Attachment Details

Slack notifications automatically include:
- **Trace/Request ID** field
- **View Logs** link (via `monitoring.GetLogLink`) — New Relic or OpenTelemetry log URL

Attachments must be `*slack-go-webhook.Attachment` type. Trace ID is auto-generated from `requestId` context value or a new UUID v4.

### FCM Message Structure

```go
type FCMAttachment struct {
    Title string
    Data  map[string]string
}
```

Default notification title is `"timetraq"`, overridable via `FCMAttachment.Title`. Trace ID is injected into `msg.Data["trace_id"]`.

---

## Mail (Mandrill & SMTP)

```go
import "github.com/kodekoding/phastos/v2/go/mail"
```

### Mandrill

Uses [keighl/mandrill](https://github.com/keighl/mandrill). Chained builder API with fluent interface.

```go
m := mail.NewMandrill(&mail.Config{
    SecretKey: "mandrill-api-key",
    EmailFrom: "noreply@example.com",
    FromName:  "My App",
})

err := m.
    AddRecipient("user@example.com", "User Name").
    SetHTMLContent("Welcome!", "<h1>Hello</h1>").
    Send()

// With template
err = m.
    AddRecipient("user@example.com", "User Name").
    SetTemplate("welcome-template", map[string]string{"name": "User"}).
    Send()
```

| Method | Description |
|--------|-------------|
| `AddRecipient(email, name)` | Add a recipient to the message |
| `SetHTMLContent(subject, html)` | Set subject and HTML body |
| `SetTextContent(subject, text)` | Set subject and plain-text body |
| `SetTemplate(name, content)` | Use a Mandrill template |
| `SetEmailFrom(email)` | Override sender email per message |
| `SetFromName(name)` | Override sender name per message |
| `AddAttachment(attachment)` | Add file attachment |
| `SetGlobalMergeVars(data)` | Set template merge variables |
| `Send()` | Send the email and reset for next message |

### SMTP

Fluent builder with embedded HTML template support.

```go
smtp := mail.NewSMTP(
    mail.WithEmail("me@example.com"),
    mail.WithEmailPassword("app-password"),
    mail.WithHost("smtp.gmail.com"),
    mail.WithPort(587),
    mail.WithSender("My App"),
    mail.WithEmailFrom("noreply@example.com"),
)

err := smtp.
    AddRecipient("user@example.com").
    SetContent("Subject Line", "<p>HTML body</p>").
    Send()
```

#### HTML Templates

```go
//go:embed templates/*.html
var templateFS embed.FS

err := smtp.
    AddRecipient("user@example.com").
    SetHTMLTemplate(templateFS, "templates/welcome.html", "Welcome", map[string]string{
        "Name": "User",
    }).
    Send()

// From file path
err = smtp.
    AddRecipient("user@example.com").
    SetHTMLTemplateFromPath("./templates/welcome.html", "Welcome", data).
    Send()
```

| Method | Description |
|--------|-------------|
| `AddRecipient(email...)` | Add one or more recipients |
| `SetSingleRecipient(email)` | Set exactly one recipient (replaces all) |
| `SetContent(subject, message)` | Set subject and HTML body |
| `SetHTMLTemplate(fs, tplFile, subject, args)` | Use embedded HTML template |
| `SetHTMLTemplateFromPath(tplFile, subject, args)` | Use template from file path |
| `Send()` | Send and reset |

---

## Slack Socket Mode

Full Slack Socket Mode support for slash commands, interactive blocks, and events API — all dispatched to typed handlers.

```go
import (
    "github.com/kodekoding/phastos/v2/go/third_party/slack"
    "github.com/kodekoding/phastos/v2/go/third_party/slack/handler"
)
```

### Creating the App

```go
app, err := slack.NewSlackApp(
    "xapp-...",  // SLACK_APP_TOKEN — must start with "xapp-"
    "xoxb-...",  // SLACK_BOT_TOKEN — must start with "xoxb-"
    slack.WithSocketMode(),
    slack.WithHttp(8000),     // optional HTTP server on port
    slack.WithDebug(true),    // optional debug logging
)
if err != nil {
    log.Fatal(err)
}
```

### Adding Handlers

Handlers dispatch based on event type identifier:

```go
type MyHandler struct {
    handler.SocketHandlerImpl
}

func (h *MyHandler) GetConfig() handler.SocketHandlerConfig {
    return handler.SocketHandlerConfig{
        Handler: []handler.SocketEvent{
            // Slash command (identifier starts with "/")
            handler.RegisterEvent(
                h.handleGreet,
                handler.WithEventType("/greet"),
            ),

            // Block action (identifier starts with "action_")
            handler.RegisterEvent(
                h.handleApprove,
                handler.WithEventType("action_approve_button"),
            ),

            // Interaction type dispatch
            handler.RegisterEvent(
                h.handleViewSubmission,
                handler.WithEventType(slack.InteractionTypeViewSubmission),
            ),

            // Event type dispatch
            handler.RegisterEvent(
                h.handleEvent,
                handler.WithEventType(socketmode.EventTypeConnecting),
            ),

            // Events API type dispatch
            handler.RegisterEvent(
                h.handleAppMention,
                handler.WithEventType(slackevents.AppMention),
            ),

            // Default handler (empty type — catches unmatched)
            handler.RegisterEvent(
                h.handleDefault,
            ),
        },
    }
}

func (h *MyHandler) handleGreet(ctx context.Context, req handler.SocketRequest) error {
    data, _ := req.GetSlashCommandData()
    // data.Command, data.Text, data.ChannelID, etc.
    return nil
}

func (h *MyHandler) handleViewSubmission(ctx context.Context, req handler.SocketRequest) error {
    data, _ := req.GetInteractionData()
    // data.View, data.ActionCallback, data.User, etc.
    return nil
}
```

Handler auto-acknowledges requests on success. Pass `false` as `shouldAck` for default handlers.

### SocketRequest Methods

```go
type SocketRequest struct {
    GetSlashCommandData  func() (*slack.SlashCommand, error)
    GetInteractionData   func() (*slack.InteractionCallback, error)
    GetEventData         func() (*slackevents.EventsAPIEvent, error)
    Client               *socketmode.Client
    Event                *socketmode.Event
}
```

### Starting

```go
app.AddHandler(myHandler)
app.Start() // runs socket event loop + optional HTTP server
```

### Event Type Dispatch Matrix

| Prefix/Type | `WithEventType` | Registered As |
|-------------|-----------------|---------------|
| `"/..."` | slash command string | `HandleSlashCommand` |
| `"action_..."` | block action string | `HandleInteractionBlockAction` |
| `slack.InteractionType` | interaction type const | `HandleInteraction` |
| `socketmode.EventType` | event type const | `Handle` |
| `slackevents.EventsAPIType` | events API type const | `HandleEvents` |
| `""` (empty) | none (default) | `HandleDefault` |
