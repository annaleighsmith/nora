package ai

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/annaleighsmith/nora/config"
)

// APILogEntry is a single JSON-lines record written to the debug log.
type APILogEntry struct {
	Timestamp     string  `json:"ts"`
	Caller        string  `json:"caller"`
	Model         string  `json:"model"`
	Streaming     bool    `json:"streaming,omitempty"`
	LatencyMs     int64   `json:"latency_ms"`
	InputTokens   int64   `json:"input_tokens"`
	OutputTokens  int64   `json:"output_tokens"`
	CacheCreation int64   `json:"cache_creation_tokens,omitempty"`
	CacheRead     int64   `json:"cache_read_tokens,omitempty"`
	TotalInput    int64   `json:"total_input"`
	Extended      bool    `json:"extended_context,omitempty"`
	CostUSD       float64 `json:"cost_usd"`
}

// apiLogger writes JSON-lines to ~/.config/nora/logs/api.log when debug is on.
type apiLogger struct {
	mu      sync.Mutex
	enabled bool
	file    *os.File
	enc     *json.Encoder
}

// DebugLog is the package-level API logger. Call InitDebugLog to enable it.
var DebugLog = &apiLogger{}

const maxLogSize = 5 * 1024 * 1024 // 5 MB

// InitDebugLog opens the log file if debug mode is enabled.
func InitDebugLog(debug bool) {
	if !debug {
		return
	}

	dir, err := config.LogsDir()
	if err != nil {
		return
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return
	}

	logPath := filepath.Join(dir, "api.log")

	// Rotate if file exceeds max size
	if info, err := os.Stat(logPath); err == nil && info.Size() > maxLogSize {
		os.Rename(logPath, logPath+".1")
	}

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return
	}

	DebugLog.mu.Lock()
	defer DebugLog.mu.Unlock()
	DebugLog.enabled = true
	DebugLog.file = f
	DebugLog.enc = json.NewEncoder(f)
}

// Close flushes and closes the log file.
func (l *apiLogger) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		l.file.Close()
		l.file = nil
		l.enabled = false
	}
}

// Log writes an entry if debug logging is enabled.
func (l *apiLogger) Log(e APILogEntry) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.enabled {
		return
	}

	e.Timestamp = time.Now().UTC().Format(time.RFC3339)
	e.TotalInput = e.InputTokens + e.CacheCreation + e.CacheRead
	e.Extended = e.TotalInput > extendedContextThreshold

	if pricing, ok := modelPricing[e.Model]; ok {
		mult := 1.0
		if e.Extended {
			mult = 2.0
		}
		e.CostUSD = float64(e.InputTokens)/1_000_000*pricing[0]*mult +
			float64(e.OutputTokens)/1_000_000*pricing[1]*mult +
			float64(e.CacheCreation)/1_000_000*pricing[2]*mult +
			float64(e.CacheRead)/1_000_000*pricing[3]*mult
	}

	l.enc.Encode(e)
}
