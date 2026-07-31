package core

import (
	"context"
	"testing"
	"time"
)

func TestNewClientDefaultTimeout(t *testing.T) {
	c := NewClient(&mockProvider{})
	if c.timeout != DefaultTimeout {
		t.Errorf("default timeout = %v, want %v", c.timeout, DefaultTimeout)
	}
}

func TestWithTimeoutOverridesDefault(t *testing.T) {
	c := NewClient(&mockProvider{}, WithTimeout(5*time.Second))
	if c.timeout != 5*time.Second {
		t.Errorf("timeout = %v, want 5s", c.timeout)
	}
}

func TestWithTimeoutZeroDisables(t *testing.T) {
	c := NewClient(&mockProvider{}, WithTimeout(0))
	if c.timeout != 0 {
		t.Errorf("timeout = %v, want 0 (disabled)", c.timeout)
	}
}

func TestEffectiveTimeoutPrecedence(t *testing.T) {
	c := NewClient(&mockProvider{}, WithTimeout(30*time.Second))

	// Caller ctx has a deadline -> 0 (caller wins).
	ctxWithDeadline, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	b1 := c.Chat("m")
	if got := b1.effectiveTimeout(ctxWithDeadline); got != 0 {
		t.Errorf("with caller deadline = %v, want 0", got)
	}

	// Builder timeout set -> builder wins over client default.
	b2 := c.Chat("m")
	b2.timeout = 10 * time.Second
	if got := b2.effectiveTimeout(context.Background()); got != 10*time.Second {
		t.Errorf("builder override = %v, want 10s", got)
	}

	// Neither -> client default.
	b3 := c.Chat("m")
	if got := b3.effectiveTimeout(context.Background()); got != 30*time.Second {
		t.Errorf("client default = %v, want 30s", got)
	}
}
