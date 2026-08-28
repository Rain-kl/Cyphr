# Wavelet 异步任务代码示例 (Cordis 插件化架构)

这些示例用于在 Cordis 插件中开发或修改 Asynq 任务时快速套用。

---

## 1. 任务定义与 Handler 编写

在对应的业务插件中（如 `backend/plugins/domain/user/tasks.go`）：

```go
package user

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/hibiken/asynq"
)

// 异步任务类型标识。格式推荐为 "{plugin}:{action}"
const TaskTypeSendEmail = "user:send_email"

type SendEmailPayload struct {
	To      string `json:"to"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

// Handler 处理函数
func (p *Plugin) handleSendEmail(ctx context.Context, t *asynq.Task) error {
	var req SendEmailPayload
	if err := json.Unmarshal(t.Payload(), &req); err != nil {
		return fmt.Errorf("解析任务参数: %w", err)
	}

	req.To = strings.TrimSpace(req.To)
	req.Subject = strings.TrimSpace(req.Subject)
	req.Body = strings.TrimSpace(req.Body)
	if req.To == "" || req.Subject == "" || req.Body == "" {
		return errors.New("to、subject、body 不能为空")
	}

	// 执行实际邮件发送业务逻辑
	return p.emailSvc.Send(ctx, req.To, req.Subject, req.Body)
}
```

---

## 2. 插件内自包含注册 (`Apply`)

在插件的 `Apply(ctx *core.Context)` 中：

```go
package user

import (
	"time"

	"github.com/Rain-kl/Wavelet/core"
	"github.com/Rain-kl/Wavelet/core/extpoints"
)

func (p *Plugin) Apply(ctx *core.Context) error {
	// 1. 注册 Asynq 异步任务消费处理器
	ctx.Task().Register(
		TaskTypeSendEmail,
		p.handleSendEmail,
		extpoints.WithTaskRetry(3),
		extpoints.WithTaskTimeout(2*time.Minute),
	)

	// 2. 注册 Cron 调度任务（例如每天凌晨 3 点清理过期 Token）
	ctx.Schedule().RegisterCron(
		"0 3 * * *",
		"user:cleanup_expired_tokens",
		map[string]any{"scope": "expired"},
	)

	return nil
}
```

---

## 3. 业务中投递异步任务

```go
func (s *UserService) TriggerWelcomeEmail(ctx context.Context, toEmail, username string) error {
	payload, _ := json.Marshal(SendEmailPayload{
		To:      toEmail,
		Subject: "欢迎加入",
		Body:    fmt.Sprintf("你好 %s，欢迎使用我们的平台！", username),
	})

	task := asynq.NewTask(
		TaskTypeSendEmail,
		payload,
		asynq.MaxRetry(3),
	)

	_, err := s.taskClient.EnqueueContext(ctx, task)
	return err
}
```

---

## 4. 任务处理函数单元测试

```go
func TestSendEmailPayloadValidation(t *testing.T) {
	tests := []struct {
		name    string
		payload []byte
		wantErr bool
	}{
		{
			name:    "valid payload",
			payload: []byte(`{"to":"user@example.com","subject":"hi","body":"welcome"}`),
			wantErr: false,
		},
		{
			name:    "empty payload",
			payload: nil,
			wantErr: true,
		},
		{
			name:    "missing required fields",
			payload: []byte(`{"to":"user@example.com","subject":""}`),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req SendEmailPayload
			err := json.Unmarshal(tt.payload, &req)
			if err == nil {
				if strings.TrimSpace(req.To) == "" || strings.TrimSpace(req.Subject) == "" || strings.TrimSpace(req.Body) == "" {
					err = errors.New("missing fields")
				}
			}
			if (err != nil) != tt.wantErr {
				t.Fatalf("validation error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}
```
