// Package model provides model catalog, inspection, and download functionality.
package model

const (
	// ModelIDQwen06B is the default repo ID for Qwen 0.6B.
	ModelIDQwen06B = "Qwen/Qwen3-ASR-0.6B"
	// ModelIDQwen17B is the default repo ID for Qwen 1.7B.
	ModelIDQwen17B = "Qwen/Qwen3-ASR-1.7B"
)

// PresetModel represents a recommended or supported ASR model preset.
type PresetModel struct {
	ID            string // Default model identifier e.g. "Qwen/Qwen3-ASR-0.6B"
	HuggingFaceID string // Hugging Face repo ID e.g. "Qwen/Qwen3-ASR-0.6B"
	ModelScopeID  string // ModelScope repo ID e.g. "Qwen/Qwen3-ASR-0.6B"
	PkgDir        string // Local directory name e.g. "qwen3-asr-0.6b"
	Name          string // Display name e.g. "Qwen3-ASR-0.6B"
	Description   string // Description
	Tag           string // Badge tag e.g. "推荐"
	SizeEst       string // Estimated size e.g. "~1.8GB"
}

// PresetCatalog lists all standard preconfigured models with their respective repository IDs across platforms.
var PresetCatalog = []PresetModel{
	{
		ID:            ModelIDQwen06B,
		HuggingFaceID: ModelIDQwen06B,
		ModelScopeID:  ModelIDQwen06B,
		PkgDir:        "qwen3-asr-0.6b",
		Name:          "Qwen3-ASR-0.6B",
		Description:   "通义千问 ASR 0.6B，体积与精度兼备，适合通用中文及多语种",
		Tag:           "推荐/默认",
		SizeEst:       "~1.8GB",
	},
	{
		ID:            ModelIDQwen17B,
		HuggingFaceID: ModelIDQwen17B,
		ModelScopeID:  ModelIDQwen17B,
		PkgDir:        "qwen3-asr-1.7b",
		Name:          "Qwen3-ASR-1.7B",
		Description:   "通义千问 ASR 1.7B 高精度大模型，适合复杂声学与专业术语",
		Tag:           "高精度",
		SizeEst:       "~4.5GB",
	},
	{
		ID:            "openai/whisper-large-v3",
		HuggingFaceID: "openai/whisper-large-v3",
		ModelScopeID:  "AI-ModelScope/whisper-large-v3",
		PkgDir:        "whisper-large-v3",
		Name:          "Whisper-large-v3",
		Description:   "OpenAI Whisper 顶级精度模型，支持跨语种翻译与超长音频",
		Tag:           "高精度",
		SizeEst:       "~3.1GB",
	},
	{
		ID:            "openai/whisper-base",
		HuggingFaceID: "openai/whisper-base",
		ModelScopeID:  "",
		PkgDir:        "whisper-base",
		Name:          "Whisper-base",
		Description:   "OpenAI 基础多语种轻量模型，资源消耗低 (ModelScope 暂无官方源)",
		Tag:           "轻量多语",
		SizeEst:       "~145MB",
	},
	{
		ID:            "openai/whisper-small",
		HuggingFaceID: "openai/whisper-small",
		ModelScopeID:  "",
		PkgDir:        "whisper-small",
		Name:          "Whisper-small",
		Description:   "OpenAI 中等精度通用模型，英文与常见多语表现优秀 (ModelScope 暂无官方源)",
		Tag:           "中等模型",
		SizeEst:       "~480MB",
	},
}
