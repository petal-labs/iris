package gemini

import (
	"errors"
	"testing"
)

func TestFileErrors(t *testing.T) {
	// Test that file error sentinels exist and are distinct
	if errors.Is(ErrFileProcessing, ErrFileFailed) {
		t.Error("ErrFileProcessing should not equal ErrFileFailed")
	}

	if ErrFileProcessing == nil {
		t.Error("ErrFileProcessing should not be nil")
	}

	if ErrFileFailed == nil {
		t.Error("ErrFileFailed should not be nil")
	}
}
