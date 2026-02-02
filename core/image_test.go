// core/image_test.go
package core

import "testing"

func TestImageSizeValidation(t *testing.T) {
	tests := []struct {
		size  ImageSize
		valid bool
	}{
		{ImageSize1024x1024, true},
		{ImageSize1536x1024, true},
		{ImageSize1024x1536, true},
		{ImageSizeAuto, true},
		{ImageSize("invalid"), false},
	}

	for _, tt := range tests {
		if got := tt.size.IsValid(); got != tt.valid {
			t.Errorf("ImageSize(%q).IsValid() = %v, want %v", tt.size, got, tt.valid)
		}
	}
}

func TestImageQualityValidation(t *testing.T) {
	tests := []struct {
		quality ImageQuality
		valid   bool
	}{
		{ImageQualityLow, true},
		{ImageQualityMedium, true},
		{ImageQualityHigh, true},
		{ImageQualityAuto, true},
		{ImageQuality("invalid"), false},
	}

	for _, tt := range tests {
		if got := tt.quality.IsValid(); got != tt.valid {
			t.Errorf("ImageQuality(%q).IsValid() = %v, want %v", tt.quality, got, tt.valid)
		}
	}
}

func TestImageFormatValidation(t *testing.T) {
	tests := []struct {
		format ImageFormat
		valid  bool
	}{
		{ImageFormatPNG, true},
		{ImageFormatJPEG, true},
		{ImageFormatWebP, true},
		{ImageFormat("gif"), false},
	}

	for _, tt := range tests {
		if got := tt.format.IsValid(); got != tt.valid {
			t.Errorf("ImageFormat(%q).IsValid() = %v, want %v", tt.format, got, tt.valid)
		}
	}
}
