package app

import "testing"

func TestDynamicTargetConfig_ComputeDynamicTarget(t *testing.T) {
	tests := []struct {
		name       string
		cfg        DynamicTargetConfig
		viablePool int
		fallback   int
		want       int
	}{
		{
			name:       "disabled returns fallback",
			cfg:        DynamicTargetConfig{Enabled: false, Ratio: 0.05, MinTarget: 20, MaxTarget: 100},
			viablePool: 1000,
			fallback:   30,
			want:       30,
		},
		{
			name:       "zero pool returns min",
			cfg:        DynamicTargetConfig{Enabled: true, Ratio: 0.05, MinTarget: 20, MaxTarget: 100},
			viablePool: 0,
			fallback:   30,
			want:       20,
		},
		{
			name:       "negative pool returns min",
			cfg:        DynamicTargetConfig{Enabled: true, Ratio: 0.05, MinTarget: 20, MaxTarget: 100},
			viablePool: -5,
			fallback:   30,
			want:       20,
		},
		{
			name:       "small pool 100 PRs returns min (20)",
			cfg:        DynamicTargetConfig{Enabled: true, Ratio: 0.05, MinTarget: 20, MaxTarget: 100},
			viablePool: 100,
			fallback:   20,
			want:       20,
		},
		{
			name:       "medium pool 1000 PRs returns 50",
			cfg:        DynamicTargetConfig{Enabled: true, Ratio: 0.05, MinTarget: 20, MaxTarget: 100},
			viablePool: 1000,
			fallback:   20,
			want:       50,
		},
		{
			name:       "large pool 5000 PRs returns 100 (capped at max)",
			cfg:        DynamicTargetConfig{Enabled: true, Ratio: 0.05, MinTarget: 20, MaxTarget: 100},
			viablePool: 5000,
			fallback:   20,
			want:       100,
		},
		{
			name:       "pool 2000 PRs returns 100 (exactly at max with 0.05 ratio)",
			cfg:        DynamicTargetConfig{Enabled: true, Ratio: 0.05, MinTarget: 20, MaxTarget: 100},
			viablePool: 2000,
			fallback:   20,
			want:       100,
		},
		{
			name:       "pool 2001 PRs returns 100 (capped)",
			cfg:        DynamicTargetConfig{Enabled: true, Ratio: 0.05, MinTarget: 20, MaxTarget: 100},
			viablePool: 2001,
			fallback:   20,
			want:       100,
		},
		{
			name:       "custom min 50 overrides calculation",
			cfg:        DynamicTargetConfig{Enabled: true, Ratio: 0.05, MinTarget: 50, MaxTarget: 100},
			viablePool: 100,
			fallback:   20,
			want:       50,
		},
		{
			name:       "custom max 30 caps calculation",
			cfg:        DynamicTargetConfig{Enabled: true, Ratio: 0.05, MinTarget: 10, MaxTarget: 30},
			viablePool: 1000,
			fallback:   20,
			want:       30,
		},
		{
			name:       "custom ratio 0.10 with 1000 pool gives 100",
			cfg:        DynamicTargetConfig{Enabled: true, Ratio: 0.10, MinTarget: 20, MaxTarget: 100},
			viablePool: 1000,
			fallback:   20,
			want:       100,
		},
		{
			name:       "custom ratio 0.01 with 1000 pool gives 10 clamped to min 20",
			cfg:        DynamicTargetConfig{Enabled: true, Ratio: 0.01, MinTarget: 20, MaxTarget: 100},
			viablePool: 1000,
			fallback:   20,
			want:       20,
		},
		{
			name:       "pool 400 PRs with 5% ratio gives 20 exactly at min",
			cfg:        DynamicTargetConfig{Enabled: true, Ratio: 0.05, MinTarget: 20, MaxTarget: 100},
			viablePool: 400,
			fallback:   20,
			want:       20,
		},
		{
			name:       "pool 401 PRs with 5% ratio gives 20.05 floored to 20",
			cfg:        DynamicTargetConfig{Enabled: true, Ratio: 0.05, MinTarget: 20, MaxTarget: 100},
			viablePool: 401,
			fallback:   20,
			want:       20,
		},
		{
			name:       "pool 1 returns min",
			cfg:        DynamicTargetConfig{Enabled: true, Ratio: 0.05, MinTarget: 20, MaxTarget: 100},
			viablePool: 1,
			fallback:   20,
			want:       20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.ComputeDynamicTarget(tt.viablePool, tt.fallback)
			if got != tt.want {
				t.Errorf("ComputeDynamicTarget(%d, %d) = %d; want %d", tt.viablePool, tt.fallback, got, tt.want)
			}
		})
	}
}

