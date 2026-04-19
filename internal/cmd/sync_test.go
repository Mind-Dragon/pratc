package cmd

import (
	"context"
	"fmt"
	"testing"
)

func TestAttemptTokenFallback_UsesSecondTokenOnRetryableError(t *testing.T) {
	orig := discoverTokensFn
	defer func() { discoverTokensFn = orig }()

	discoverTokensFn = func(context.Context) ([]string, error) {
		return []string{"token-1-fails", "token-2-succeeds"}, nil
	}

	ctx := context.Background()
	var used []string

	err := attemptTokenFallback(ctx, func(token string) error {
		used = append(used, token)
		if token == "token-1-fails" {
			return fmt.Errorf("401 Unauthorized")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("attemptTokenFallback() error = %v, want nil", err)
	}
	if got, want := len(used), 2; got != want {
		t.Fatalf("attemptTokenFallback() used %d tokens, want %d (used=%v)", got, want, used)
	}
	if used[1] != "token-2-succeeds" {
		t.Fatalf("attemptTokenFallback() second token = %q, want %q", used[1], "token-2-succeeds")
	}
}
