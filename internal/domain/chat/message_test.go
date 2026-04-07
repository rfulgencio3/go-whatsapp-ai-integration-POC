package chat

import "testing"

func TestNormalizePhoneNumber(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"+55 (34) 98828-3531": "5534988283531",
		"34988283531":         "5534988283531",
		"3488567673":          "553488567673",
		"553488567673":        "553488567673",
	}

	for input, expected := range tests {
		if got := NormalizePhoneNumber(input); got != expected {
			t.Fatalf("NormalizePhoneNumber(%q) = %q, want %q", input, got, expected)
		}
	}
}
