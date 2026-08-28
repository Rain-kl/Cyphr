---
name: "new-async-task"
description: "Wavelet 项目专用：新增或修改基于 Cordis 插件的 Asynq 异步任务、后台 Worker 消费处理器、Cron 定时调度任务与任务执行追踪时必须使用。"
---

# 异步任务与定时调度开发规范 (Cordis 插件化架构)

本技能是 Wavelet 在 Cordis 微内核与插件化架构下，进行 Asynq 异步后台任务与 Cron 定时调度开发的唯一指导规范。

---

## 1. 核心架构：插件内自包含任务声明

在 Cordis 架构中，后台 Worker 消费与定时调度**不再集中在中心化的注册表**，而是由各个业务插件在自身的 `Apply` 方法中通过微内核扩展点直接声明。

### 扩展点矩阵

| 扩展点方法 | 说明 | 适用场景 |
| :--- | :--- | :--- |
| `ctx.Task().Register(pattern, handler, opts...)` | 注册 Asynq 任务类型与消费处理器 | 异步耗时计算、队列任务、通知外发 |
| `ctx.Schedule().RegisterCron(spec, taskType, payload)` | 注册 Cron 表达式定时调度任务 | 周期统计、定时清理、健康检查 |

---

## 2. 异步任务开发全流程

### 步骤 1：定义任务 Payload 结构与类型常量

在插件内（如 `backend/plugins/domain/order/tasks.go`）：

```go
package order

import (
	"context"
	"encoding/json"
	"time"

	"github.com/hibiken/asynq"
)

const (
	TaskTypeOrderTimeoutCancel = "order:timeout_cancel"
)

// OrderTimeoutPayload 定义任务入参
type OrderTimeoutPayload struct {
	OrderID   string `json:"order_id"`
	Reason    string `json:"reason"`
	CreatedAt int64  `json:"created_at"`
}
```

### 步骤 2：实现任务执行处理器 (Handler)

Handler 必须接受 `ctx context.Context, t *asynq.Task`，返回 `error`：

```go
func (p *Plugin) handleOrderTimeoutCancel(ctx context.Context, t *asynq.Task) error {
	var payload OrderTimeoutPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return err // 反序列化失败，直接中断
	}

	// 记录任务日志
	// task.AppendLog(ctx, "开始处理订单超时关单: order_id=%s", payload.OrderID)

	// 执行业务逻辑
	if err := p.svc.CancelTimeoutOrder(ctx, payload.OrderID, payload.Reason); err != nil {
		// 返回 error 触发 Asynq 框架自动重试
		return err
	}

	return nil
}
```

### 步骤 3：在插件 `Apply` 中注册任务与定时调度

```go
func (p *Plugin) Apply(ctx *core.Context) error {
	// 1. 注册异步任务处理器
	ctx.Task().Register(
		TaskTypeOrderTimeoutCancel,
		p.handleOrderTimeoutCancel,
		extpoints.WithTaskRetry(3),
		extpoints.WithTaskTimeout(5*time.Minute),
	)

	// 2. 注册定时调度任务 (例如每天凌晨 2 点执行汇总)
	ctx.Schedule().RegisterCron(
		"0 2 * * *",
		"order:daily_settlement",
		map[string]any{"scope": "all"},
	)

	return nil
}
```

### 步骤 4：在业务逻辑中投递异步任务

当业务需要下发延迟或异步任务时：

```go
func (s *OrderService) EnqueueTimeoutCheck(ctx context.Context, orderID string) error {
	payloadBytes, _ := json.Marshal(OrderTimeoutPayload{
		OrderID:   orderID,
		Reason:    "15分钟未支付自动关单",
		CreatedAt: time.Now().Unix(),
	})

	task := asynq.NewTask(
		TaskTypeOrderTimeoutCancel,
		payloadBytes,
		asynq.ProcessIn(15*time.Minute), // 延迟 15 分钟执行
		asynq.MaxRetry(3),
	)

	// 投递到任务客户端
	_, err := s.taskClient.EnqueueContext(ctx, task)
	return err
}
```

---

## 3. 运行切面透明性 (Profile Transparency)

Cordis 微内核支持多种启动切面（`api`、`worker`、`schedule`、`all`）：
- 插件开发者**无需在插件代码中编写 `if mode == "worker"` 分支**。
- 插件只需在 `Apply` 中把任务与调度注册进 `Context`。
- 当进程以 `worker` 切面启动时，微内核的 `driver_asynq_worker` 驱动会自动拾取并监听已注册的任务。
- 当进程以 `schedule` 切面启动时，`driver_asynq_cron` 驱动会自动启动调度器引擎。

---

## 4. 任务日志与重试规范

1. **日志记录**：
   - 记录任务启动参数摘要、分批处理进度及最终完成统计。
   - 大循环处理中应按批次记录日志，禁止每条数据单独打日志刷屏。
2. **重试机制**：
   - Handler 返回 error 即自动触发 Asynq 重试策略。
   - 禁止在 Handler 内部编写裸 `for` 死循环重试。
3. **幂等性保障**：
   - 任务由于网络波动或超时可能被重复消费，业务操作必须实现幂等保护（如基于订单状态机检查或分布式锁 `ctx.DistLock()`）。

---

## 5. 质量验证

```bash
make format
make code-check
go test ./plugins/...
```
