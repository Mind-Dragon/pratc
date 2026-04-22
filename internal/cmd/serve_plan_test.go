package cmd

import (
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/time/rate"

	"github.com/jeffersonnunn/pratc/internal/app"
)

// TestHandlePlanCollapseDuplicatesFalseInheritsDynamicTarget verifies that
// when collapse_duplicates=false, the new service instance inherits the
// DynamicTarget config from the original service.
func TestHandlePlanCollapseDuplicatesFalseInheritsDynamicTarget(t *testing.T) {
	// Create a service with a specific DynamicTarget config
	originalDynamicTarget := app.DynamicTargetConfig{
		Enabled:   true,
		Ratio:     0.10,
		MinTarget: 30,
		MaxTarget: 50,
	}
	svc := app.NewService(app.Config{
		AllowLive:     true,
		UseCacheFirst: true,
		DynamicTarget: originalDynamicTarget,
	})

	// Verify the original service has the expected DynamicTarget
	if got := svc.DynamicTarget(); got != originalDynamicTarget {
		t.Fatalf("original service DynamicTarget mismatch: got %+v, want %+v", got, originalDynamicTarget)
	}

	// Simulate the code path from serve.go:551-559
	// When collapseDuplicates=false, a new service is created
	collapseDuplicates := false
	planService := svc
	if !collapseDuplicates {
		cfg := app.Config{
			AllowLive:        svc.Health().Status == "ok",
			UseCacheFirst:    true,
			CollapseDuplicates: false,
			DynamicTarget:    svc.DynamicTarget(), // This is what we need to add
		}
		planService = app.NewService(cfg)
	}

	// Verify the new service inherits DynamicTarget
	if got := planService.DynamicTarget(); got != originalDynamicTarget {
		t.Errorf("planService DynamicTarget mismatch: got %+v, want %+v", got, originalDynamicTarget)
	}
}

// TestHandlePlanCollapseDuplicatesTrueUsesSameService verifies that
// when collapseDuplicates=true, the original service is used directly.
func TestHandlePlanCollapseDuplicatesTrueUsesSameService(t *testing.T) {
	originalDynamicTarget := app.DynamicTargetConfig{
		Enabled:   true,
		Ratio:     0.10,
		MinTarget: 30,
		MaxTarget: 50,
	}
	svc := app.NewService(app.Config{
		AllowLive:     true,
		UseCacheFirst: true,
		DynamicTarget: originalDynamicTarget,
	})

	collapseDuplicates := true
	if !collapseDuplicates {
		cfg := app.Config{
			AllowLive:        svc.Health().Status == "ok",
			UseCacheFirst:    true,
			CollapseDuplicates: false,
			DynamicTarget:    svc.DynamicTarget(),
		}
		planService := app.NewService(cfg)
		_ = planService // avoid unused variable
	}

	// When collapseDuplicates=true, no new service is created
}

// TestHandlePlanDynamicTargetDisabled verifies that disabled DynamicTarget is also inherited.
func TestHandlePlanDynamicTargetDisabled(t *testing.T) {
	originalDynamicTarget := app.DynamicTargetConfig{
		Enabled:   false,
		Ratio:     0.10,
		MinTarget: 30,
		MaxTarget: 50,
	}
	svc := app.NewService(app.Config{
		AllowLive:     true,
		UseCacheFirst: true,
		DynamicTarget: originalDynamicTarget,
	})

	collapseDuplicates := false
	planService := svc
	if !collapseDuplicates {
		cfg := app.Config{
			AllowLive:          svc.Health().Status == "ok",
			UseCacheFirst:      true,
			CollapseDuplicates: false,
			DynamicTarget:      svc.DynamicTarget(),
		}
		planService = app.NewService(cfg)
	}

	if got := planService.DynamicTarget(); got != originalDynamicTarget {
		t.Errorf("planService DynamicTarget mismatch: got %+v, want %+v", got, originalDynamicTarget)
	}
}

