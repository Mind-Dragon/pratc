package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jeffersonnunn/pratc/internal/cache"
	"github.com/jeffersonnunn/pratc/internal/github"
	"github.com/jeffersonnunn/pratc/internal/logger"
)

var openCacheStoreFn = cache.Open

var sleepUntilFn = func(ctx context.Context, until time.Time) error {
	if until.IsZero() {
		return nil
	}
	d := time.Until(until)
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func openSyncStoreWithFallback(dbPath string, log *logger.Logger) (*cache.Store, bool, error) {
	if log != nil {
		log.Info("opening sync cache store", "db_path", dbPath)
	}
	store, err := openCacheStoreFn(dbPath)
	if err == nil {
		if log != nil {
			log.Info("sync cache store opened", "db_path", dbPath, "fallback", false)
		}
		return store, false, nil
	}

	fallbackPath := fmt.Sprintf("file:pratc-sync-fallback-%d?mode=memory&cache=shared", time.Now().UnixNano())
	fallback, fallbackErr := cache.Open(fallbackPath)
	if fallbackErr != nil {
		return nil, false, fmt.Errorf("open cache store: %w; fallback open failed: %v", err, fallbackErr)
	}
	if log != nil {
		log.Warn("cache store unavailable, using ephemeral fallback", "db_path", dbPath, "error", err)
	}
	return fallback, true, nil
}

func attemptTokenFallbackWithTrace(ctx context.Context, log *logger.Logger, attempt func(token string) error) error {
	tokens, err := discoverTokensFn(ctx)
	if err != nil {
		return err
	}
	if len(tokens) == 0 {
		return github.AttemptWithTokenFallback(ctx, tokens, attempt)
	}
	if log != nil {
		log.Info("token pool discovered", "token_count", len(tokens))
	}
	var lastErr error
	for i, token := range tokens {
		if log != nil {
			log.Info("selected GitHub token", "token_index", i, "token_count", len(tokens))
		}
		lastErr = attempt(token)
		if lastErr == nil {
			if log != nil {
				log.Info("GitHub token succeeded", "token_index", i)
			}
			return nil
		}
		if !github.IsRetryableError(lastErr) {
			if log != nil {
				log.Error("GitHub token failed with hard error", "token_index", i, "error", lastErr.Error())
			}
			return lastErr
		}
		if log != nil {
			log.Warn("GitHub token exhausted, rotating", "token_index", i, "error", lastErr.Error())
		}
	}
	if log != nil && lastErr != nil {
		log.Error("GitHub token pool exhausted", "error", lastErr.Error())
	}
	return lastErr
}

func attemptTokenFallback(ctx context.Context, attempt func(token string) error) error {
	return attemptTokenFallbackWithTrace(ctx, nil, attempt)
}

func waitAndRetrySync(
	ctx context.Context,
	wait bool,
	resumeAtFn func() time.Time,
	pause func(time.Time, string) error,
	sleepUntil func(context.Context, time.Time) error,
	attempt func() error,
) error {
	if sleepUntil == nil {
		sleepUntil = sleepUntilFn
	}
	for {
		err := attempt()
		if err == nil {
			return nil
		}
		if !strings.Contains(err.Error(), "rate limit budget exhausted") {
			return err
		}
		resumeAt := time.Time{}
		if resumeAtFn != nil {
			resumeAt = resumeAtFn()
		}
		if pause != nil {
			if pauseErr := pause(resumeAt, err.Error()); pauseErr != nil {
				return pauseErr
			}
		}
		if !wait {
			return err
		}
		if err := sleepUntil(ctx, resumeAt); err != nil {
			return err
		}
	}
}
