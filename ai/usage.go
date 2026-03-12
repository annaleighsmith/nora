package ai

import (
	"fmt"
	"sync"

	"github.com/fatih/color"
)

// TODO: Gate this behind a -v/--verbose flag instead of always printing.

// Per-million-token pricing by model (input, output, cache_write, cache_read).
var modelPricing = map[string][4]float64{
	// Haiku 4.5: $1/$5 input/output
	"claude-haiku-4-5": {1.00, 5.00, 1.25, 0.10},
	// Sonnet 4/4.5/4.6: $3/$15 input/output
	"claude-sonnet-4":   {3.00, 15.00, 3.75, 0.30},
	"claude-sonnet-4-5": {3.00, 15.00, 3.75, 0.30},
	"claude-sonnet-4-6": {3.00, 15.00, 3.75, 0.30},
	// Opus 4/4.1: $15/$75 input/output
	"claude-opus-4":   {15.00, 75.00, 18.75, 1.50},
	"claude-opus-4-1": {15.00, 75.00, 18.75, 1.50},
	// Opus 4.5/4.6: $5/$25 input/output
	"claude-opus-4-5": {5.00, 25.00, 6.25, 0.50},
	"claude-opus-4-6": {5.00, 25.00, 6.25, 0.50},
}

// Usage tracks cumulative token usage across API calls.
var Usage = &tokenUsage{
	models: make(map[string]*modelStats),
}

type modelStats struct {
	input         int64
	output        int64
	cacheCreation int64
	cacheRead     int64
	calls         int
	cost          float64
	extended      bool // true if current call's input exceeded 200k (for pricing output)
}

type tokenUsage struct {
	mu     sync.Mutex
	models map[string]*modelStats
}

// extendedContextThreshold is the input token count above which the API
// bills at 2x rates. Total input = input + cache_creation + cache_read.
const extendedContextThreshold = 200_000

func (u *tokenUsage) Add(model string, input, output, cacheCreation, cacheRead int64) {
	u.mu.Lock()

	stats, ok := u.models[model]
	if !ok {
		stats = &modelStats{}
		u.models[model] = stats
	}
	stats.input += input
	stats.output += output
	stats.cacheCreation += cacheCreation
	stats.cacheRead += cacheRead
	if input > 0 || output > 0 || cacheCreation > 0 || cacheRead > 0 {
		stats.calls++
	}

	pricing, ok := modelPricing[model]
	if !ok {
		u.mu.Unlock()
		return
	}

	// Input tokens arrive in message_start; detect extended context pricing.
	var warning string
	if input > 0 || cacheCreation > 0 || cacheRead > 0 {
		totalInput := input + cacheCreation + cacheRead
		stats.extended = totalInput > extendedContextThreshold
		if stats.extended {
			warning = fmt.Sprintf("[warning: %dk input tokens — billed at extended context rates]\n", totalInput/1000)
		}
	}

	mult := 1.0
	if stats.extended {
		mult = 2.0
	}

	stats.cost += float64(input) / 1_000_000 * pricing[0] * mult
	stats.cost += float64(output) / 1_000_000 * pricing[1] * mult
	stats.cost += float64(cacheCreation) / 1_000_000 * pricing[2] * mult
	stats.cost += float64(cacheRead) / 1_000_000 * pricing[3] * mult

	u.mu.Unlock()

	if warning != "" {
		dim := color.New(color.Faint)
		dim.Print(warning)
	}
}

func (u *tokenUsage) Print() {
	u.mu.Lock()
	defer u.mu.Unlock()

	var totalCalls int
	var totalInput, totalOutput, totalCacheCreation, totalCacheRead int64
	var totalCost float64

	for _, stats := range u.models {
		totalCalls += stats.calls
		totalInput += stats.input
		totalOutput += stats.output
		totalCacheCreation += stats.cacheCreation
		totalCacheRead += stats.cacheRead
		totalCost += stats.cost
	}

	if totalCalls == 0 {
		return
	}

	totalTokens := totalInput + totalOutput + totalCacheCreation + totalCacheRead
	dim := color.New(color.Faint)
	dim.Printf("[%d API call(s): %d tokens ~$%.4f]\n",
		totalCalls, totalTokens, totalCost)
}
