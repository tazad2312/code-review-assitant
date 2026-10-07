package review

import (
	"context"
	"fmt"
	"strings"
)

type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityHigh     Severity = "high"
	SeverityMedium   Severity = "medium"
	SeverityLow      Severity = "low"
)

type Comment struct {
	File     string   `json:"file"`
	Line     int      `json:"line"`
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`
	Category string   `json:"category"`
}

type Provider interface {
	Review(context.Context, string) ([]Comment, error)
}

func ValidateComments(comments []Comment) error {
	for i, comment := range comments {
		if strings.TrimSpace(comment.File) == "" {
			return fmt.Errorf("comment %d has no file", i)
		}
		if comment.Line < 1 {
			return fmt.Errorf("comment %d has invalid line %d", i, comment.Line)
		}
		switch comment.Severity {
		case SeverityCritical, SeverityHigh, SeverityMedium, SeverityLow:
		default:
			return fmt.Errorf("comment %d has invalid severity %q", i, comment.Severity)
		}
		if strings.TrimSpace(comment.Message) == "" {
			return fmt.Errorf("comment %d has no message", i)
		}
		if strings.TrimSpace(comment.Category) == "" {
			return fmt.Errorf("comment %d has no category", i)
		}
	}
	return nil
}
