package model

import (
	"testing"
)

func TestCatalogMapping(t *testing.T) {
	if len(PresetCatalog) == 0 {
		t.Fatal("PresetCatalog is empty")
	}

	for _, p := range PresetCatalog {
		if p.ID == "" {
			t.Errorf("model missing ID: %+v", p)
		}
		if p.HuggingFaceID == "" {
			t.Errorf("model %s missing HuggingFaceID", p.Name)
		}
		if p.PkgDir == "" {
			t.Errorf("model %s missing PkgDir", p.Name)
		}
	}
}

func TestFetchRepoFilesHuggingFaceInvalid(t *testing.T) {
	// Should fail cleanly on nonexistent model
	_, err := FetchRepoFiles(t.Context(), "huggingface", "invalid-org/non-existent-model-xyz-12345", "https://hf-mirror.com")
	if err == nil {
		t.Errorf("expected error for nonexistent model, got nil")
	}
}

func TestFetchRepoFilesModelScopeInvalid(t *testing.T) {
	// Should fail cleanly on nonexistent model
	_, err := FetchRepoFiles(t.Context(), "modelscope", "invalid-org/non-existent-model-xyz-12345", "")
	if err == nil {
		t.Errorf("expected error for nonexistent model, got nil")
	}
}
