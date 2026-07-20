# Phastos README & Documentation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rewrite `README.md` and create 7 per-module docs under `docs/` — comprehensive, code-example-rich documentation fit for public release.

**Architecture:** `README.md` as landing page (badges, quick-start full app, feature TOC), `docs/*.md` for deep-dive per module. Each doc file is independently readable. All Go code examples are syntactically correct and match actual source signatures.

**Tech Stack:** GitHub-flavored Markdown, Go code blocks

**Spec:** [2026-07-18-readme-design.md](../specs/2026-07-18-readme-design.md)

## Global Constraints

- Documentation only — zero .go file changes
- Remove `go/README.md` (merge into new docs)
- All Go code examples must compile against `github.com/kodekoding/phastos/v2`
- Use relative links between docs (`docs/api.md`, etc.)
- Every function/method name must match actual source code
- Badges preserved from existing README

---

## File Structure

| File | Create/Modify | Responsibility |
|------|--------------|----------------|
| `README.md` | Replace entirely | Landing: badges, overview, install, quick-start app, TOC |
| `docs/api.md` | Create | HTTP framework: App, controllers, routes, groups, handlers, OpenAPI |
| `docs/database.md` | Create | SQL: Connect, Read/Write, transactions, pagination |
| `docs/cache.md` | Create | Redis: Get/Set/HGet/HSet, fallback, singleflight, streams |
| `docs/monitoring.md` | Create | OTEL, NewRelic, composite, spans, trace IDs |
| `docs/integrations.md` | Create | Cron, SSE, Notifications, Mail, Slack socket |
| `docs/utilities.md` | Create | Generators, Storage, Importer, Auth middleware, env, log |
| `docs/configuration.md` | Create | .env reference, all Options functions |
| `go/README.md` | Delete | Content merged into new docs |

---

### Task 1: README.md — Landing Page

**Files:**
- Replace: `README.md`
- Delete: `go/README.md`

**Interfaces:**
- Produces: None (standalone file)
- Links to: `docs/api.md`, `docs/database.md`, `docs/cache.md`, `docs/monitoring.md`, `docs/integrations.md`, `docs/utilities.md`, `docs/configuration.md`

- [ ] **Step 1: Write README.md**

```markdown
# Phastos — Go Development Kit

[![Go Version](https://img.shields.io/github/go-mod/go-version/kodekoding/phastos?color=00ADD8&logo=go&logoColor=white)](https://go.dev)
[![Version](https://img.shields.io/github/v/tag/kodekoding/phastos?color=blue&label=version&logo=git&logoColor=white)](https://github.com/kodekoding/phastos/tags)
[![License](https://img.shields.io/github/license/kodekoding/phastos?color=lightgrey)](LICENSE)
[![CI](https://img.shields.io/github/actions/workflow/status/kodekoding/phastos/auto-tag.yml?color=2088FF&label=CI&logo=github&logoColor=white)](https://github.com/kodekoding/phastos/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/kodekoding/phastos/v2)](https://goreportcard.com/report/github.com/kodekoding/phastos/v2)
[![Go Reference](https://img.shields.io/badge/reference-go.dev-00ADD8?logo=go&logoColor=white)](https://pkg.go.dev/github.com/kodekoding/phastos/v2)

Phastos is a batteries-included Go framework for building production-ready HTTP APIs. It bundles routing (chi/fasthttp), SQL query builder, Redis caching with fallback and singleflight deduplication, OpenTelemetry + NewRelic monitoring, auto-generated OpenAPI 3.0.3 docs, cron scheduler, SSE events, file generators (PDF/CSV/Excel/QR), and multi-platform notifications (Slack/Telegram/FCM) — all wired with sensible defaults.

## Installation

```bash
go get github.com/kodekoding/phastos/v2
```

Requires Go 1.26+.

## Quick Start

A complete API in one `main.go`:

```go
package main

import (
	"context"
	"net/http"

	"github.com/kodekoding/phastos/v2/go/api"
	"github.com/kodekoding/phastos/v2/go/cache"
	"github.com/kodekoding/phastos/v2/go/database"
)

type UserController struct {
	api.ControllerImpl
}

