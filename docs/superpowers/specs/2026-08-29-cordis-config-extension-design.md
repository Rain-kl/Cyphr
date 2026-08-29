# Cordis 配置扩展点设计 (Config Extension Point)

- **文档状态**: 已敲定 (Approved)
- **版本**: v1.0.0 (2026-08-29)
- **适用范围**: `backend/core/`(微内核)、`backend/plugins/`(自包含插件)、`backend/cmd/`(组合根)、`backend/pkg/`(无状态基础库)

---

## 0. 背景与动机

`backend/pkg/config` 同时承担了三件事：viper 装载 `config.yaml`、环境变量覆盖、以及以全局单例 `config.Config` 暴露全量配置模型。它与架构文档对 `backend/pkg/` 的定位（"Stateless utilities and algorithm libraries"）冲突，并且带来两个结构性问题：

1. **配置所有权倒挂**：任何包都能读到全量配置，因此 `cmd` 直接替 `cache` 插件判断 Redis 是否启用、`risk_control` 直接判断 `clickhouse.enabled`。配置的"读者"与"所有者"没有关系约束。
2. **隐式全局状态**：`init()` 内完成文件搜索、解析与 `log.Fatalf`，并以 `isTest()` 猜测执行上下文来禁用数据库/Redis/ClickHouse；测试通过改写全局单例驱动生产代码路径。

本设计把"配置的读取框架"下沉为内核扩展点，把"读哪些字段"的所有权交给各插件自己声明，并一次性迁移全部 27 个消费文件（109 处引用），彻底删除全局单例。

`AGENTS.md` 与 `new-setting` skill 中早已写明插件应通过 `ctx.Config().Bind(...)` 绑定静态配置，但该 API 在代码中从未存在——本设计同时修正这一文档漂移。

---

## 1. 决策记录

| # | 决策 | 理由与取舍 |
| :--- | :--- | :--- |
| D1 | **预声明阶段 + 配置门禁** | 内核在 `Apply` 之前收集声明并求值门禁，使组合根不再跨插件读配置选实现。代价是给 Fiber 增加"被门禁跳过"语义。 |
| D2 | **混合读取形态：结构体 `Bind` + 泛型 `Get`** | `redis`/`database` 等 14+ 字段结构体整体消费，逐 key 声明不可读；`app.session_secret` 等单字段不值得为它绑一个结构体。 |
| D3 | **共享声明 + 内核冲突校验** | 配置是进程级只读事实，不存在数据表那种写竞争，因此允许读者各自声明同一 key；由内核强制"重复声明必须一致"兜底。放弃严格单所有权（需为若干配置值另造契约接口，且 `driver_http` 需 session store 连接参数是真实底层依赖）。 |
| D4 | **一次性全量迁移** | 不留双轨，架构一次到位；接受较大的 diff。 |
| D5 | **显式测试缝** | 删除 `isTest()` 魔法，测试通过 `core.WithConfigValues(...)` 注入。放弃"测试环境自动禁用中间件"的安全网，换取语义透明与可并行。 |
| D6 | **内核持抽象，viper 归 infra 适配器** | 微内核防线规定 `core/` 严禁 import 具体运行时依赖。`core` 只依赖 `ConfigSource` 接口，viper/yaml 装载放 `plugins/infra/config`。放弃"全放 core/config"（污染内核纯净性）与"完全插件化 + `contracts.ConfigService`"（门禁求值在 core，而 config 插件 `Apply` 尚未运行，存在鸡生蛋时序问题）。 |
| D7 | **顺带解耦 `pkg/idgen`** | 其 `init()` 读全局配置，导致 `pkg` 反向依赖配置单例。 |

---

## 2. 分层与物理结构

```text
backend/core/extpoints/config.go     # 配置引擎（仅 stdlib + reflect）：
                                     #   ConfigSource 接口、声明注册、解析、冲突校验、脱敏 dump
backend/core/config.go               # 泛型读取入口与 App 装配选项（Go 方法不支持类型参数）
backend/core/fiber.go                # 新增 FiberSkipped 状态与门禁求值
backend/core/types.go                # 新增 ConfigExtension / ConfigBinding / ConfigView 别名
backend/plugins/infra/config/        # viper + yaml 适配器，实现 core.ConfigSource（非 core.Plugin）
backend/cmd/                         # 组合根：host 声明集 + app.Prepare()
backend/pkg/idgen/                   # 移除 config 依赖，改为显式 Init(nodeID)
删除 backend/pkg/config/              # 全局单例 config.Config 一并消失
```

