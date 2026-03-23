package planning

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/jeffersonnunn/pratc/internal/types"
)

func TestTimeDecayWindow_NewTimeDecayWindow(t *testing.T) {
	fixedTime := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)
	prs := []types.PR{
		{Number: 1, CreatedAt: "2026-03-21T08:00:00Z", Labels: []string{"bug"}},
		{Number: 2, CreatedAt: "2026-03-18T08:00:00Z", Labels: []string{"security"}},
		{Number: 3, CreatedAt: "2026-03-01T08:00:00Z", Labels: []string{"cve", "urgent"}},
	}

	config := TimeDecayConfig{
		HalfLifeHours:  72.0,
		WindowHours:    168.0,
		ProtectedHours: 336.0,
		MinScore:       0.6,
	}

	window := NewTimeDecayWindow(prs, config, fixedTime)

	if window == nil {
		t.Fatal("NewTimeDecayWindow returned nil")
	}
	if len(window.scores) != 3 {
		t.Errorf("Expected 3 scores, got %d", len(window.scores))
	}
	if len(window.protected) != 3 {
		t.Errorf("Expected 3 protected entries, got %d", len(window.protected))
	}
}

func TestTimeDecayWindow_ScorePR_ExponentialDecay(t *testing.T) {
	fixedTime := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		createdAt   string
		halfLife    float64
		expectedMin float64
		expectedMax float64
		shouldDecay bool
	}{
		{
			name:        "brand_new_pr_no_decay",
			createdAt:   "2026-03-21T11:00:00Z",
			halfLife:    72.0,
			expectedMin: 0.99,
			expectedMax: 1.0,
			shouldDecay: false,
		},
		{
			name:        "one_day_old_half_life_72h",
			createdAt:   "2026-03-20T12:00:00Z",
			halfLife:    72.0,
			expectedMin: 0.78,
			expectedMax: 0.82,
			shouldDecay: true,
		},
		{
			name:        "three_days_old_half_life_72h",
			createdAt:   "2026-03-18T12:00:00Z",
			halfLife:    72.0,
			expectedMin: 0.49,
			expectedMax: 0.51,
			shouldDecay: true,
		},
		{
			name:        "seven_days_old_half_life_72h",
			createdAt:   "2026-03-14T12:00:00Z",
			halfLife:    72.0,
			expectedMin: 0.18,
			expectedMax: 0.22,
			shouldDecay: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := TimeDecayConfig{
				HalfLifeHours:  tt.halfLife,
				WindowHours:    168.0,
				ProtectedHours: 336.0,
				MinScore:       0.6,
			}

			pr := types.PR{
				Number:    1,
				CreatedAt: tt.createdAt,
				Labels:    []string{"bug"},
			}

			window := NewTimeDecayWindow([]types.PR{pr}, config, fixedTime)
			score := window.ScorePR(pr)

			if score < tt.expectedMin || score > tt.expectedMax {
				t.Errorf("Score %v not in expected range [%v, %v]", score, tt.expectedMin, tt.expectedMax)
			}

			// Verify decay formula: score = e^(-ln(2) * ageHours / halfLifeHours)
			if tt.shouldDecay {
				createdAt, _ := time.Parse(time.RFC3339, tt.createdAt)
				hoursSinceCreation := fixedTime.Sub(createdAt).Hours()
				expectedDecay := math.Exp(-math.Ln2 * hoursSinceCreation / tt.halfLife)

				if math.Abs(score-expectedDecay) > 0.01 {
					t.Errorf("Decay formula mismatch: got %v, expected %v (diff: %v)", score, expectedDecay, math.Abs(score-expectedDecay))
				}
			}
		})
	}
}