func (c *UserController) GetConfig() api.ControllerConfig {
	return api.ControllerConfig{
		Path: "/users",
		Routes: []api.Route{
			api.NewRoute(http.MethodGet, c.GetList, api.WithPath("/")),
		},
	}
}

func (c *UserController) GetList(req api.Request, ctx context.Context) *api.Response {
	// Get DB and cache from context
	db := c.DB     // set during app.AddController
	cch := c.Cache // set during app.AddController

	// Try cache first with fallback to DB
	type User struct { ID int `json:"id"`; Name string `json:"name"` }
	var users []User
	err := cch.Get(ctx, "users:list", &users, func(ctx context.Context) (any, int64, error) {
		db.Read(ctx, &database.QueryOpts{
			BaseQuery: "SELECT id, name FROM users ORDER BY name",
			Result:    &users,
			IsList:    true,
		})
		return users, 120, nil // cache for 2 minutes
	})
	if err != nil {
		return api.NewResponse().SetError(api.InternalServerError(err.Error(), "CACHE_ERROR"))
	}

	return api.NewResponse().SetData(users)
}

func main() {
	app := api.NewApp(
		api.WithAppPort(8080),
		api.WithAPITimeout(5),
		api.WithTimezone("Asia/Jakarta"),
		api.WithOpenAPI(),
	)
	app.Init()

	// Connect DB
	sql, _ := database.Connect()
	// Connect cache
	cch := cache.New(cache.WithAddress("localhost:6379"))

	// Add controller
	ctrl := &UserController{}
	app.AddController(ctrl)

	app.Start()
}
```

## Key Features

| Module | Description |
|--------|-------------|
| [API](docs/api.md) | chi/fasthttp routing, controllers, route groups, Handler/HandlerV2, OpenAPI auto-gen |
| [Database](docs/database.md) | SQL query builder, Read/Write, transactions, pagination, soft-delete |
| [Cache](docs/cache.md) | Redis with fallback pattern, singleflight dedup, hash operations, streams |
| [Monitoring](docs/monitoring.md) | OpenTelemetry + NewRelic composite, spans, trace IDs, log correlation |
| [Integrations](docs/integrations.md) | Cron scheduler, SSE, Slack/Telegram/FCM notifications, Mandrill/SMTP mail |
| [Utilities](docs/utilities.md) | PDF/CSV/Excel/QR generators, GCS storage, JWT auth, env, logging |
| [Configuration](docs/configuration.md) | All .env keys, App/Cache/DB/Monitoring Options reference |

## Documentation

- **[API Framework](docs/api.md)** — Creating apps, controllers, routes, groups, handlers, middleware, OpenAPI
- **[Database](docs/database.md)** — Connection, Read/Write operations, query builder, transactions
- **[Cache](docs/cache.md)** — Redis operations, fallback pattern, singleflight, hash maps, streams
- **[Monitoring](docs/monitoring.md)** — OpenTelemetry, NewRelic, composite provider, spans, trace IDs
- **[Integrations](docs/integrations.md)** — Cron, SSE, Notifications, Mail, Slack Socket
- **[Utilities](docs/utilities.md)** — File generators, GCS storage, importer, auth middleware, helpers
- **[Configuration](docs/configuration.md)** — Environment variables and Options reference

## License

MIT
```

- [ ] **Step 2: Delete go/README.md**

```bash
rm go/README.md
```

- [ ] **Step 3: Verify README renders correctly**

```bash
# Eye-ball check — no markdown lint errors, links valid
```

- [ ] **Step 4: Commit**

```bash
git add README.md go/README.md
git commit -m "docs: rewrite README.md with quick-start and feature overview"
```

---

### Task 2: docs/api.md — API Framework

**Files:**
- Create: `docs/api.md`

**Interfaces:**
- Consumes: None
- Produces: Standalone doc

- [ ] **Step 1: Write docs/api.md**

Write a `docs/api.md` with these sections and accurate code examples (verify against `go/api/app.go`, `go/api/controller.go`, `go/api/errors.go`, `go/api/response.go`):

