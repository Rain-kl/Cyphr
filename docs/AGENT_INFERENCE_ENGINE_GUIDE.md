# 推理引擎编写指导规范

适用范围：`backend/agent/src/models/*` 新引擎 + `src/job_runner.py` / `src/reporter.py` / `src/models/registry.py` 相关改动。违反 MUST 项的 PR 直接打回。

## 1. 引擎契约（MUST）

- 继承 `BaseEngine`（`src/models/base.py`）：实现 `load(work_mode)` / `unload()` / `transcribe(audio_path, language, task_type, log_callback)`，返回 OpenAI `verbose_json` 结构（`text` + `segments[id/seek/start/end/text]`）。
- `transcribe` MUST 是同步纯函数（跑在 `run_in_executor` 线程池）：内部禁止 `await`、禁止直接 HTTP POST；进度经 `log_callback(progress, msg)` 同步回调，由 `job_runner` 节流上报。
- `load` 的重量工作 MUST 走 `asyncio.to_thread`（参考 `Qwen3ASREngine.load`），禁止阻塞 event loop。

## 2. 并发（MUST）

- 禁止新增全局推理锁。跨 job 串行只允许按 `(model, device)` 细粒度锁；`registry.acquire_engine` 的引用计数是唯一的防热卸载机制，推理期间 MUST 持有它。
- `torch.inference_mode()` 包裹前向；同 `(model, device)` 禁止并发进模型（Qwen 内核非线程安全），不同 `(model, device)` 允许并行。

## 3. Batch 与解码（MUST/SHOULD）

- MUST 按**时长预算**拼 batch（默认 `180s`，`QWEN3_ASR_MAX_BATCH_SECONDS` 可覆），再叠个数上限（`QWEN3_ASR_BATCH_SIZE`）。禁止只按个数拼 batch。
- MUST 实现 OOM 减半重试（≤3 次，预算减半仍保证收敛）→ 退化 single-chunk。`is_oom_error` 判 `out of memory / CUDA error / CUBLAS` 类错误，非 OOM 走原 single-chunk fallback。
- `max_new_tokens` SHOULD ≤512（ASR 30s chunk <150 tokens），经 `QWEN3_ASR_MAX_NEW_TOKENS` 覆盖并 clamp 128~1024。
- 静音跳过 SHOULD 默认开（能量 VAD，`QWEN3_ASR_SKIP_SILENCE=1`），全静音 MUST 零推理返回。

## 4. 预处理（SHOULD）

- 16k mono 直读快径：只校验 `sr==16k + 单声道`，不白名单 format；失败再走 ffmpeg。
- ffmpeg 参数用模块单例常量，`pipe:1` + `s16le`，禁止每 job 拼临时 wav 文件；大文件禁止全量 `frombuffer` 常驻，走分块 resample。
- `cudnn.benchmark` 默认关（变长 batch 下 autotune 抖动），`torch.compile` 默认关、env 显式开并 try/except 回落 eager。

## 5. I/O 与上报（MUST）

- `reporter` 内禁止同步文件写：分块（64KB）+ `asyncio.to_thread`。
- 推理临界区禁止逐 batch POST：节流（≥2s 或 ≥10%，100% 必报），仅成功上报后更新水位。
- 心跳 `list_loaded_models()` MUST 保持去重模型名（控制器兼容）；明细用 `list_loaded_models_detailed()`（`model@device`）。

## 6. 多卡（MUST）

- 新引擎禁止直写 `cuda:0`。设备选择 MUST 经 `GpuScheduler.select_device()`（least-loaded：util → used_ratio → index；`QWEN3_ASR_DEVICE` 显式优先；`CUDA_VISIBLE_DEVICES`/`GPU_DEVICES` 过滤；无卡回落 `cpu`）。
- registry key MUST 是 `(model, device)` 二元；`set_work_mode` 支持 `"cuda:0,cuda:1"` 多卡串，禁止顺手 `unload_all`（按需迁移）。
- monitor MUST 上报 `gpu_devices[{index,percent,used_mb,total_mb}]` 明细，聚合字段（`gpu_percent`=峰值，显存求和）保持兼容。

## 7. 配置与测试（MUST）

- 新增调参 MUST 经 env 覆盖 + 默认值收敛（如上表），并在 `config.example.yaml` / README 配置表登记。
- TDD：新行为先加 `tests/test_agent.py` 用例（红→绿），`uv run pytest tests/test_agent.py -q` 全绿方可提交；并行分工时测试按文件拆分，禁止多 Agent 同写一文件。
