package toolcalls

import (
	"errors"
	"testing"
)

func TestAssemblerFinalizeEmpty(t *testing.T) {
	a := NewAssembler(Config{})
	calls, err := a.Finalize()
	if err != nil {
		t.Fatalf("Finalize() error = %v", err)
	}
	if calls != nil {
		t.Errorf("Finalize() = %v, want nil", calls)
	}
}

func TestAssemblerFragmentsAndOrder(t *testing.T) {
	a := NewAssembler(Config{})

	a.AddFragment(Fragment{Index: 1, ID: "call_2", Name: "time", Arguments: `{"tz":"UTC"}`})
	a.AddFragment(Fragment{Index: 0, ID: "call_1", Name: "weather"})
	a.AddFragment(Fragment{Index: 0, Arguments: `{"city":`})
	a.AddFragment(Fragment{Index: 0, Arguments: `"NYC"}`})

	calls, err := a.Finalize()
	if err != nil {
		t.Fatalf("Finalize() error = %v", err)
	}
	if len(calls) != 2 {
		t.Fatalf("len(calls) = %d, want 2", len(calls))
	}
	if calls[0].ID != "call_1" || calls[0].Name != "weather" || string(calls[0].Arguments) != `{"city":"NYC"}` {
		t.Errorf("calls[0] = %+v", calls[0])
	}
	if calls[1].ID != "call_2" || calls[1].Name != "time" || string(calls[1].Arguments) != `{"tz":"UTC"}` {
		t.Errorf("calls[1] = %+v", calls[1])
	}
}

func TestAssemblerInvalidJSON(t *testing.T) {
	a := NewAssembler(Config{})
	a.AddFragment(Fragment{Index: 0, ID: "bad", Name: "broken", Arguments: `{invalid`})

	_, err := a.Finalize()
	if !errors.Is(err, ErrInvalidJSON) {
		t.Errorf("err = %v, want ErrInvalidJSON", err)
	}
}

func TestAssemblerStartCallAndEmptyArgs(t *testing.T) {
	a := NewAssembler(Config{EmptyArgumentsJSON: "{}"})
	a.StartCall(0, "call_1", "no_args")
	a.AddArguments(0, "")

	calls, err := a.Finalize()
	if err != nil {
		t.Fatalf("Finalize() error = %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("len(calls) = %d, want 1", len(calls))
	}
	if string(calls[0].Arguments) != "{}" {
		t.Errorf("Arguments = %s, want {}", calls[0].Arguments)
	}
}

func TestAssemblerAddArgumentsWithoutStartIsNoop(t *testing.T) {
	a := NewAssembler(Config{})
	a.AddArguments(0, `{"x":1}`)

	calls, err := a.Finalize()
	if err != nil {
		t.Fatalf("Finalize() error = %v", err)
	}
	if calls != nil {
		t.Errorf("calls = %v, want nil", calls)
	}
}