```
# API Framework

## Creating an App
- api.NewApp(opts...) — list all Options: WithAppPort, WithAPITimeout, WithTimezone, WithNewRelic, WithOTel, WithOpenAPI, WithFastHttp, WithSSE, WithCronJob, WithPprof, WithGlobalMiddleware, WithSkipLogPaths
- app.Init() — initializes chi router, DB, cron, notifications, middleware registry
- app.Start() — starts HTTP server with graceful shutdown

## Controllers & Routes
- Controller interface { GetConfig() ControllerConfig }
- ControllerConfig: Path, Routes, Middlewares
- Route struct: Method, Path, Handler, Version, Middlewares, SubRoutes, Doc
- NewRoute("GET", handler, WithPath("/list"), WithVersion(1)) — leaf routes
- NewGroup("/admin", []Route{...}, WithMiddleware(auth)) — nested groups
- Path param types: {id:int64}, {slug:string}, {amount:float64}
- app.AddController(ctrl) / app.AddControllers(ctrls)

## Handlers
- Legacy Handler: func(Request, context.Context) *Response
  - Request: GetParams, GetQuery, GetBody, GetFile, GetHeaders
- HandlerV2: func(context.Context) (any, error) — auto-binding via RouteDoc

## Route Documentation & OpenAPI
- WithOpenAPI() option
- RouteDoc fields: Summary, Description, Tags, Deprecated, RequestType, ResponseType, ErrorResponses, Headers, Security
- WithSummary, WithDescription, WithTags, WithRequest, WithResponse, WithErrorResponse
- Serves /docs (Swagger UI) and /docs/openapi.json
- Middleware docs via WithSecurity, WithRequiredHeader, WithMiddlewareDescription

## Middleware
- Global: app.AddGlobalMiddleware(mw)
- Per-controller: ControllerConfig.Middlewares
- Per-route: WithMiddleware(mw1, mw2)
- Built-in: api.JWTAuth, api.StaticAuth, api.RateLimiter
- Custom: api.RegisterMiddlewareFunc(key, mw, opts...)
- Controller-side: UseMiddleware(key), JoinMiddleware(mw1, mw2)

## Response
- NewResponse() / ReleaseResponse(response) — sync.Pool
- SetData, SetMessage, SetError, SetHTTPError, SetFile
- Error helpers: InternalServerError, BadRequest, NotFound, Unauthorized, Forbidden, ConflictError, TooManyRequest, UnprocessableEntity

## Server Configuration
- port, ReadTimeout, WriteTimeout, apiTimeout, timezone, TLS

## Testing Handlers
- app.wrapHandler(handler) returns http.HandlerFunc for httptest
```

- [ ] **Step 2: Verify code examples compile against actual source**

Check each function/method signature used in examples matches `go/api/app.go`, `go/api/controller.go`, etc.

- [ ] **Step 3: Commit**

```bash
git add docs/api.md
git commit -m "docs(api): add comprehensive API framework documentation"
```

---

### Task 3: docs/database.md — SQL Query Builder

**Files:**
- Create: `docs/database.md`

- [ ] **Step 1: Write docs/database.md**

```
# Database / Query Builder

## Connection
- database.Connect() — uses DATABASE_ENGINE, DATABASE_CONN_STRING_MASTER, DATABASE_CONN_STRING_FOLLOWER env vars
- Pool config: DATABASE_MAX_OPEN_CONN, DATABASE_MAX_IDLE_CONN, DATABASE_CONN_MAX_LIFETIME

## Read Operations
- db.Read(ctx, &database.QueryOpts{...})
- QueryOpts fields: BaseQuery, Result, Conditions, IsList, SelectRequest, UseMaster, LockingType, Trx
- SelectRequest / TableRequest: pagination (Page, Limit), search (Keyword, SearchColsStr), ordering (OrderBy), date range, grouping
- Returns into Result (struct or *[]struct)
- SelectResponse{Data, *ResponseMetaData} for lists

## Write Operations
- db.Write(ctx, &database.QueryOpts{...}, isSoftDelete...)
- CUDConstructData: Column/Value mapping for INSERT/UPDATE/UPSERT/DELETE
- CUDResponse: Status, RowsAffected, LastInsertID, Message
- Auto soft-delete (deleted_at timestamp)

## Transactions
- trx, _ := db.Begin()
- defer db.Finish(trx, &err)
- Use trx in QueryOpts.Trx

## CRUD Helpers
- ReadRepo / WriteRepo interfaces
- UsecaseCRUD interface

## Error Handling
- sendNilResponse: maps pg constraint violations to HTTP 409/422
- Slow query detection + Slack notification
```

