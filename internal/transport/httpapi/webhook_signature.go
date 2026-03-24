package httpapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
)

const signaturePrefix = "sha256="

var errInvalidWebhookSignature = errors.New("invalid webhook signature")

func validateWebhookSignature(signatureHeader string, payload []byte, appSecret string) error {
	if strings.TrimSpace(appSecret) == "" {
		return nil
	}

	signatureHeader = strings.TrimSpace(signatureHeader)
	if len(signatureHeader) <= len(signaturePrefix) || !strings.EqualFold(signatureHeader[:len(signaturePrefix)], signaturePrefix) {
		return errInvalidWebhookSignature
	}

	providedSignature, err := hex.DecodeString(strings.TrimSpace(signatureHeader[len(signaturePrefix):]))
	if err != nil {
		return errInvalidWebhookSignature
	}

	mac := hmac.New(sha256.New, []byte(appSecret))
	_, _ = mac.Write(payload)
	expectedSignature := mac.Sum(nil)

	if !hmac.Equal(providedSignature, expectedSignature) {
		return errInvalidWebhookSignature
	}

	return nil
}
