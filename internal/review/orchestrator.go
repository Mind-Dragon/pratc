package review

import (
	"github.com/jeffersonnunn/pratc/internal/settings"
)

type Orchestrator struct {
	analyzers     []Analyzer
	config        settings.AnalyzerConfig
	settingsStore *settings.Store
}

func NewOrchestrator(cfg settings.AnalyzerConfig, store *settings.Store) *Orchestrator {
	return &Orchestrator{
		analyzers:     make([]Analyzer, 0),
		config:        cfg,
		settingsStore: store,
	}
}

func (o *Orchestrator) RegisterAnalyzer(a Analyzer) {
	o.analyzers = append(o.analyzers, a)
}

func (o *Orchestrator) Analyzers() []Analyzer {
	result := make([]Analyzer, len(o.analyzers))
	copy(result, o.analyzers)
	return result
}

func (o *Orchestrator) Config() settings.AnalyzerConfig {
	return o.config
}

func (o *Orchestrator) SettingsStore() *settings.Store {
	return o.settingsStore
}
