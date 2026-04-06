package chat

import "errors"

var ErrPhoneNumberNotAllowed = errors.New("phone number is not allowed to use this chatbot")

var ErrUnsupportedMessageType = errors.New("unsupported inbound message type")
