package observability

import (
	"encoding/json"
	"log"
	"os"
	"time"
)

type Logger struct {
	logger *log.Logger
}

type logEntry struct {
	Timestamp string         `json:"timestamp"`
	Level     string         `json:"level"`
	Message   string         `json:"message"`
	Fields    map[string]any `json:"fields,omitempty"`
}

func NewLogger() *Logger {
	return &Logger{logger: log.New(os.Stdout, "", 0)}
}

func (l *Logger) Info(message string, fields map[string]any) {
	l.log("info", message, fields)
}

func (l *Logger) Error(message string, fields map[string]any) {
	l.log("error", message, fields)
}

func (l *Logger) log(level, message string, fields map[string]any) {
	entry := logEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Level:     level,
		Message:   message,
		Fields:    fields,
	}

	payload, err := json.Marshal(entry)
	if err != nil {
		l.logger.Printf(`{"timestamp":%q,"level":"error","message":"encode log entry failed","fields":{"original_message":%q,"error":%q}}`, time.Now().UTC().Format(time.RFC3339Nano), message, err.Error())
		return
	}

	l.logger.Print(string(payload))
}