职责边界：

| 单元 | 做什么 | 不做什么 |
| :--- | :--- | :--- |
| `core/extpoints` 配置引擎 | 维护 key 注册表、按优先级解析、类型转换、冲突校验、脱敏输出 | 不知道任何具体 key 的名字，不读文件，不 import viper |
| `plugins/infra/config` | 定位 `config.yaml`（`CONFIG_PATH` → 向上查找）、解析成 raw map、代理 env 查询 | 不含 schema、不含业务字段语义 |
| 各插件 | 声明自己读哪些字段（tag 结构体）、声明门禁谓词 | 不读未声明的 key、不访问他插件的声明类型 |
| `cmd` | 声明 host 级 key、注入 `ConfigSource`、按已解析值初始化 logger/trace/banner | 不做 `if redis.enabled { ... }` 这类跨插件判断 |

`plugins/infra/config` 不实现 `core.Plugin`，不出现在 `app.Use()` 列表里：它只向内核提供一个 `ConfigSource` 实例，没有服务、路由或任务可注册。它归 `plugins/infra/` 而非 `pkg/`，是因为它封装了具体运行时依赖（viper、文件系统）并持有装载状态，不符合 `pkg/` 的无状态定位。

`pkg/idgen` 解耦后，`backend/pkg/` 恢复"不依赖配置源"的无状态定位。

---

## 3. 核心类型与 API

### 3.1 声明形态：带 tag 的结构体

唯一的批量作者形态是结构体 tag，一个字段同时表达 yaml 路径、env 覆盖名、默认值与敏感标记：

```go
// plugins/infra/cache/redis_config.go —— redis 配置由 redis 插件自己声明
type redisConfig struct {
    Enabled            bool     `config:"enabled"             env:"REDIS_ENABLED"            default:"false"      autoEnable:"REDIS_ADDR"`
    Addrs              []string `config:"addrs"               env:"REDIS_ADDR"`
    Username           string   `config:"username"            env:"REDIS_USERNAME"`
    Password           string   `config:"password"            env:"REDIS_PASSWORD"           secret:"true"`
    DB                 int      `config:"db"                  env:"REDIS_DB"`
    ClusterMode        bool     `config:"cluster_mode"        env:"REDIS_CLUSTER_MODE"`
    MasterName         string   `config:"master_name"         env:"REDIS_MASTER_NAME"`
    KeyPrefix          string   `config:"key_prefix"          env:"REDIS_KEY_PREFIX"`
    MaintNotifications bool     `config:"maint_notifications" env:"REDIS_MAINT_NOTIFICATIONS" default:"false"`
    // ...pool/timeout 字段略
}
```

支持的 tag：`config`（yaml 相对路径，必填）、`env`（覆盖用环境变量名）、`default`（字符串形式，缺省时视为未设置）、`autoEnable`（该 env 一旦存在即把本布尔字段置 true）、`secret`（dump 时脱敏）。

### 3.2 内核接口

```go
// ConfigSource 抽象了"原始值从哪来"，由 infra 适配器实现，使内核不绑定 viper。
type ConfigSource interface {
    Lookup(path string) (any, bool)   // config.yaml 中的点分路径
    LookupEnv(name string) (string, bool)
    Describe() string                 // 用于日志，如 "config.yaml" 或 "<env only>"
}

// ConfigBinding 把一个结构体绑定到某个 yaml 前缀上，是插件的声明单元。
type ConfigBinding struct {
    Prefix string // "redis"；空串表示字段 key 即完整路径
    Target any    // 指向带 tag 的结构体的指针
}

// ConfigView 是只读的已解析视图，供门禁与零散取值使用。
type ConfigView interface {
    String(key, fallback string) string
    Bool(key string, fallback bool) bool
    Int(key string, fallback int) int
    Duration(key string, fallback time.Duration) time.Duration
    Strings(key string) []string
    WasSet(envName string) bool
    Source(key string) string // "env" | "yaml" | "default"，用于诊断
}

// ConfigExtension 是挂载在 Context 上的扩展点，根 Context 与所有 Fork 共享。
type ConfigExtension interface {
    ConfigView
    Declare(pluginID string, bindings ...ConfigBinding) error
    Bind(prefix string, target any) error
    Entries() []ConfigEntry   // 有效配置的脱敏视图
}
```

