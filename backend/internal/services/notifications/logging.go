package notifications

import (
	"encoding/json"
	"os"
	"time"
)

// LogEntry represents a structured log entry
type LogEntry struct {
	Timestamp string                 `json:"timestamp"`
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Context   map[string]interface{} `json:"context,omitempty"`
}

// LogInfo logs an informational message
func LogInfo(message string, ctx map[string]interface{}) {
	writeLog("INFO", message, ctx)
}

// LogError logs an error message
func LogError(message string, err error, ctx map[string]interface{}) {
	if ctx == nil {
		ctx = make(map[string]interface{})
	}
	if err != nil {
		ctx["error"] = err.Error()
	}
	writeLog("ERROR", message, ctx)
}

// LogWarn logs a warning message
func LogWarn(message string, ctx map[string]interface{}) {
	writeLog("WARN", message, ctx)
}

func writeLog(level, message string, ctx map[string]interface{}) {
	entry := LogEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Level:     level,
		Message:   message,
		Context:   ctx,
	}
	json.NewEncoder(os.Stdout).Encode(entry)
}