func TestTimeDecayWindow_ProtectedLane(t *testing.T) {
	fixedTime := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		labels      []string
		createdAt   string
		isProtected bool
	}{
		{
			name:        "security_label_old_pr",
			labels:      []string{"security"},
			createdAt:   "2026-03-01T08:00:00Z", // 20 days old
			isProtected: true,
		},
		{
			name:        "cve_label_old_pr",
			labels:      []string{"cve"},
			createdAt:   "2026-03-01T08:00:00Z",
			isProtected: true,
		},
		{
			name:        "urgent_label_old_pr",
			labels:      []string{"urgent"},
			createdAt:   "2026-03-01T08:00:00Z",
			isProtected: true,
		},
		{
			name:        "vulnerability_label_old_pr",
			labels:      []string{"vulnerability"},
			createdAt:   "2026-03-01T08:00:00Z",
			isProtected: true,
		},
		{
			name:        "security_fix_label_old_pr",
			labels:      []string{"security-fix"},
			createdAt:   "2026-03-01T08:00:00Z",
			isProtected: true,
		},
		{
			name:        "bug_label_old_pr_not_protected",
			labels:      []string{"bug"},
			createdAt:   "2026-03-01T08:00:00Z",
			isProtected: false,
		},
		{
			name:        "security_label_new_pr_not_protected",
			labels:      []string{"security"},
			createdAt:   "2026-03-21T08:00:00Z", // 4 hours old
			isProtected: true,                   // Has protected label, but won't get score boost (not old enough)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := TimeDecayConfig{
				HalfLifeHours:  72.0,
				WindowHours:    168.0,
				ProtectedHours: 336.0, // 14 days
				MinScore:       0.6,
			}

			pr := types.PR{
				Number:    1,
				CreatedAt: tt.createdAt,
				Labels:    tt.labels,
			}

			window := NewTimeDecayWindow([]types.PR{pr}, config, fixedTime)

			protected := window.IsProtected(pr.Number)
			if protected != tt.isProtected {
				t.Errorf("IsProtected() = %v, want %v", protected, tt.isProtected)
			}

			if tt.isProtected {
				score := window.ScorePR(pr)
				if score < config.MinScore {
					t.Errorf("Protected PR score %v < MinScore %v", score, config.MinScore)
				}
			}
		})
	}
}

func TestTimeDecayWindow_SelectProtected(t *testing.T) {
	fixedTime := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)

	prs := []types.PR{
		{Number: 1, CreatedAt: "2026-03-01T08:00:00Z", Labels: []string{"security"}},
		{Number: 2, CreatedAt: "2026-03-01T08:00:00Z", Labels: []string{"bug"}},
		{Number: 3, CreatedAt: "2026-03-01T08:00:00Z", Labels: []string{"cve", "urgent"}},
		{Number: 4, CreatedAt: "2026-03-20T08:00:00Z", Labels: []string{"security"}}, // New security PR
		{Number: 5, CreatedAt: "2026-03-01T08:00:00Z", Labels: []string{"hotfix"}},
	}

	config := TimeDecayConfig{
		HalfLifeHours:  72.0,
		WindowHours:    168.0,
		ProtectedHours: 336.0,
		MinScore:       0.6,
	}

	window := NewTimeDecayWindow(prs, config, fixedTime)
	protected := window.SelectProtected()

	// Should have 3 protected PRs (1, 3, 5) - old enough AND have protected labels
	// PR 4 has security label but is only 1 day old (not old enough for protected lane)
	if len(protected) != 3 {
		t.Errorf("Expected 3 protected PRs, got %d", len(protected))
	}

	// Verify the correct PRs are protected
	protectedNumbers := make(map[int]bool)
	for _, pr := range protected {
		protectedNumbers[pr.Number] = true
	}

	expectedProtected := []int{1, 3, 5}
	for _, num := range expectedProtected {
		if !protectedNumbers[num] {
			t.Errorf("Expected PR %d to be protected", num)
		}
	}
}