`core` 侧导出别名与泛型入口（沿用仓库既有 `core.Provide[T]` / `core.Inject[T]` 风格）：

```go
func ConfigGet[T any](v extpoints.ConfigView, key string) (T, error)
```

### 3.3 插件侧用法

```go
// 批量绑定（Apply 内）
var cfg redisConfig
if err := ctx.Config().Bind("redis", &cfg); err != nil {
    return err
}

// 单字段读取：带 fallback 的访问器（门禁使用）
secret := ctx.Config().String("app.session_secret", "")

// 单字段读取：需要区分"未设置"与"设置为零值"时用泛型入口
rate, err := core.ConfigGet[float64](ctx.Config(), "otel.sampling_rate")
```

`ConfigEntry` 是 `Entries()` 返回的诊断单元，只含元数据与脱敏后的值：

```go
type ConfigEntry struct {
    Key      string // "redis.password"
    PluginID string // 首次声明者，用于冲突报错点名
    Env      string
    Source   string // "env" | "yaml" | "default"
    Value    string // secret key 输出 "******"
}
```

`Bind` 的双重语义：若该 prefix 尚未声明，则按 `Target` 的 tag 自登记；若已声明，则是纯读取。登记时提供的 `env`/`default`/`secret` 元数据一律参与冲突校验（依 D3），因此自登记不会绕过校验。**只有需要早于 `Apply` 求值的插件才必须显式 `DeclareConfig()`。**

### 3.4 门禁接口与 Fiber 跳过态

```go
// ConfigGatedPlugin 是可选接口：让内核在 Apply 之前决定插件是否激活。
type ConfigGatedPlugin interface {
    Plugin
    DeclareConfig() []extpoints.ConfigBinding     // 门禁所需 key 必须提前声明
    ConfigEnabled(v extpoints.ConfigView) bool
}
```

`FiberState` 新增 `FiberSkipped`。`App.reconcileLocked()` 在 `Load()` 前求值门禁：门禁为 false 的 Fiber 置 `FiberSkipped`，不计入依赖 satisfied 判定，也不参与 driver 启动。`App.Stop()` 对 skipped 与 active 一视同仁地按 LIFO 卸载其 scoped Context。

受门禁的插件对（现状仅三对，均为 Redis 存在与否的互斥实现）：

| 启用 | 跳过 | 门禁谓词 |
| :--- | :--- | :--- |
| `infra/cache` | `infra/cache_memory` | `redis.enabled` |
| `drivers/driver_asynq_worker` | `drivers/driver_inproc_worker` | `redis.enabled` |
| `drivers/driver_asynq_cron` | `drivers/driver_inproc_cron` | `redis.enabled` |

### 3.5 组合根

```go
src := config.NewSource()                      // plugins/infra/config：仅定位与 raw 解析
app := core.NewApp(
    core.WithProfile(profile),
    core.WithConfigSource(src),
    core.WithConfigDecl(hostBinding...),       // app.* / log.* / otel.*
)
app.Use(
    infradb.New(), logger.New(), storage.New(),
    cache.New(), cache_memory.New(),            // 不再 if/else，门禁决定
    driver_asynq_worker.New(), driver_inproc_worker.New(),
    driver_asynq_cron.New(), driver_inproc_cron.New(),
    admin.New(), user.New(), auth.New(), /* ... */
    driver_http.New(),                          // addr 由插件自己声明读取
)
if err := app.Prepare(); err != nil { return err }   // 解析屏障 + 门禁求值

timeout, _ := app.Context().Config().Duration("app.graceful_shutdown_timeout", 30)
app.SetShutdownTimeout(timeout)
```

`WithShutdownTimeout(d)` 保留为显式覆盖入口（测试与非标准装配使用），生产路径改为 `Prepare()` 之后由已解析视图经 `SetShutdownTimeout` 设定。`driver_http.New(WithAddr(...))` 选项删除，addr 归 `driver_http` 在 `Apply` 内声明读取。

