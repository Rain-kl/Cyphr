---
name: "new-setting"
description: "Wavelet 项目专用：当新增或修改基于 Cordis 插件的静态配置文件绑定、动态系统/业务设置声明 (ctx.Settings)、管理台热加载设置或前端公共配置消费逻辑时必须使用。"
---

# 插件配置与动态系统设置开发规范 (Cordis 插件化架构)

本技能覆盖 Wavelet 在 Cordis 架构下的静态与动态设置体系。

---

## 1. 两种配置模式与选型

Wavelet 提供两种维度的配置能力：

| 模式 | 机制 | 适用场景 | 注册/读取方式 |
| :--- | :--- | :--- | :--- |
| **静态启动配置** | `config.yaml` / 环境变量 | 进程启动前必须确定、不热更新的配置（如第三方 API Key、端口、物理路径） | `ctx.Config().Bind("plugins.<name>", &cfg)` |
| **动态系统设置** | 数据库持久化 + 缓存 + 热加载 | 运行时可被管理员在管理控制台动态修改的业务规则、开关、阈值 | `ctx.Settings().Register(SettingSchema{...})` |

---

## 2. 插件内配置声明与绑定

### 2.1 静态配置声明与绑定 (`DeclareConfig` 与 `ctx.Config().Bind`)

静态启动配置遵循插件自包含声明与解耦规范：

```go
type OrderStaticConfig struct {
	PaymentGatewayURL string `config:"payment_gateway_url" env:"ORDER_PAYMENT_URL" default:"https://pay.example.com"`
	TimeoutSeconds    int    `config:"timeout_seconds" env:"ORDER_TIMEOUT" default:"30"`
	ApiKey            string `config:"api_key" env:"ORDER_API_KEY" secret:"true"`
}

// 可选：实现 DeclareConfig 声明配置模式（若需门禁求值则实现 core.ConfigGatedPlugin）
func (p *Plugin) DeclareConfig() []core.ConfigBinding {
	return []core.ConfigBinding{
		{Prefix: "plugins.order", Target: &OrderStaticConfig{}},
	}
}

func (p *Plugin) Apply(ctx *core.Context) error {
	var cfg OrderStaticConfig
	// 从统一配置源绑定 plugins.order 节点配置（支持 YAML 与环境变量覆盖）
	_ = ctx.Config().Bind("plugins.order", &cfg)
	return nil
}
```

### 2.2 动态设置注册 (`ctx.Settings().Register`)

插件在 `Apply` 中声明其支持动态调节的 Schema：

```go
func (p *Plugin) Apply(ctx *core.Context) error {
	// 1. 注册内部业务规则设置
	ctx.Settings().Register(extpoints.SettingSchema{
		Key:         "order.auto_cancel_mins",
		Default:     15,
		Description: "未支付订单自动取消时间 (分钟)",
		Category:    "business",
		Public:      false,
	})

	// 2. 注册前端公共可见开关 (Public: true)
	ctx.Settings().Register(extpoints.SettingSchema{
		Key:         "order.invoice_enabled",
		Default:     true,
		Description: "是否开启订单电子发票开具功能",
		Category:    "business",
		Public:      true, // 允许前端通过 /api/v1/config/public 匿名读取
	})

	return nil
}
```

---

## 3. Schema 核心属性说明

- **`Key`**：全局唯一配置键名，推荐小写点分蛇形命名（如 `domain.setting_name`）。
- **`Default`**：默认值（支持 `bool`、`int`、`string`、`JSON 结构`）。
- **`Category`**：
  - `"business"`：业务规则、用户额度、流程开关。
  - `"system"`：系统底层调优、平台安全参数。
- **`Public`**：布尔值。若为 `true`，会自动暴露至 `/api/v1/config/public`，供前端未登录或全局消费。
- **`ReadOnly`**：若为 `true`，管理台仅做展示，禁止通过 API 修改。

---

## 4. 前端消费与管理台界面

### 4.1 前端公共配置消费
当设置声明为 `Public: true` 时，前端可使用 `usePublicConfig` hook 消费：

```tsx
import { usePublicConfig } from "@/hooks/use-public-config";

export function InvoiceButton() {
  const { data: config } = usePublicConfig();
  const invoiceEnabled = config?.["order.invoice_enabled"] === "true";

  if (!invoiceEnabled) return null;
  return <Button>申请开票</Button>;
}
```

### 4.2 管理后台热加载设置 (`/admin/settings` 与 `/admin/system`)
- **`/admin/system`**：通用参数表，自动根据所有已注册的 `SettingSchema` 渲染全量配置项的读写管理。
- **`/admin/settings`**：图形化设置面板。如需在特定的 Tab 中提供高体验的开关/输入组件，参考 `shadcn` 技能使用标准组件进行开发，并通过 `AdminService.updateSystemConfig` 更新。

---

## 5. 质量与验证门禁

```bash
make format
make code-check
go test ./plugins/...
```
