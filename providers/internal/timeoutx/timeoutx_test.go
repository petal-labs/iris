package timeoutx

import (
	"context"
	"testing"
	"time"
)

func TestApply(t *testing.T) {
	// d <= 0 -> no-op, original ctx
	ctx, cancel := Apply(context.Background(), 0)
	cancel()
	if _, ok := ctx.Deadline(); ok {
		t.Error("d=0 should not add a deadline")
	}
	// caller deadline wins
	base, c := context.WithTimeout(context.Background(), time.Hour)
	defer c()
	ctx2, cancel2 := Apply(base, time.Second)
	cancel2()
	dl, _ := ctx2.Deadline()
	if time.Until(dl) < 30*time.Minute {
		t.Error("caller deadline should win")
	}
	// applies otherwise
	ctx3, cancel3 := Apply(context.Background(), time.Second)
	defer cancel3()
	if _, ok := ctx3.Deadline(); !ok {
		t.Error("should apply a deadline")
	}
}