- [ ] **Step 2: Verify against go/database/sql.go and go/database/definition.go**

- [ ] **Step 3: Commit**

```bash
git add docs/database.md
git commit -m "docs(database): add SQL query builder documentation"
```

---

### Task 4: docs/cache.md — Redis Cache

**Files:**
- Create: `docs/cache.md`

- [ ] **Step 1: Write docs/cache.md**

```
# Redis Cache

## Connection
- cache.New(cache.WithAddress("localhost:6379"), cache.WithDatabaseNo(0))
- Options: WithAddress, WithDatabaseNo, WithTimeout, WithMaxActive, WithMaxIdle, WithMaxRetry, WithPassword, WithUsername

## Basic Operations
- Get(ctx, key, &dest) — auto JSON unmarshal
- Set(ctx, key, value, expireSeconds) — default 10min
- Del(ctx, key) — returns count deleted
- HGet(ctx, key, field, &dest) — hash field get
- HSet(ctx, key, field, value, expireSeconds)
- HDel(ctx, key, field)
- HGetAll(ctx, key, &mapDest)
- HSetBulk(ctx, key, map[string]any, expireSeconds)

## Fallback Pattern
- FallbackFn: func(ctx context.Context) (result any, expire int64, err error)
- Get with fallback: miss → call DB/API → auto-cache → return
- Singleflight: concurrent requests for same key deduplicate fallback calls
- HGet also supports fallback

## Redis Streams
- PublishStream(ctx, streamName, fields...)
- SubscribeStream(ctx, streamName, handlerFn)
- XGroupCreateMkStream, XReadGroup, XAck for consumer groups

## Context Integration
- cch.WrapToHandler(next) — stores cache in request context
- cache.GetCacheFromContext(ctx) — retrieves from context

## Connection Pool
- redigo.Pool with MaxIdle, MaxActive, IdleTimeout, TestOnBorrow (PING)
- MaxRetry on ErrPoolExhausted with 1s backoff
```

- [ ] **Step 2: Verify against go/cache/redis.go interface**

- [ ] **Step 3: Commit**

```bash
git add docs/cache.md
git commit -m "docs(cache): add Redis cache documentation"
```

---

### Task 5: docs/monitoring.md — Observability

**Files:**
- Create: `docs/monitoring.md`

- [ ] **Step 1: Write docs/monitoring.md**

```
# Monitoring (Observability)

## Provider Architecture
- Provider interface: StartSpan, GetTraceId, GetLogLink
- Global functions route to active provider
- NoopProvider by default (safe no-op)

## OpenTelemetry
- monitoring.InitOTelSDK(ctx, OTelConfig{...})
- Auto-init from env: OTEL_EXPORTER_OTLP_ENDPOINT, OTEL_SERVICE_NAME
- W3C trace context propagation
- Batch span export every 5s

## New Relic
- monitoring.InitNewRelic(opts...) or InitNewRelicOnly()
- Envs: NEW_RELIC_APP_NAME, NEW_RELIC_LICENSE_KEY, NEW_RELIC_ACCOUNT_ID
- Transaction per request via NewRelicHTTPMiddleware

## Composite (Both)
- WithNewRelic() + WithOTel() in NewApp — both active simultaneously
- compositeProvider fans out StartSpan to all backends
- compositeSpan fans out End() and SetAttributes()

## Global Functions
- monitoring.StartSpan(ctx, "my-operation")
- monitoring.GetTraceId(ctx) — auto-routes to active provider
- monitoring.GetLogLink(traceId) — clickable log URL

## API Integration
- WithNewRelic() / WithOTel() options on NewApp
- Auto wrapping of HTTP handler with monitoring middleware
```

- [ ] **Step 2: Verify against go/monitoring/monitor.go, otel.go, new_relic.go, composite.go**

- [ ] **Step 3: Commit**