---

## 4. 解析语义与启动时序

### 4.1 单 key 优先级链

```text
1. 显式 env 命中          env:"DB_ENABLED"        → 最高优先级
2. autoEnable env 命中    autoEnable:"DB_HOST"     → true（被 1 覆盖）
3. config.yaml 命中       config:"enabled"
4. default tag 兜底
```

需要保留的既有特殊语义：

- **标量 env 填充切片字段**：`REDIS_ADDR=redis:6379` → `redis.addrs = ["redis:6379"]`；`CLICKHOUSE_HOST` 同理。
- **隐式启用**：`DB_HOST` → `database.enabled=true`、`REDIS_ADDR` → `redis.enabled=true`、`CLICKHOUSE_HOST` → `clickhouse.enabled=true`；显式 `*_ENABLED` 始终优先于隐式推导。
- **同一 env 的双重角色**：`REDIS_ADDR` 既是 `redis.addrs` 的值来源，又是 `redis.enabled` 的 `autoEnable` 触发器。引擎按 key 独立解析、允许一个 env 名服务多个 key，实现时不可把它建模成"env → 单一 key"的一对一映射。
- **时长字段**：`slow_threshold: 200ms` 解析为 `time.Duration`。
- **文件定位**：`CONFIG_PATH` 优先；否则从工作目录向上最多 5 层查找 `config.yaml`（该文件位于仓库根，`backend/` 为其子目录）。

### 4.2 时序

```text
config.NewSource()            # 读 yaml → raw map；零 schema 知识
  ↓
core.NewApp(WithConfigSource) # 记录 host 声明
  ↓
app.Use(...)                  # 遇 DeclareConfig() 立即登记 binding（叶子 key + env + default + secret）
  ↓
app.Prepare()                 # ① 冲突校验 ② 逐 key 解析 ③ 脱敏 dump ④ 门禁求值 → FiberSkipped
  ↓
app.Run() → Reconcile/Apply   # 插件内 Bind/Get 读取已解析值
```

`App.Start()` 在未显式调用 `Prepare()` 时幂等补做，防止遗漏。冲突校验规则：同一 key 的多份声明必须 `env` 名、`default`、`secret` 三项一致，否则 `Prepare()` 返回错误并点名两个声明者。

### 4.3 有意的行为变更

| # | 变更 | 现状 | 变更后 |
| :--- | :--- | :--- | :--- |
| C1 | `default` 生效条件 | `applyDefaults` 对零值二次回落（`session_age<=0` → 86400） | 仅当 env 与 yaml 均缺失时生效；`app.session_age<=0` 在 `Prepare()` 判为配置错误（fail fast 优于静默改写） |
| C2 | 测试上下文 | `isTest()` 自动禁用 DB/Redis/ClickHouse 并把 sqlite 指向 `:memory:` | 删除该魔法；测试用 `core.WithConfigValues(...)` 显式声明。未声明 `database.enabled` 时按 default `false` 落 sqlite 后备，其路径沿用 `postgres.go` 既有的 `./data/wavelet.db` 回落——需要内存库的用例必须显式注入 `database.sqlite_path = ":memory:"` |
| C3 | 配置 dump | `printConfig` 明文打印全量结构体，含 `DB_PASSWORD`、`APP_SESSION_SECRET` | 按 `secret:"true"` 脱敏后输出，并标注每个 key 的来源（env/yaml/default） |
| C4 | 队列默认值 | 硬编码在 `pkg/config` 的 `applyEnvOverrides` | 移入唯一消费者 `driver_asynq_worker` 的声明（`webhook`/`whitelist_only`/`default` 三级优先级不变） |
| C5 | 非法 env 值 | `envInt/envBool/envFloat64` 在 `strconv` 失败时静默丢弃 env 值、回落 yaml/default | `Prepare()` 返回 `ErrConfigType` 并点名 key 与非法值 |

除此之外，解析结果与现状逐 key 等价（由 §7.1 第 4 条的对拍测试证明）。

---

## 5. 声明归属映射

