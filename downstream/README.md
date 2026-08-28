# Downstream Custom Plugins

This directory is the designated location for downstream (deployment-specific) Cordis plugins.

## Architecture

```
downstream/
├── README.md
└── plugins/
    └── custom_example/       # Example plugin — copy & rename to get started
        └── plugin.go
```

Downstream plugins follow the same `core.Plugin` contract as platform plugins:

```go
type Plugin interface {
    Name() string
    Apply(ctx *core.Context) error
}
```

## Rules

1. **Naming**: Each plugin directory name becomes its import path and plugin ID (kebab-case recommended).
2. **Dependencies**: Downstream plugins may import `core/`, `core/contracts/`, `pkg/`, and `plugins/infra/` packages from the platform. They MUST NOT import domain plugin internal packages — use `core.Inject[contracts.XxxService](ctx)` instead.
3. **Registration**: Add your downstream plugin to `cmd/app.go` before the platform plugins or after, depending on which services it needs:
   ```go
   // newWaveletApp in cmd/app.go
   app.Use(
       database.New(),
       cache.New(),
       logger.New(),
       storage.New(),
       // ... platform domain plugins ...
       custom_hello.New(), // your downstream plugin
       driver_http.New(driver_http.WithAddr(config.Config.App.Addr)),
       driver_asynq_worker.New(),
       driver_asynq_cron.New(),
   )
   ```
4. **Migration**: If your plugin needs database tables, embed SQL files in a `migrations/` directory and register via `ctx.Migrations().Register(...)` in `Apply()`.

## Quick Start

```go
package custom_example

import (
    "github.com/Rain-kl/Wavelet/core"
    "github.com/Rain-kl/Wavelet/core/contracts"
    "github.com/gin-gonic/gin"
)

type Plugin struct{}

func New() *Plugin { return &Plugin{} }

func (p *Plugin) Name() string { return "custom_example" }

func (p *Plugin) Apply(ctx *core.Context) error {
    // Example: register a route that uses AuthService
    var authSvc contracts.AuthService
    if err := ctx.Using(func(svc contracts.AuthService) { authSvc = svc }); err != nil {
        return err
    }

    g := ctx.Router().Group("/api/v1/custom", authSvc.RequireAuthMiddleware().(gin.HandlerFunc))
    g.GET("/hello", func(c *gin.Context) {
        user, _ := authSvc.GetCurrentUser(c.Request.Context())
        c.JSON(200, gin.H{"message": "Hello " + user.Username})
    })

    return nil
}
```