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
	DB    database.ISQL
	Cache *cache.Store
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
	db := c.DB
	cch := c.Cache

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
	ctrl := &UserController{
		DB:    sql,
		Cache: cch,
	}
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
