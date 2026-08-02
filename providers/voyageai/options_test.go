package voyageai

import "testing"

func TestWithAPIKeyOverridesConstructorArg(t *testing.T) {
	p := New("ctor-key", WithAPIKey("opt-key"))
	if got := p.config.APIKey.Expose(); got != "opt-key" {
		t.Errorf("APIKey = %q, want opt-key (option should win over constructor)", got)
	}
}
