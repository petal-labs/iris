package iris

import (
	"os"
	"testing"
)

func TestOpenAI(t *testing.T) {
	// Save and restore original env
	original := os.Getenv("OPENAI_API_KEY")
	defer os.Setenv("OPENAI_API_KEY", original)

	// Test with key set
	os.Setenv("OPENAI_API_KEY", "test-key")
	client, err := OpenAI()
	if err != nil {
		t.Fatalf("OpenAI() error: %v", err)
	}
	if client == nil {
		t.Error("OpenAI() returned nil client")
	}

	// Test without key
	os.Unsetenv("OPENAI_API_KEY")
	_, err = OpenAI()
	if err == nil {
		t.Error("OpenAI() should return error when key not set")
	}
}

func TestAnthropic(t *testing.T) {
	original := os.Getenv("ANTHROPIC_API_KEY")
	defer os.Setenv("ANTHROPIC_API_KEY", original)

	os.Setenv("ANTHROPIC_API_KEY", "test-key")
	client, err := Anthropic()
	if err != nil {
		t.Fatalf("Anthropic() error: %v", err)
	}
	if client == nil {
		t.Error("Anthropic() returned nil client")
	}

	os.Unsetenv("ANTHROPIC_API_KEY")
	_, err = Anthropic()
	if err == nil {
		t.Error("Anthropic() should return error when key not set")
	}
}

func TestGemini(t *testing.T) {
	originalGemini := os.Getenv("GEMINI_API_KEY")
	originalGoogle := os.Getenv("GOOGLE_API_KEY")
	defer func() {
		os.Setenv("GEMINI_API_KEY", originalGemini)
		os.Setenv("GOOGLE_API_KEY", originalGoogle)
	}()

	// Test with GEMINI_API_KEY
	os.Unsetenv("GOOGLE_API_KEY")
	os.Setenv("GEMINI_API_KEY", "test-key")
	client, err := Gemini()
	if err != nil {
		t.Fatalf("Gemini() with GEMINI_API_KEY error: %v", err)
	}
	if client == nil {
		t.Error("Gemini() returned nil client")
	}

	// Test with GOOGLE_API_KEY
	os.Unsetenv("GEMINI_API_KEY")
	os.Setenv("GOOGLE_API_KEY", "test-key")
	client, err = Gemini()
	if err != nil {
		t.Fatalf("Gemini() with GOOGLE_API_KEY error: %v", err)
	}
	if client == nil {
		t.Error("Gemini() returned nil client")
	}

	// Test without any key
	os.Unsetenv("GEMINI_API_KEY")
	os.Unsetenv("GOOGLE_API_KEY")
	_, err = Gemini()
	if err == nil {
		t.Error("Gemini() should return error when no key set")
	}
}

func TestXAI(t *testing.T) {
	original := os.Getenv("XAI_API_KEY")
	defer os.Setenv("XAI_API_KEY", original)

	os.Setenv("XAI_API_KEY", "test-key")
	client, err := XAI()
	if err != nil {
		t.Fatalf("XAI() error: %v", err)
	}
	if client == nil {
		t.Error("XAI() returned nil client")
	}

	os.Unsetenv("XAI_API_KEY")
	_, err = XAI()
	if err == nil {
		t.Error("XAI() should return error when key not set")
	}
}

func TestOllama(t *testing.T) {
	// Ollama doesn't require API key
	client, err := Ollama()
	if err != nil {
		t.Fatalf("Ollama() error: %v", err)
	}
	if client == nil {
		t.Error("Ollama() returned nil client")
	}
}

func TestFromEnv(t *testing.T) {
	// Save all original values
	originals := map[string]string{
		"OPENAI_API_KEY":    os.Getenv("OPENAI_API_KEY"),
		"ANTHROPIC_API_KEY": os.Getenv("ANTHROPIC_API_KEY"),
		"GEMINI_API_KEY":    os.Getenv("GEMINI_API_KEY"),
		"GOOGLE_API_KEY":    os.Getenv("GOOGLE_API_KEY"),
		"XAI_API_KEY":       os.Getenv("XAI_API_KEY"),
	}
	defer func() {
		for k, v := range originals {
			os.Setenv(k, v)
		}
	}()

	// Clear all keys
	clearAllKeys := func() {
		for k := range originals {
			os.Unsetenv(k)
		}
	}

	// Test priority: OpenAI first
	clearAllKeys()
	os.Setenv("OPENAI_API_KEY", "test-openai")
	os.Setenv("ANTHROPIC_API_KEY", "test-anthropic")
	client, err := FromEnv()
	if err != nil {
		t.Fatalf("FromEnv() error: %v", err)
	}
	if client == nil {
		t.Error("FromEnv() returned nil client")
	}

	// Test Anthropic when OpenAI not set
	clearAllKeys()
	os.Setenv("ANTHROPIC_API_KEY", "test-anthropic")
	client, err = FromEnv()
	if err != nil {
		t.Fatalf("FromEnv() with Anthropic error: %v", err)
	}
	if client == nil {
		t.Error("FromEnv() returned nil client")
	}

	// Test Gemini when others not set
	clearAllKeys()
	os.Setenv("GEMINI_API_KEY", "test-gemini")
	client, err = FromEnv()
	if err != nil {
		t.Fatalf("FromEnv() with Gemini error: %v", err)
	}
	if client == nil {
		t.Error("FromEnv() returned nil client")
	}

	// Test XAI when others not set
	clearAllKeys()
	os.Setenv("XAI_API_KEY", "test-xai")
	client, err = FromEnv()
	if err != nil {
		t.Fatalf("FromEnv() with XAI error: %v", err)
	}
	if client == nil {
		t.Error("FromEnv() returned nil client")
	}

	// Test error when no keys set
	clearAllKeys()
	_, err = FromEnv()
	if err != ErrNoAPIKey {
		t.Errorf("FromEnv() error = %v, want ErrNoAPIKey", err)
	}
}

func TestMustOpenAI(t *testing.T) {
	original := os.Getenv("OPENAI_API_KEY")
	defer os.Setenv("OPENAI_API_KEY", original)

	os.Setenv("OPENAI_API_KEY", "test-key")
	client := MustOpenAI()
	if client == nil {
		t.Error("MustOpenAI() returned nil client")
	}
}

func TestMustOpenAIPanics(t *testing.T) {
	original := os.Getenv("OPENAI_API_KEY")
	defer os.Setenv("OPENAI_API_KEY", original)

	os.Unsetenv("OPENAI_API_KEY")

	defer func() {
		if r := recover(); r == nil {
			t.Error("MustOpenAI() should panic when key not set")
		}
	}()

	MustOpenAI()
}

func TestMustFromEnvPanics(t *testing.T) {
	// Save all original values
	originals := map[string]string{
		"OPENAI_API_KEY":    os.Getenv("OPENAI_API_KEY"),
		"ANTHROPIC_API_KEY": os.Getenv("ANTHROPIC_API_KEY"),
		"GEMINI_API_KEY":    os.Getenv("GEMINI_API_KEY"),
		"GOOGLE_API_KEY":    os.Getenv("GOOGLE_API_KEY"),
		"XAI_API_KEY":       os.Getenv("XAI_API_KEY"),
	}
	defer func() {
		for k, v := range originals {
			os.Setenv(k, v)
		}
	}()

	// Clear all keys
	for k := range originals {
		os.Unsetenv(k)
	}

	defer func() {
		if r := recover(); r == nil {
			t.Error("MustFromEnv() should panic when no key set")
		}
	}()

	MustFromEnv()
}