```bash
git add docs/monitoring.md
git commit -m "docs(monitoring): add observability documentation"
```

---

### Task 6: docs/integrations.md — Cron, SSE, Notifications, Mail, Slack

**Files:**
- Create: `docs/integrations.md`

- [ ] **Step 1: Write docs/integrations.md**

```
# Integrations

## Cron Scheduler
- app.AddScheduler("*/5 * * * *", handler) via WithCronJob()
- cron.New(WithTimeZone("Asia/Jakarta"))
- engine.RegisterScheduler(pattern, handler), Start(), Stop()
- Auto Slack notification on job lifecycle (start/finish/error)
- CRON_JOB_TIMEOUT_PROCESS env (default 1 minute)
- HandlerFunc: func(ctx context.Context) *cron.Response

## SSE (Server-Sent Events)
- WithSSE() option on NewApp
- hub := app.SSE(); hub.Broadcast(event)
- Client registration/unregistration
- Message buffering (1000 msg, 24h) for missed messages
- GET /events (stream), GET /events/missed-msg (recovery)

## Notifications
- notifications.New(ActivateSlack(webhook), ActivateTelegram(token), ActivateFirebase(saPath))
- Platforms interface: Slack(), Telegram(), FCM(), GetAllPlatform()
- WrapToHandler stores platform in context
- Auto 500 error notification via response.SetError
- Slack attachments with trace IDs, log links, fields

## Mail (Mandrill & SMTP)
- Mandrills interface: AddRecipient, SetHTMLContent, SetTemplate, SetEmailFrom, Send
- SMTP support

## Slack Socket Mode
- slack.NewSlackApp(appToken, botToken, WithSocketMode(), WithHttp(port))
- app.AddHandler(socketHandler) — slash commands, interactions, events
- Event type dispatch: InteractionType, EventType, EventsAPIType
```

- [ ] **Step 2: Verify against source packages**

- [ ] **Step 3: Commit**

```bash
git add docs/integrations.md
git commit -m "docs(integrations): add cron, sse, notifications, mail, slack docs"
```

---

### Task 7: docs/utilities.md — Generators, Storage, Importer, Auth, Helpers

**Files:**
- Create: `docs/utilities.md`

- [ ] **Step 1: Write docs/utilities.md**

```
# Utilities

## File Generators
- FileGenerator interface: Generate() error, FileName() string
- PDF generator via wkhtmltopdf (requires binary install)
- CSV generator: headers + rows
- Excel generator via excelize: multiple sheets, styling
- QR Code generator via yeqown/go-qrcode
- Banner Image generator via fogleman/gg

## Cloud Storage (GCS)
- Buckets interface: UploadImage, UploadFile, UploadFromLocalPath
- Signed URLs: GetSignedURLFile, GenerateSignedURL
- File management: CopyFileToAnotherBucket, DeleteFile, RollbackProcess

## Data Importer
- CSV/Excel import with worker pool
- Transaction support per batch
- Progress tracking + Slack notifications

## Auth Middleware
- JWTAuth: Authorization: Bearer <token>, HS256, JWT_SIGNING_KEY env
  - Claims stored in context via context.GetJWT(ctx)
- StaticAuth: secret header vs SERVICE_SECRET env
- RateLimiter: token-bucket per IP, 10 rps default
  - WithRate(rps, burst), WithSkipPaths, WithMessage

## Environment
- env.ServiceEnv() → "local"|"development"|"staging"|"production"
- env.IsProduction(), IsStaging(), IsDevelopment(), IsLocal()

## Logging
- log.Get(WithAppPort(8080), WithNewRelicApp(nrApp), WithOTelLogEndpoint())
- log.Ctx(ctx) returns *zerolog.Logger with trace context
- Console writer in dev; JSON + NR/OTel in production

## Helpers
- helper.GenerateJWT(claims, signingKey) / ParseJWT
- helper.EncryptAES / DecryptAES
- helper.SendSlackNotification
- helper.GenerateUUID, GenerateRandomString
```

- [ ] **Step 2: Verify against source packages**

- [ ] **Step 3: Commit**

```bash
git add docs/utilities.md
git commit -m "docs(utilities): add generators, storage, importer, auth, helpers docs"
```

