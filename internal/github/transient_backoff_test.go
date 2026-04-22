package github

import (
	"testing"
	"time"
)

func TestTransientBackoff_WithinDocumentedCap(t *testing.T) {
	for _, attempt := range []int{0, 1, 2, 5, 8, 10} {
		wait := transientBackoff(attempt)
		if wait > 30*time.Second {
			t.Fatalf("transientBackoff(%d) = %s, want <= 30s", attempt, wait)
		}
		if wait <= 0 {
			t.Fatalf("transientBackoff(%d) = %s, want > 0", attempt, wait)
		}
	}
}
