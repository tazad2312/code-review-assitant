package review

import (
	"context"
	"sync"
)

type MockProvider struct {
	mu       sync.Mutex
	Comments []Comment
	Calls    int
}

func (p *MockProvider) Review(_ context.Context, _ string) ([]Comment, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Calls++
	return append([]Comment(nil), p.Comments...), nil
}
