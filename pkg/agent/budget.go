package agent

import (
	"fmt"
	"sync"
)

// Budget tracks and limits agent resource consumption
type Budget struct {
	MaxIterations int
	MaxTokens     int
	MaxCostUSD    float64

	iterations int
	tokens     int
	cost       float64
	mu         sync.Mutex

	// Pricing per million tokens (configurable per model)
	InputPricePerM  float64
	OutputPricePerM float64
}

// NewBudget creates a budget with default Sonnet pricing
func NewBudget(maxIter, maxTokens int, maxCost float64) *Budget {
	return &Budget{
		MaxIterations:   maxIter,
		MaxTokens:       maxTokens,
		MaxCostUSD:      maxCost,
		InputPricePerM:  3.0,  // Claude Sonnet default
		OutputPricePerM: 15.0,
	}
}

// Check returns an error if any budget limit is exceeded
func (b *Budget) Check() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	switch {
	case b.iterations >= b.MaxIterations:
		return fmt.Errorf("iteration limit reached (%d/%d)", b.iterations, b.MaxIterations)
	case b.tokens >= b.MaxTokens:
		return fmt.Errorf("token limit reached (%d/%d)", b.tokens, b.MaxTokens)
	case b.cost >= b.MaxCostUSD:
		return fmt.Errorf("cost limit reached ($%.4f/$%.2f)", b.cost, b.MaxCostUSD)
	}
	return nil
}

// Record records token usage from an API response
func (b *Budget) Record(usage Usage) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.iterations++
	b.tokens += usage.InputTokens + usage.OutputTokens
	b.cost += float64(usage.InputTokens)*b.InputPricePerM/1_000_000 +
		float64(usage.OutputTokens)*b.OutputPricePerM/1_000_000
}

// Summary returns a human-readable budget summary
func (b *Budget) Summary() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return fmt.Sprintf("iterations=%d/%d tokens=%d/%d cost=$%.4f/$%.2f",
		b.iterations, b.MaxIterations, b.tokens, b.MaxTokens, b.cost, b.MaxCostUSD)
}

// Remaining returns remaining budget in each dimension
func (b *Budget) Remaining() (iters, tokens int, cost float64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.MaxIterations - b.iterations, b.MaxTokens - b.tokens, b.MaxCostUSD - b.cost
}
