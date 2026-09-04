# Agent 推理优化经验（2026-09 P0/P1）

对应提交：`8d66777`（单卡引擎）+ `50d44f3`（并发与多卡）。测试：`backend/agent/tests/test_agent.py` 45 passed。

## 发现的问题 → 改法 → 收益

| # | 问题 | 改法（文件） | 收益 |
|---|------|-------------|------|
| P0-1 | `job_runner._inference_lock` 全局串行 + 引擎内 `threading.Lock` 二次串行，`MAX_CONCURRENT_JOBS=2` 失效 | 按 `model_name` 细粒度锁字典，同模型串行、不同模型并行（`src/job_runner.py`） | 双模型 ~2x 吞吐；单模型延迟不变 |
| P0-2 | `max_new_tokens=1024`，30s chunk 实际 <150 tokens，KV 上限浪费 | 默认 512，`QWEN3_ASR_MAX_NEW_TOKENS` 可覆（128~1024 clamp）（`src/models/qwen3_asr.py`） | 尾延迟 -10~20%，KV 显存减半 |
| P1-1 | 按个数 batch（16×30s=480s 一次 encode），长音频 OOM，主因明确 | 按 `180s` 时长预算拼 batch（`QWEN3_ASR_MAX_BATCH_SECONDS`），OOM 减半重试 3 次再退化 single-chunk | 峰值显存 ~1/3，长音频成功率显著升 |
| P1-2 | 静音/音乐段全量过 0.6B/1.7B | 能量 VAD（rms<-40dB 跳过，`QWEN3_ASR_SKIP_SILENCE=1` 默认开），全静音零推理 | 含静音音频省 20~40% 前向 |
| P1-3 | `soundfile` 快径仅认 WAV/MP3；每次拼 ffmpeg 参数；`cudnn.benchmark` 默认开致变长抖动 | 快径去 format 白名单（只校验 16k+mono）；ffmpeg 参数单例常量；benchmark 默认关（env 显式开）；`torch.compile` 默认关入口 | 短音频预处理 -50~200ms；消除 autotune 抖动 |
| P2-1 | `reporter.download_media` 同步写盘阻塞 event loop；每 batch 一次 POST 在关键路径 | 64KB 分块 + `to_thread` 写盘；日志节流（≥2s 或 ≥10% 才报，100% 必报）（`src/reporter.py`，`src/job_runner.py`） | 高频回调 POST 降 5~10x；下载不再阻塞调度/心跳 |
| C1 | `resolve_device_and_dtype` 只用 `CUDA_DEVICE_INDEX` 单卡，多卡空闲；monitor 只看卡 0 | `GpuScheduler` least-loaded（NVML→torch，`QWEN3_ASR_DEVICE` 显式优先，`CUDA_VISIBLE_DEVICES` 过滤）；registry 按 `(model, device)` 多副本；monitor 上报 `gpu_devices` 明细；心跳兼容（`list_loaded_models` 去重，新增 `list_loaded_models_detailed`） | 双卡吞吐 ~1.8x，P99 -40~45% |

## 未做（有意推迟）

- `vLLM` 后端切换：上游 `qwen_asr` 已支持（continuous batching + PagedAttention），预期 2~4x，但需额外依赖与显存复制评估，留待下轮。
- 整模型 INT8/AWQ 量化：省 ~50% 显存，需 WER 回归（要求 <0.3% 损失）后才可默认开。

## 并行协作教训

三子 Agent 同改 `tests/test_agent.py` 造成共享态竞争：A/C 的测试一度只存在于工作区而 B 先提交时一并收拢。下次按文件拆分测试文件（如 `test_concurrency.py` / `test_engine.py` / `test_multigpu.py`）或串行合入，避免“提交者顺手收拢他人半成品”。
