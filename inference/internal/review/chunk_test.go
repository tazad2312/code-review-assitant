package review

import (
	"strings"
	"testing"
)

func TestChunkDiff(t *testing.T) {
	tests := []struct {
		name       string
		diff       string
		budget     int
		wantChunks int
		wantError  bool
	}{
		{
			name:       "groups small diff",
			diff:       "diff --git a/a.go b/a.go\n@@ -1 +1 @@\n-old\n+new\n",
			budget:     100,
			wantChunks: 1,
		},
		{
			name: "splits files",
			diff: "diff --git a/a.go b/a.go\n@@ -1 +1 @@\n-a\n+b\n" +
				"diff --git a/b.go b/b.go\n@@ -1 +1 @@\n-c\n+d\n",
			budget:     16,
			wantChunks: 2,
		},
		{
			name: "splits oversized hunk without dropping lines",
			diff: "diff --git a/a.go b/a.go\n@@ -1 +1 @@\n" +
				"-" + strings.Repeat("x", 30) + "\n+" + strings.Repeat("y", 30) + "\n",
			budget:     8,
			wantChunks: 2,
		},
		{name: "rejects invalid budget", diff: "diff", budget: 0, wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			chunks, err := ChunkDiff(test.diff, test.budget)
			if (err != nil) != test.wantError {
				t.Fatalf("ChunkDiff() error = %v, wantError %v", err, test.wantError)
			}
			if len(chunks) != test.wantChunks {
				t.Fatalf("ChunkDiff() returned %d chunks, want %d: %#v", len(chunks), test.wantChunks, chunks)
			}
			if !test.wantError {
				joined := strings.Join(chunks, "")
				for _, line := range strings.Split(strings.TrimSpace(test.diff), "\n") {
					if line != "" && !strings.Contains(joined, line) {
						t.Errorf("chunk output lost line %q", line)
					}
				}
			}
		})
	}
}
