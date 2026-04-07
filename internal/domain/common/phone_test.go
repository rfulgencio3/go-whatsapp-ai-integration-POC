package common

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

func TestPhoneNumberLookupCandidates(t *testing.T) {
	t.Parallel()

	tests := map[string][]string{
		"+55 (34) 98828-3531": {"5534988283531", "553488283531"},
		"553488283531":        {"553488283531", "5534988283531"},
		"3488567673":          {"553488567673", "5534988567673"},
	}

	for input, expected := range tests {
		got := PhoneNumberLookupCandidates(input)
		if len(got) != len(expected) {
			t.Fatalf("PhoneNumberLookupCandidates(%q) returned %d candidates, want %d", input, len(got), len(expected))
		}
		for i := range expected {
			if got[i] != expected[i] {
				t.Fatalf("PhoneNumberLookupCandidates(%q)[%d] = %q, want %q", input, i, got[i], expected[i])
			}
		}
	}
}
