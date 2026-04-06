package httpapi

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"errors"
	"net/url"
	"sort"
	"strings"
)

var errInvalidTwilioWebhookSignature = errors.New("invalid twilio webhook signature")

func validateTwilioWebhookSignature(signatureHeader string, requestURL string, payload []byte, authToken string) error {
	if strings.TrimSpace(authToken) == "" {
		return nil
	}

	values, err := url.ParseQuery(string(payload))
	if err != nil {
		return errInvalidTwilioWebhookSignature
	}

	expected := computeTwilioSignature(strings.TrimSpace(requestURL), values, authToken)
	if !hmac.Equal([]byte(strings.TrimSpace(signatureHeader)), []byte(expected)) {
		return errInvalidTwilioWebhookSignature
	}

	return nil
}

func computeTwilioSignature(requestURL string, values url.Values, authToken string) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var builder strings.Builder
	builder.WriteString(requestURL)
	for _, key := range keys {
		sortedValues := append([]string(nil), values[key]...)
		sort.Strings(sortedValues)
		for _, value := range sortedValues {
			builder.WriteString(key)
			builder.WriteString(value)
		}
	}

	mac := hmac.New(sha1.New, []byte(strings.TrimSpace(authToken)))
	_, _ = mac.Write([]byte(builder.String()))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}
