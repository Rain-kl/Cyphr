---
name: "new-api"
description: "Wavelet 项目专用：当新增或修改业务 API、Handler、服务层逻辑、插件路由注册时必须使用。本技能指导基于 Cordis 插件的 API 架构、ctx.Router() 声明式路由注册、Handler/Service 分层、Swagger 与质量门禁。"
---

# 新增业务 API 开发与路由注册规范 (Cordis 插件化架构)

本技能是 Wavelet 在 Cordis 微内核与插件化架构下，进行 HTTP API 接口开发与路由注册的唯一指导规范。

---

## 1. 核心架构哲学：插件自包含 (Self-Contained Plugins)

在 Cordis 架构中，**业务 API 不再集中在旧的 `internal/router/` 或 `internal/apps/` 目录**。
所有业务能力均封装为**高内聚、扁平自包含的插件 (Plugin)**。每个插件自主管理自身的路由声明、中间件挂载、服务逻辑、数据模型与迁移脚本。

### 插件目录推荐结构 (`backend/plugins/domain/<name>/` 或下游 `custom_plugins/<name>/`)

```text
backend/plugins/domain/order/
├── plugin.go        # 插件入口：实现 core.Plugin，通过 ctx.Router() 挂载路由
├── handlers.go      # HTTP 控制器：参数校验、上下文提取、调用 Service、信封响应
├── service.go       # 业务服务层：纯 Go 逻辑，仅依赖 context.Context
├── models.go        # GORM 数据实体定义（自带表前缀）
├── errs.go          # 模块内错误常量定义（camelCase 字符串）
└── migrations/      # 专属嵌入式 Goose SQL 迁移脚本
    └── 20260827000001_create_orders_table.sql
```

---

## 2. 插件契约与路由注册流程

### 步骤 1：定义插件结构并实现 `core.Plugin`

插件必须实现 `core.Plugin` 接口：

```go
package order

import (
	"github.com/Rain-kl/Wavelet/backend/core"
	"github.com/Rain-kl/Wavelet/backend/core/contracts"
)

type Plugin struct {
	svc *OrderService
}

func (p *Plugin) Name() string {
	return "domain.order"
}

func (p *Plugin) Apply(ctx *core.Context) error {
	// 1. 初始化业务 Service
	p.svc = NewOrderService(ctx)

	// 2. 如果需要对外暴露服务，注入 IoC 容器供其他插件消费
	// core.Provide[contracts.OrderService](ctx, p.svc)

	// 3. 注册 HTTP 路由与中间件
	p.registerRoutes(ctx)

	return nil
}
```

### 步骤 2：通过 `ctx.Router()` 挂载路由组与中间件

通过微内核扩展点 `ctx.Router()` 声明式挂载语义化路由与鉴权中间件：

```go
func (p *Plugin) registerRoutes(ctx *core.Context) {
	// 获取认证服务提供的标准中间件（若需要）
	authSvc, _ := core.Inject[contracts.AuthService](ctx)
	
	// 创建带语义化版本前缀的路由组
	group := ctx.Router().Group("/api/v1/orders")
	if authSvc != nil {
		group.Use(authSvc.RequireAuthMiddleware())
	}

	// 绑定 Handler
	group.GET("", p.handleListOrders)
	group.POST("", p.handleCreateOrder)
	group.GET("/:id", p.handleGetOrderDetail)
	group.PUT("/:id/cancel", p.handleCancelOrder)
}
```

---

## 3. Handler 与 Service 职责划分

### Handler 规范 (`handlers.go`)
Handler 负责协议接入层：
1. 参数绑定：使用 `c.ShouldBindJSON` 或 `c.ShouldBindQuery`。
2. 提取当前登录用户信息（如 `oauth.GetCurrentUser(c)`）。
3. 调用底层纯函数或 Service 逻辑。
4. 错误处理：统一使用 `response.Abort*` 系列函数中断请求，禁止直接 `c.JSON(status, response.Err(...))`。
5. 成功响应：使用 `c.JSON(http.StatusOK, response.OK(data))` 或 `response.OKNil()`。
6. 编写完整的 Swagger / OpenAPI 注释。

```go
// @Summary 创建订单
// @Description 创建一笔新的业务订单
// @Tags Order
// @Accept json
// @Produce json
// @Param request body CreateOrderRequest true "创建订单参数"
// @Success 200 {object} response.Envelope{data=OrderDTO} "创建成功"
// @Failure 400 {object} response.Envelope "参数绑定失败"
// @Failure 401 {object} response.Envelope "未授权"
// @Router /api/v1/orders [post]
func (p *Plugin) handleCreateOrder(c *gin.Context) {
	var req CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, errs.ErrBindParamsFailed)
		return
	}

	user, ok := oauth.GetCurrentUser(c)
	if !ok {
		response.AbortUnauthorized(c, errs.ErrUnauthorized)
		return
	}

	order, err := p.svc.CreateOrder(c.Request.Context(), user.ID, req)
	if err != nil {
		// 底层已记录日志，此处根据业务错误码响应
		response.AbortInternal(c, errs.ErrCreateOrderFailed)
		return
	}

	c.JSON(http.StatusOK, response.OK(order))
}
```

### Service / Logics 规范 (`service.go`)
1. 纯 Go 逻辑，第一参数为 `ctx context.Context`，返回 `(result, error)`。
2. **严禁依赖 `*gin.Context`** 或调用 `c.JSON`/`Abort*`。
3. 数据库操作通过 `ctx.DB()` 或受 Trace 保护的 DB 实例完成。
4. 缓存操作通过 `ctx.Cache()` 完成。

---

## 4. 跨插件依赖与防线 (Guardrails)

1. **严禁跨插件 import 内部实现**：插件之间不得直接 import 对方包中的具体结构体或私有逻辑。
2. **面向契约编程**：跨插件调用一律在 `core/contracts/` 中定义 Interface，通过 `core.Provide` 注册、`core.Inject` 或 `ctx.Using` 延迟解析。
3. **事件驱动通知**：涉及跨域状态联动（如用户注册成功、订单支付完成），统一使用 `ctx.Events().Emit(...)` 广播领域事件，由订阅方自愿监听，消除循环依赖。

---

## 5. 质量验证门禁

在完成 API 开发后，必须依次运行以下命令：
```bash
make license        # 确保新文件具有开源许可头
make swagger        # 重新生成 Swagger 文档
make format         # 代码自动格式化
make code-check     # 静态代码质量检查 (golangci-lint)
go test ./plugins/...  # 运行插件单元测试
```
