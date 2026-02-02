package zai

import (
	"testing"

	"github.com/petal-labs/iris/core"
)

func TestGetModelInfo(t *testing.T) {
	tests := []struct {
		id          core.ModelID
		wantDisplay string
		wantNil     bool
	}{
		{ModelGLM47, "GLM-4.7", false},
		{ModelGLM47Flash, "GLM-4.7 Flash", false},
		{ModelGLM46, "GLM-4.6", false},
		{ModelGLM46V, "GLM-4.6V", false},
		{ModelGLM45, "GLM-4.5", false},
		{ModelGLM4_32B, "GLM-4 32B", false},
		{"nonexistent", "", true},
	}

	for _, tt := range tests {
		t.Run(string(tt.id), func(t *testing.T) {
			info := GetModelInfo(tt.id)
			if tt.wantNil {
				if info != nil {
					t.Errorf("GetModelInfo(%q) = %v, want nil", tt.id, info)
				}
				return
			}

			if info == nil {
				t.Fatalf("GetModelInfo(%q) = nil, want non-nil", tt.id)
			}

			if info.DisplayName != tt.wantDisplay {
				t.Errorf("DisplayName = %q, want %q", info.DisplayName, tt.wantDisplay)
			}
		})
	}
}

func TestModelCapabilities(t *testing.T) {
	// GLM-4.7 should have reasoning
	info := GetModelInfo(ModelGLM47)
	if info == nil {
		t.Fatal("GLM-4.7 info should not be nil")
	}

	hasReasoning := false
	for _, cap := range info.Capabilities {
		if cap == core.FeatureReasoning {
			hasReasoning = true
			break
		}
	}
	if !hasReasoning {
		t.Error("GLM-4.7 should have reasoning capability")
	}

	// GLM-4.7-flash should NOT have reasoning
	flashInfo := GetModelInfo(ModelGLM47Flash)
	if flashInfo == nil {
		t.Fatal("GLM-4.7-flash info should not be nil")
	}

	hasReasoning = false
	for _, cap := range flashInfo.Capabilities {
		if cap == core.FeatureReasoning {
			hasReasoning = true
			break
		}
	}
	if hasReasoning {
		t.Error("GLM-4.7-flash should NOT have reasoning capability")
	}
}

func TestModelsCount(t *testing.T) {
	if len(models) != 14 {
		t.Errorf("len(models) = %d, want 14", len(models))
	}
}

func TestModelsMethod(t *testing.T) {
	p := New("test-key")
	modelList := p.Models()

	if len(modelList) != len(models) {
		t.Errorf("len(Models()) = %d, want %d", len(modelList), len(models))
	}

	// Verify it's a copy, not the original slice
	modelList[0].DisplayName = "modified"
	if models[0].DisplayName == "modified" {
		t.Error("Models() should return a copy, not the original slice")
	}
}
