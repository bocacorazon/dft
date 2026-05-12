package agentjson

import "testing"

func TestDecodeFirstAcceptsLeadingProse(t *testing.T) {
	var got struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}

	if err := DecodeFirst("Preparing result...\n{\"name\":\"todo\",\"count\":3}\nDone.\n", &got); err != nil {
		t.Fatalf("DecodeFirst returned error: %v", err)
	}

	if got.Name != "todo" || got.Count != 3 {
		t.Fatalf("decoded = %#v, want todo/3", got)
	}
}

func TestDecodeFirstAcceptsLeadingProseBeforeArray(t *testing.T) {
	var got []struct {
		ID string `json:"id"`
	}

	if err := DecodeFirst("Assignments:\n[{\"id\":\"001\"}]\n", &got); err != nil {
		t.Fatalf("DecodeFirst returned error: %v", err)
	}

	if len(got) != 1 || got[0].ID != "001" {
		t.Fatalf("decoded = %#v, want one assignment", got)
	}
}

func TestDecodeFirstReturnsErrorWhenNoJSONValueExists(t *testing.T) {
	var got map[string]string

	if err := DecodeFirst("no structured output here", &got); err == nil {
		t.Fatal("DecodeFirst returned nil error, want failure")
	}
}
