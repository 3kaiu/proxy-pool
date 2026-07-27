package circuit

import (
	"sync"
	"time"
)

type State int

const (
	StateClosed State = iota
	StateOpen
	StateHalfOpen
)

type Breaker struct {
	mu          sync.Mutex
	threshold   int
	cooldown    time.Duration
	failCount   int
	state       State
	openedAt    time.Time
}

func New(threshold int, cooldown time.Duration) *Breaker {
	return &Breaker{
		threshold: threshold,
		cooldown:  cooldown,
		state:     StateClosed,
	}
}

func (b *Breaker) RecordSuccess() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failCount = 0
	b.state = StateClosed
}

func (b *Breaker) RecordFailure() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failCount++
	if b.failCount >= b.threshold && b.state == StateClosed {
		b.state = StateOpen
		b.openedAt = time.Now()
	}
}

func (b *Breaker) IsOpen() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.state == StateClosed {
		return false
	}

	if b.state == StateOpen {
		if time.Since(b.openedAt) > b.cooldown {
			b.state = StateHalfOpen
			b.failCount = 0
			return false // allow probing
		}
		return true // still cooling down
	}

	// StateHalfOpen
	return false
}