| 声明方 | key 前缀 | 消费者（含跨插件读） |
| :--- | :--- | :--- |
| `cmd` host 声明集 | `app.{env,app_name,addr,node_id,graceful_shutdown_timeout}`、`log.*`、`otel.*` | `cmd/root.go`、`cmd/banner.go`、`core.App` |
| `plugins/infra/cache` | `redis.*`（含 `enabled` 门禁、`autoEnable: REDIS_ADDR`） | `infra/cache`、`driver_http`(session store)、`driver_asynq_worker`、`driver_asynq_cron` |
| `plugins/infra/database` | `database.*`、`clickhouse.*` | `infra/database`、`admin`、`risk_control` |
| `plugins/domain/auth` | `app.session_*`（cookie/secret/age/domain/secure/http_only） | `auth`、`cap`、`message_gateway`、`driver_http` |
| `plugins/drivers/driver_asynq_worker` | `worker.*`（并发、strict_priority、queues 默认值） | 自身 |
| 其余 | 按需就近声明 | — |

跨插件读同一 key（如 `cap` 读 auth 声明的 `app.session_secret`）依 D3 走共享声明：`cap` 也声明该 key，三份元数据必须与 auth 一致，否则启动失败。

**Key 命名约定**：既有 infra key 保持顶层（`redis.*`、`database.*`），以兼容线上 `config.yaml`；新增插件的私有配置归 `plugins.<name>.*` 命名空间，与 `new-setting` skill 的描述对齐。

### 5.1 `pkg/idgen` 解耦

- 删除 `init()` 中对 `config.Config.App.NodeID` 的读取。
- 新增 `idgen.Init(nodeID int64) error`，由 host 在 `Prepare()` 之后显式调用（值来自 host 声明的 `app.node_id`）。
- 未初始化时 `NextUint64ID()` panic 并点名"未调用 idgen.Init"，而非静默使用 nodeID=0 生成可能与集群冲突的 ID。
- 11 个调用点的 `idgen.NextUint64ID()` 签名保持不变；依赖 ID 生成的测试需显式 `idgen.Init`。这是本次迁移唯一会触及既有测试文件之处。

### 5.2 明确不在范围内

本设计只改变配置的**来源与所有权**，不动这些既有全局变量：`cache.Redis`、`driver_asynq_worker.RedisOpt`/`AsynqClient`、`infra/database.db`。它们各自的收敛属于独立议题。

---

## 6. 错误处理

- `Prepare()` 以 `errors.Join` 聚合全部配置错误，哨兵错误：`ErrConfigConflict`（重复声明不一致）、`ErrConfigType`（env 值无法转为目标类型）、`ErrConfigInvalid`（值域校验失败，如 `session_age<=0`）、`ErrConfigNotResolved`（`Prepare()` 之前调用 `Bind`/`Get`，错误信息点名正确调用顺序）。
- 所有配置错误经 `error` 返回，由 `cmd` 决定终止方式；`core` 与 `extpoints` 内不再有 `log.Fatalf`。
- `config.yaml` 缺失不是错误（沿用"仅用 env"路径，记一条 info 日志）；`CONFIG_PATH` 显式指定但读不到或解析失败 → 返回 error。
- 门禁 `ConfigEnabled(v ConfigView) bool` 只读已解析值、用带 fallback 的访问器，配置错误已在 `Prepare()` 阶段暴露，因此门禁不引入新的错误源。
- `Declare` 与 `Bind` 校验 `Target` 必须是非 nil 结构体指针，否则返回 error（不 panic）。

---

## 7. 测试与验收

### 7.1 测试分层

1. **引擎单测**（`core/extpoints`）：内存 fake `ConfigSource`，表驱动覆盖优先级四档、标量 env→切片、`autoEnable` 与显式 env 的优先关系、冲突校验、脱敏 dump、`time.Duration` 与嵌套结构体 tag 解析、`Prepare()` 前访问的错误路径。
2. **门禁单测**（`core`）：互斥插件对恰好激活一个、被跳过插件不计入依赖 satisfied、`FiberSkipped` 参与 `Stop` 的 LIFO 卸载。
3. **适配器单测**（`plugins/infra/config`）：`t.TempDir()` 写 yaml + `t.Setenv`，禁止相对路径。
4. **新旧对拍**：迁移期间保留一份临时对拍测试，用仓库现网 `config.yaml` 与 `.env` 逐 key 比较旧 `pkg/config` 与新引擎的输出，证明除 C1–C5 外完全等价；验证通过后随旧包一并删除。
5. **迁移后插件测试**：改用 `core.WithConfigValues(...)` 显式注入；依赖 ID 生成的测试显式 `idgen.Init`。