---

### Task 8: docs/configuration.md — Env Vars & Options Reference

**Files:**
- Create: `docs/configuration.md`

- [ ] **Step 1: Write docs/configuration.md**

Create a `docs/configuration.md` with two tables:

**Table 1: Environment Variables**
```
| Variable | Default | Description | Used By |
|----------|---------|-------------|---------|
| APP_NAME | — | Application name | log, monitoring |
| APP_ENV | — | Environment | env, log |
| APP_VERSION | — | Version string | log, monitoring |
| DATABASE_ENGINE | — | "mysql" or "postgres" | database |
| DATABASE_CONN_STRING_MASTER | — | Master DB DSN | database |
| DATABASE_CONN_STRING_FOLLOWER | — | Follower DB DSN | database |
| DATABASE_MAX_OPEN_CONN | 10 | Max open connections | database |
| DATABASE_MAX_IDLE_CONN | 2 | Max idle connections | database |
| DATABASE_CONN_MAX_LIFETIME | 300 | Max connection lifetime (s) | database |
| DATABASE_CONN_MAX_IDLE_TIME | 45 | Max idle time (s) | database |
| REDIS_PREFIX_KEY | phastos: | Key prefix | cache |
| JWT_SIGNING_KEY | — | HS256 signing key | middleware |
| SERVICE_SECRET | — | Static auth secret | middleware |
| NEW_RELIC_APP_NAME | — | NR application name | monitoring |
| NEW_RELIC_LICENSE_KEY | — | NR license key | monitoring |
| NEW_RELIC_ACCOUNT_ID | — | NR account for log links | monitoring |
| OTEL_EXPORTER_OTLP_ENDPOINT | — | OTLP collector URL | monitoring |
| OTEL_SERVICE_NAME | APP_NAME | OTel service name | monitoring |
| OTEL_UI_URL | — | OTel UI for log links | monitoring |
| MONITORING_PROVIDER | — | "otel" or unset (NR) | monitoring |
| SINGLEFLIGHT_ACTIVE | false | Enable request dedup | api |
| CRON_JOB_TIMEOUT_PROCESS | 1 | Cron job timeout (min) | cron |
| NOTIFICATIONS_SLACK_WEBHOOK_URL | — | Slack error webhook | notifications |
| NOTIFICATION_SLACK_INFO_WEBHOOK | — | Slack info webhook | notifications |
| CONTAINER_NAME | — | Container identifier | log |
```

**Table 2: Options Reference**

List every `With*` function with signature and description:
- `api.WithAppPort(int)` — set HTTP port
- `api.WithAPITimeout(int)` — handler timeout in seconds (0 = sync)
- `api.WithTimezone(string)` — timezone (default Asia/Jakarta)
- `api.WithNewRelic()` — enable NR APM
- `api.WithOTel()` — enable OTel tracing
- `api.WithOpenAPI()` — enable OpenAPI docs
- `api.WithFastHttp()` — use fasthttp instead of chi
- `api.WithSSE()` — enable SSE hub
- `api.WithCronJob(string)` — enable cron with timezone
- `api.WithPprof(bool)` — enable pprof
- `api.WithGlobalMiddleware(...func(http.Handler)http.Handler)` — global middlewares
- `api.WithSkipLogPaths(...string)` — paths excluded from request logging
- `cache.WithAddress(string)` — Redis address
- `cache.WithDatabaseNo(int)` — Redis DB number
- `cache.WithTimeout(int)` — connection timeout
- `cache.WithMaxActive(int)` — max pool connections
- `cache.WithMaxIdle(int)` — max idle connections
- `cache.WithMaxRetry(int)` — retry count (default 10)
- `cache.WithPassword(string)` — Redis password
- `cache.WithUsername(string)` — Redis username (Redis 6+)
- `monitoring.InitNewRelic(WithAppName, WithLicenseKey)`
- `monitoring.InitOTelSDK(ctx, OTelConfig)`
- `cron.WithTimeZone(string)`
```

- [ ] **Step 2: Verify all env vars and options exist in source**

- [ ] **Step 3: Commit**

```bash
git add docs/configuration.md
git commit -m "docs(configuration): add env vars and options reference"
```