// TestResolveDynamicTargetConfig_ZeroValueEnabledDefault verifies that a zero-valued
// DynamicTargetConfig (Enabled=false by default) results in Enabled=true per the
// "default to enabled" contract in the implementation comment.
func TestResolveDynamicTargetConfig_ZeroValueEnabledDefault(t *testing.T) {
	cfg := DynamicTargetConfig{}
	got := resolveDynamicTargetConfig(cfg)
	if got.Enabled != true {
		t.Errorf("resolveDynamicTargetConfig(DynamicTargetConfig{}) Enabled = %v; want true (default to enabled per comment)", got.Enabled)
	}
}

func TestResolveDynamicTargetConfig(t *testing.T) {
	tests := []struct {
		name  string
		input DynamicTargetConfig
		want  DynamicTargetConfig
	}{
		{
			name:  "zero values get defaults",
			input: DynamicTargetConfig{},
			want: DynamicTargetConfig{
				Ratio:     0.05,
				MinTarget: 20,
				MaxTarget: 100,
			},
		},
		{
			name: "negative ratio gets default",
			input: DynamicTargetConfig{
				Ratio:     -0.1,
				MinTarget: 10,
				MaxTarget: 50,
			},
			want: DynamicTargetConfig{
				Ratio:     0.05,
				MinTarget: 10,
				MaxTarget: 50,
			},
		},
		{
			name: "negative min gets default",
			input: DynamicTargetConfig{
				Ratio:     0.1,
				MinTarget: -5,
				MaxTarget: 50,
			},
			want: DynamicTargetConfig{
				Ratio:     0.1,
				MinTarget: 20,
				MaxTarget: 50,
			},
		},
		{
			name: "negative max gets default",
			input: DynamicTargetConfig{
				Ratio:     0.1,
				MinTarget: 10,
				MaxTarget: -50,
			},
			want: DynamicTargetConfig{
				Ratio:     0.1,
				MinTarget: 10,
				MaxTarget: 100,
			},
		},
		{
			name: "explicit enabled false is preserved",
			input: DynamicTargetConfig{
				Enabled:   false,
				Ratio:     0,
				MinTarget: 0,
				MaxTarget: 0,
			},
			want: DynamicTargetConfig{
				Enabled:   false,
				Ratio:     0.05,
				MinTarget: 20,
				MaxTarget: 100,
			},
		},
		{
			name: "explicit enabled true is preserved with custom values",
			input: DynamicTargetConfig{
				Enabled:   true,
				Ratio:     0.03,
				MinTarget: 15,
				MaxTarget: 75,
			},
			want: DynamicTargetConfig{
				Enabled:   true,
				Ratio:     0.03,
				MinTarget: 15,
				MaxTarget: 75,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveDynamicTargetConfig(tt.input)
			if got.Enabled != tt.want.Enabled {
				t.Errorf("resolveDynamicTargetConfig().Enabled = %v; want %v", got.Enabled, tt.want.Enabled)
			}
			if got.Ratio != tt.want.Ratio {
				t.Errorf("resolveDynamicTargetConfig().Ratio = %v; want %v", got.Ratio, tt.want.Ratio)
			}
			if got.MinTarget != tt.want.MinTarget {
				t.Errorf("resolveDynamicTargetConfig().MinTarget = %v; want %v", got.MinTarget, tt.want.MinTarget)
			}
			if got.MaxTarget != tt.want.MaxTarget {
				t.Errorf("resolveDynamicTargetConfig().MaxTarget = %v; want %v", got.MaxTarget, tt.want.MaxTarget)
			}
		})
	}
}