func TestTimeDecayWindow_GetWindowStats(t *testing.T) {
	fixedTime := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)

	prs := []types.PR{
		{Number: 1, CreatedAt: "2026-03-21T08:00:00Z", Labels: []string{"bug"}},      // 4h old, eligible
		{Number: 2, CreatedAt: "2026-03-18T08:00:00Z", Labels: []string{"bug"}},      // 3d old, eligible
		{Number: 3, CreatedAt: "2026-03-01T08:00:00Z", Labels: []string{"security"}}, // 20d old, protected
		{Number: 4, CreatedAt: "2026-02-01T08:00:00Z", Labels: []string{"bug"}},      // 48d old, outside window
	}

	config := TimeDecayConfig{
		HalfLifeHours:  72.0,
		WindowHours:    168.0, // 7 days
		ProtectedHours: 336.0, // 14 days
		MinScore:       0.6,
	}

	window := NewTimeDecayWindow(prs, config, fixedTime)
	stats := window.GetWindowStats()

	// 2 PRs within 7-day window (PR 1, 2)
	if stats.EligibleCount != 2 {
		t.Errorf("EligibleCount = %d, want 2", stats.EligibleCount)
	}

	// 1 PR in protected lane (PR 3)
	if stats.ProtectedCount != 1 {
		t.Errorf("ProtectedCount = %d, want 1", stats.ProtectedCount)
	}

	// Decay min should be > 0 (no PR gets zero score)
	if stats.DecayMin <= 0 {
		t.Errorf("DecayMin = %v, should be > 0", stats.DecayMin)
	}

	// Decay max should be close to 1.0 for new PR
	if stats.DecayMax < 0.9 {
		t.Errorf("DecayMax = %v, should be close to 1.0", stats.DecayMax)
	}

	// Decay avg should be between min and max
	if stats.DecayAvg < stats.DecayMin || stats.DecayAvg > stats.DecayMax {
		t.Errorf("DecayAvg = %v, should be between min %v and max %v", stats.DecayAvg, stats.DecayMin, stats.DecayMax)
	}
}

func TestTimeDecayWindow_GetReasonCodes(t *testing.T) {
	fixedTime := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name          string
		pr            types.PR
		expectedCodes []string
	}{
		{
			name: "recent_pr_in_window",
			pr: types.PR{
				Number:    1,
				CreatedAt: "2026-03-21T08:00:00Z",
				Labels:    []string{"bug"},
			},
			expectedCodes: []string{ReasonCodeTimeDecayWindow, ReasonCodeRecentWindow, ReasonCodeDecayApplied},
		},
		{
			name: "old_protected_pr",
			pr: types.PR{
				Number:    2,
				CreatedAt: "2026-03-01T08:00:00Z",
				Labels:    []string{"security"},
			},
			expectedCodes: []string{ReasonCodeProtectedLane, ReasonCodeOutsideWindow, ReasonCodeDecayApplied},
		},
		{
			name: "old_unprotected_pr",
			pr: types.PR{
				Number:    3,
				CreatedAt: "2026-02-01T08:00:00Z",
				Labels:    []string{"bug"},
			},
			expectedCodes: []string{ReasonCodeOutsideWindow, ReasonCodeBelowMinScore, ReasonCodeDecayApplied},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := TimeDecayConfig{
				HalfLifeHours:  72.0,
				WindowHours:    168.0,
				ProtectedHours: 336.0,
				MinScore:       0.6,
			}

			window := NewTimeDecayWindow([]types.PR{tt.pr}, config, fixedTime)
			codes := window.GetReasonCodes(tt.pr)

			codeMap := make(map[string]bool)
			for _, code := range codes {
				codeMap[code] = true
			}

			for _, expected := range tt.expectedCodes {
				if !codeMap[expected] {
					t.Errorf("Expected reason code %q, got %v", expected, codes)
				}
			}
		})
	}
}