### 7.2 验收标准

1. `backend/pkg/config` 不存在，`grep -rn "pkg/config\|config\.Config" backend/` 零命中。
2. `core/` 无 viper import；`backend/pkg/` 内不出现任何配置源 import。
3. `cmd/app.go` 中不存在跨插件配置判断，驱动选型完全由门禁产生。
4. `.env`、`config.yaml`、docker-compose **零改动**即可启动，行为等价（除已登记的 C1–C5）。
5. 同时挂载 `cache` 与 `cache_memory` 而仅激活其一——"预声明 + 门禁"的端到端可验证证据；两条路径（Redis 启用 → asynq；禁用 → inproc）各实跑一次。
6. `make code-check`、`make format`、`go test ./backend/...` 全绿；`go run main.go all` 实跑通过，覆盖 banner、迁移与门禁。
7. `AGENTS.md`、`new-setting` skill 与白皮书中 `ctx.Config().Bind(...)` 的签名与 key 命名约定更新为已实现的真实 API。

### 7.3 实施顺序建议

每阶段独立可验证，供实施计划拆分参考：

| 阶段 | 内容 | 验证 |
| :--- | :--- | :--- |
| P1 | 配置引擎（`core/extpoints/config.go`）+ `plugins/infra/config` 适配器 + 新旧对拍测试 | `go test ./backend/core/...`；对拍输出等价性报告 |
| P2 | 门禁与 `FiberSkipped`、`App.Prepare()` 解析屏障 | `core` 门禁单测；现有测试全绿（此时旧单例仍在，未迁移） |
| P3 | 按 infra → drivers → domain → cmd 顺序迁移 27 个文件；`idgen.Init` 解耦 | 每层迁移后 `go build ./...` + 该层测试；最后实跑两条门禁路径 |
| P4 | 删除 `backend/pkg/config` 与对拍测试；更新 `AGENTS.md`/skill/白皮书 API | §7.2 全部验收项逐条复核 |

---

## 附录 A：迁移清单

删除：`backend/pkg/config/{config.go,model.go,config_test.go}`

新增：`backend/core/extpoints/config.go`、`backend/core/config.go`、`backend/plugins/infra/config/*`、各插件内 `<name>_config.go` 声明文件

需改写的 27 个文件：

| 分组 | 文件 |
| :--- | :--- |
| 组合根 | `cmd/app.go`、`cmd/root.go`、`cmd/banner.go`、`cmd/app_test.go`、`cmd/banner_test.go`、`cmd/redis_plug_test.go` |
| 基础库 | `pkg/idgen/snowflake.go`（连带 `pkg/idgen/snowflake_test.go`） |
| infra | `plugins/infra/cache/redis.go`、`plugins/infra/database/postgres.go`、`plugins/infra/database/clickhouse.go` |
| drivers | `plugins/drivers/driver_http/engine.go`、`plugins/drivers/driver_http/middlewares.go`、`plugins/drivers/driver_asynq_worker/utils.go`、`plugins/drivers/driver_asynq_worker/utils_test.go`、`plugins/drivers/driver_asynq_cron/plugin.go` |
| domain/admin | `plugins/domain/admin/handler/db.go`、`plugins/domain/admin/repository/db.go`、`plugins/domain/admin/service/db.go`、`plugins/domain/admin/service/status.go`、`plugins/domain/admin/service/log_switch.go` |
| domain/其他 | `plugins/domain/auth/session.go`、`plugins/domain/cap/service.go`、`plugins/domain/message_gateway/service/service.go`、`plugins/domain/system/plugin.go`、`plugins/domain/risk_control/middleware.go`、`plugins/domain/risk_control/middleware_test.go`、`plugins/domain/risk_control/logstore/provider.go` |

> 注：`risk_control/middleware.go`、`cap/service.go`、`message_gateway/service/service.go` 等处以 `config.Config != nil` 做存在性判断的分支，在注入式配置模型下不再可能，迁移时一并消除。
