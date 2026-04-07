package common

import (
	"strings"
	"unicode"
)

func NormalizePhoneNumber(value string) string {
	digits := strings.Map(func(r rune) rune {
		if unicode.IsDigit(r) {
			return r
		}
		return -1
	}, strings.TrimSpace(value))

	digits = strings.TrimPrefix(digits, "00")
	switch len(digits) {
	case 10, 11:
		return "55" + digits
	default:
		return digits
	}
}

func PhoneNumberLookupCandidates(value string) []string {
	normalized := NormalizePhoneNumber(value)
	if normalized == "" {
		return nil
	}

	candidates := []string{normalized}
	if alternate := alternateBrazilianMobileNumber(normalized); alternate != "" && alternate != normalized {
		candidates = append(candidates, alternate)
	}

	return candidates
}

func alternateBrazilianMobileNumber(value string) string {
	if !strings.HasPrefix(value, "55") {
		return ""
	}

	local := strings.TrimPrefix(value, "55")
	switch len(local) {
	case 10:
		return "55" + local[:2] + "9" + local[2:]
	case 11:
		if local[2] == '9' {
			return "55" + local[:2] + local[3:]
		}
	}

	return ""
}
