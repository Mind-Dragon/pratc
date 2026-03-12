package formula

type Mode string

const (
	ModePermutation     Mode = "permutation"
	ModeCombination     Mode = "combination"
	ModeWithReplacement Mode = "with_replacement"
)

type ScoreWeights struct {
	Age              float64 `json:"age"`
	Size             float64 `json:"size"`
	CIStatus         float64 `json:"ci_status"`
	ReviewStatus     float64 `json:"review_status"`
	ConflictCount    float64 `json:"conflict_count"`
	ClusterCoherence float64 `json:"cluster_coherence"`
}

type TierConfig struct {
	Name          string `json:"name"`
	MaxCandidates int    `json:"max_candidates"`
	Mode          Mode   `json:"mode"`
}

type Config struct {
	Name               string       `json:"name"`
	Mode               Mode         `json:"mode"`
	PickCount          int          `json:"pick_count"`
	MaxPoolSize        int          `json:"max_pool_size"`
	RequirePreFiltered bool         `json:"require_pre_filtered"`
	ScoreWeights       ScoreWeights `json:"score_weights"`
	Tiers              []TierConfig `json:"tiers"`
}

func DefaultConfig() Config {
	return Config{
		Name:               "pratc-formula-default",
		Mode:               ModeCombination,
		MaxPoolSize:        64,
		RequirePreFiltered: true,
		ScoreWeights: ScoreWeights{
			Age:              0.20,
			Size:             0.15,
			CIStatus:         0.20,
			ReviewStatus:     0.20,
			ConflictCount:    0.15,
			ClusterCoherence: 0.10,
		},
		Tiers: []TierConfig{
			{Name: TierQuick, MaxCandidates: 8},
			{Name: TierThorough, MaxCandidates: 12},
			{Name: TierExhaustive, MaxCandidates: 16},
		},
	}
}

func (c Config) withDefaults() Config {
	defaults := DefaultConfig()

	if c.Name == "" {
		c.Name = defaults.Name
	}
	if c.Mode == "" {
		c.Mode = defaults.Mode
	}
	if c.MaxPoolSize == 0 {
		c.MaxPoolSize = defaults.MaxPoolSize
	}
	if c.ScoreWeights == (ScoreWeights{}) {
		c.ScoreWeights = defaults.ScoreWeights
	}
	if len(c.Tiers) == 0 {
		c.Tiers = defaults.Tiers
	}

	return c
}