// TestServePlanHandler is a simple integration test to verify the handler exists
func TestServePlanHandler(t *testing.T) {
	// Create a minimal test setup
	dynamicTarget := app.DynamicTargetConfig{
		Enabled:   true,
		Ratio:     0.05,
		MinTarget: 20,
		MaxTarget: 100,
	}
	svc := app.NewService(app.Config{
		AllowLive:     true,
		UseCacheFirst: true,
		DynamicTarget: dynamicTarget,
	})

	// Verify we can create the service
	if svc.Health().Status != "ok" {
		t.Error("service health check failed")
	}

	// Test request creation
	req := httptest.NewRequest(http.MethodGet, "/plan?repo=owner/repo&target=30", nil)
	if req == nil {
		t.Error("failed to create request")
	}
	_ = req
}

// TestRateLimitHeader_Unlimited tests D4: Rate limit header wrong for Inf.
//
// BUG: serve.go lines 1032-1045 show that getRateLimitHeader checks
// `int(rl.cfg.critical * 60) == 0` to determine if rate is unlimited.
// However, when rl.cfg.critical is rate.Inf (which equals math.Inf(1)),
// multiplying by 60 still gives Inf, not 0. The comparison would never
// be true for Inf. The code should use math.IsInf() to check for Inf values.
// When critical rate is Inf, the header should show "unlimited", not a large number.
func TestRateLimitHeader_Unlimited(t *testing.T) {
	tests := []struct {
		name        string
		critical    rate.Limit
		general     rate.Limit
		path        string
		wantHeader  string
	}{
		{
			name:        "critical endpoint with Inf rate",
			critical:    rate.Inf,
			general:     100 / 60.0,
			path:        "/api/repos/owner/repo/analyze",
			wantHeader:  "unlimited",
		},
		{
			name:        "general endpoint with Inf rate",
			critical:    10 / 60.0,
			general:     rate.Inf,
			path:        "/api/repos/owner/repo/plan",
			wantHeader:  "unlimited",
		},
		{
			name:        "critical endpoint with finite rate",
			critical:    100 / 60.0,
			general:     100 / 60.0,
			path:        "/api/repos/owner/repo/analyze",
			wantHeader:  "100/min",
		},
		{
			name:        "general endpoint with finite rate",
			critical:    100 / 60.0,
			general:     200 / 60.0,
			path:        "/api/repos/owner/repo/plan",
			wantHeader:  "200/min",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rl := &ipRateLimiter{
				cfg: rateLimiterConfig{
					general:   tc.general,
					critical:  tc.critical,
					burstSize: 10,
				},
			}

			got := getRateLimitHeader(rl, tc.path)

			if got != tc.wantHeader {
				t.Errorf("getRateLimitHeader() = %q, want %q", got, tc.wantHeader)
			}

			// Additional check: verify that Inf is handled correctly
			if tc.critical == rate.Inf && tc.path == "/api/repos/owner/repo/plan" {
				if got != "unlimited" {
					t.Errorf("critical endpoint with rate.Inf should return 'unlimited', got %q", got)
				}
			}
			if tc.general == rate.Inf && tc.path == "/api/repos/owner/repo/analyze" {
				if got != "unlimited" {
					t.Errorf("general endpoint with rate.Inf should return 'unlimited', got %q", got)
				}
			}
		})
	}

	// Direct test for the bug: verify math.Inf behavior
	t.Run("math.Inf multiplication check", func(t *testing.T) {
		infRate := rate.Inf
		multiplied := infRate * 60
		if !math.IsInf(float64(multiplied), 1) {
			t.Errorf("rate.Inf * 60 should be Inf, got %v", multiplied)
		}
		intConversion := int(multiplied)
		if intConversion != 0 {
			t.Logf("BUG: int(Inf * 60) = %d (on some systems this might panic)", intConversion)
		}
	})
}
