package review

import (
	"context"
	"fmt"
)

type Service struct {
	provider    Provider
	cache       *LRUCache
	tokenBudget int
}

func NewService(provider Provider, cache *LRUCache, tokenBudget int) (*Service, error) {
	if provider == nil {
		return nil, fmt.Errorf("review provider is required")
	}
	if tokenBudget < 1 {
		return nil, fmt.Errorf("token budget must be positive")
	}
	if cache == nil {
		cache = NewLRUCache(1)
	}
	return &Service{provider: provider, cache: cache, tokenBudget: tokenBudget}, nil
}

func (s *Service) Review(ctx context.Context, diff string) ([]Comment, error) {
	key := DiffHash(diff)
	if comments, ok := s.cache.Get(key); ok {
		return comments, nil
	}
	chunks, err := ChunkDiff(diff, s.tokenBudget)
	if err != nil {
		return nil, err
	}
	var comments []Comment
	seen := make(map[Comment]struct{})
	for _, chunk := range chunks {
		result, err := s.provider.Review(ctx, chunk)
		if err != nil {
			return nil, fmt.Errorf("review diff chunk: %w", err)
		}
		if err := ValidateComments(result); err != nil {
			return nil, fmt.Errorf("provider returned invalid comments: %w", err)
		}
		for _, comment := range result {
			if _, exists := seen[comment]; exists {
				continue
			}
			seen[comment] = struct{}{}
			comments = append(comments, comment)
		}
	}
	s.cache.Add(key, comments)
	return comments, nil
}