func TestTimeDecayWindow_NoPRsNeverExcluded(t *testing.T) {
	fixedTime := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)

	// Very old PR (1 year old)
	pr := types.PR{
		Number:    1,
		CreatedAt: "2025-03-21T12:00:00Z",
		Labels:    []string{"bug"},
	}

	config := TimeDecayConfig{
		HalfLifeHours:  72.0,
		WindowHours:    168.0,
		ProtectedHours: 336.0,
		MinScore:       0.6,
	}

	window := NewTimeDecayWindow([]types.PR{pr}, config, fixedTime)
	score := window.ScorePR(pr)

	// Even very old PRs should get non-zero score
	if score <= 0 {
		t.Errorf("Old PR score = %v, should be > 0 (no permanent exclusion)", score)
	}
}

func TestTimeDecayWindow_ConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  TimeDecayConfig
		wantErr bool
	}{
		{
			name: "valid_config",
			config: TimeDecayConfig{
				HalfLifeHours:  72.0,
				WindowHours:    168.0,
				ProtectedHours: 336.0,
				MinScore:       0.6,
			},
			wantErr: false,
		},
		{
			name: "zero_half_life_allowed",
			config: TimeDecayConfig{
				HalfLifeHours:  0,
				WindowHours:    168.0,
				ProtectedHours: 336.0,
				MinScore:       0.6,
			},
			wantErr: false,
		},
		{
			name: "negative_half_life_error",
			config: TimeDecayConfig{
				HalfLifeHours:  -1,
				WindowHours:    168.0,
				ProtectedHours: 336.0,
				MinScore:       0.6,
			},
			wantErr: true,
		},
		{
			name: "zero_window_hours_error",
			config: TimeDecayConfig{
				HalfLifeHours:  72.0,
				WindowHours:    0,
				ProtectedHours: 336.0,
				MinScore:       0.6,
			},
			wantErr: true,
		},
		{
			name: "zero_protected_hours_error",
			config: TimeDecayConfig{
				HalfLifeHours:  72.0,
				WindowHours:    168.0,
				ProtectedHours: 0,
				MinScore:       0.6,
			},
			wantErr: true,
		},
		{
			name: "negative_min_score_error",
			config: TimeDecayConfig{
				HalfLifeHours:  72.0,
				WindowHours:    168.0,
				ProtectedHours: 336.0,
				MinScore:       -0.1,
			},
			wantErr: true,
		},
		{
			name: "min_score_greater_than_one_error",
			config: TimeDecayConfig{
				HalfLifeHours:  72.0,
				WindowHours:    168.0,
				ProtectedHours: 336.0,
				MinScore:       1.1,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTimeDecayWindow_SettingsIntegration(t *testing.T) {
	t.Run("config_to_settings", func(t *testing.T) {
		config := TimeDecayConfig{
			HalfLifeHours:  96.0,
			WindowHours:    240.0,
			ProtectedHours: 480.0,
			MinScore:       0.7,
		}

		settings := TimeDecayConfigToSettings(config)

		if settings["half_life_hours"] != 96.0 {
			t.Errorf("half_life_hours = %v, want 96.0", settings["half_life_hours"])
		}
		if settings["window_hours"] != 240.0 {
			t.Errorf("window_hours = %v, want 240.0", settings["window_hours"])
		}
		if settings["protected_hours"] != 480.0 {
			t.Errorf("protected_hours = %v, want 480.0", settings["protected_hours"])
		}
		if settings["min_score"] != 0.7 {
			t.Errorf("min_score = %v, want 0.7", settings["min_score"])
		}
	})

	t.Run("settings_to_config", func(t *testing.T) {
		settings := map[string]any{
			"half_life_hours": 96.0,
			"window_hours":    240.0,
			"protected_hours": 480.0,
			"min_score":       0.7,
		}

		config, ok := TimeDecayConfigFromSettings(settings)
		if !ok {
			t.Fatal("TimeDecayConfigFromSettings returned false")
		}

		if config.HalfLifeHours != 96.0 {
			t.Errorf("HalfLifeHours = %v, want 96.0", config.HalfLifeHours)
		}
		if config.WindowHours != 240.0 {
			t.Errorf("WindowHours = %v, want 240.0", config.WindowHours)
		}
		if config.ProtectedHours != 480.0 {
			t.Errorf("ProtectedHours = %v, want 480.0", config.ProtectedHours)
		}
		if config.MinScore != 0.7 {
			t.Errorf("MinScore = %v, want 0.7", config.MinScore)
		}
	})

	t.Run("keys_to_settings", func(t *testing.T) {
		keys := TimeDecayKeysToSettings()

		expectedKeys := []string{
			"half_life_hours",
			"window_hours",
			"protected_hours",
			"min_score",
			SettingKeyTimeDecayConfig,
		}

		for _, key := range expectedKeys {
			if _, exists := keys[key]; !exists {
				t.Errorf("Expected key %q in settings keys", key)
			}
		}
	})
}

func TestTimeDecayWindow_IntegrationWithPoolSelector(t *testing.T) {
	fixedTime := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)

	prs := []types.PR{
		{Number: 1, CreatedAt: "2026-03-21T08:00:00Z", CIStatus: "success", Labels: []string{"bug"}},
		{Number: 2, CreatedAt: "2026-03-18T08:00:00Z", CIStatus: "success", Labels: []string{"bug"}},
		{Number: 3, CreatedAt: "2026-03-01T08:00:00Z", CIStatus: "success", Labels: []string{"security"}},
		{Number: 4, CreatedAt: "2026-02-01T08:00:00Z", CIStatus: "success", Labels: []string{"bug"}},
	}

	weights := PriorityWeights{
		StalenessWeight:        0.30,
		CIStatusWeight:         0.25,
		SecurityLabelWeight:    0.20,
		ClusterCoherenceWeight: 0.15,
		TimeDecayWeight:        0.10,
	}

	selector, err := NewPoolSelector(weights)
	if err != nil {
		t.Fatalf("NewPoolSelector error = %v", err)
	}
	selector.Now = func() time.Time { return fixedTime }

	config := TimeDecayConfig{
		HalfLifeHours:  72.0,
		WindowHours:    168.0,
		ProtectedHours: 336.0,
		MinScore:       0.6,
	}

	result := selector.SelectCandidates(context.Background(), "owner/repo", prs, 4, config)

	if result.SelectedCount != 4 {
		t.Errorf("SelectedCount = %d, want 4", result.SelectedCount)
	}

	// Verify time decay scores are in component scores
	for _, candidate := range result.Selected {
		if candidate.ComponentScores.TimeDecayScore <= 0 {
			t.Errorf("PR %d TimeDecayScore = %v, should be > 0", candidate.PR.Number, candidate.ComponentScores.TimeDecayScore)
		}
	}

	// Security PR (3) should have boosted score due to protected lane
	pr3Score := result.Selected[2].ComponentScores.TimeDecayScore
	if pr3Score < config.MinScore {
		t.Errorf("Protected PR 3 TimeDecayScore = %v, should be >= MinScore %v", pr3Score, config.MinScore)
	}
}

func BenchmarkTimeDecayWindow_ScoreAllPRs(b *testing.B) {
	fixedTime := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)

	// Generate 1000 PRs with varying ages
	prs := make([]types.PR, 1000)
	for i := 0; i < 1000; i++ {
		hoursAgo := time.Duration(i*2) * time.Hour
		prs[i] = types.PR{
			Number:    i + 1,
			CreatedAt: fixedTime.Add(-hoursAgo).Format(time.RFC3339),
			Labels:    []string{"bug"},
		}
	}

	config := TimeDecayConfig{
		HalfLifeHours:  72.0,
		WindowHours:    168.0,
		ProtectedHours: 336.0,
		MinScore:       0.6,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		window := NewTimeDecayWindow(prs, config, fixedTime)
		_ = window.GetWindowStats()
	}
}
