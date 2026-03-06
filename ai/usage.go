package ai

import (
	"fmt"
	"sync"
)

// TODO: Gate this behind a -v/--verbose flag instead of always printing.

// Per-million-token pricing by model (input, output, cache_write, cache_read).
var modelPricing = map[string][4]float64{
	"claude-haiku-4-5":           {1.00, 5.00, 1.25, 0.10},
	"claude-haiku-4-5-20251001":  {1.00, 5.00, 1.25, 0.10},
	"claude-sonnet-4-5":          {3.00, 15.00, 3.75, 0.30},
	"claude-sonnet-4-5-20250929": {3.00, 15.00, 3.75, 0.30},
	"claude-opus-4-5":            {15.00, 75.00, 18.75, 1.50},
	"claude-opus-4-5-20251101":   {15.00, 75.00, 18.75, 1.50},
}

// Usage tracks cumulative token usage across API calls.
var Usage = &tokenUsage{
	models: make(map[string]*modelStats),
}

type modelStats struct {
	input            int64
	output           int64
	cacheCreation    int64
	cacheRead        int64
	calls            int
}

type tokenUsage struct {
	mu     sync.Mutex
	models map[string]*modelStats
}

func (u *tokenUsage) Add(model string, input, output, cacheCreation, cacheRead int64) {
	u.mu.Lock()
	defer u.mu.Unlock()
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
	}

	if totalCalls == 0 {
		return
	}

	for model, stats := range u.models {
		if pricing, ok := modelPricing[model]; ok {
			totalCost += float64(stats.input) / 1_000_000 * pricing[0]
			totalCost += float64(stats.output) / 1_000_000 * pricing[1]
			totalCost += float64(stats.cacheCreation) / 1_000_000 * pricing[2]
			totalCost += float64(stats.cacheRead) / 1_000_000 * pricing[3]
		}
	}

	totalTokens := totalInput + totalOutput + totalCacheCreation + totalCacheRead
	fmt.Printf("\033[2m[%d API call(s): %d tokens ~$%.4f]\033[0m\n",
		totalCalls, totalTokens, totalCost)
}
