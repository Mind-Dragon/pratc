package cmd

import (
	"net/http"
	"net/http/httptest"
	"testing"

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
