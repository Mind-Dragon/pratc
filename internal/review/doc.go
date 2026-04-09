// Package review provides agentic PR review orchestration for the prATC system.
//
// Purpose:
//
//	The review package coordinates multi-analyzer PR review workflows, aggregating
//	results from specialized analyzers to produce comprehensive review recommendations.
//	It serves as the orchestration layer that manages analyzer execution, result
//	aggregation, and review report generation.
//
// Key Types:
//
//	Orchestrator    - Coordinates analyzer execution and result aggregation
//	ReviewResult    - Aggregated output from all analyzers for a single PR
//	Analyzer        - Interface for individual PR analyzers (defined in analyzer.go)
//	AnalyzerResult  - Wrapper for individual analyzer output with metadata
//	PRData          - Input data structure containing PR metadata for analysis
//
// Relationship to analyzer package:
//
//	The review package builds upon the analyzer foundation defined in analyzer.go.
//	While analyzer.go defines the Analyzer interface and individual analysis
//	contracts, the review package (via Orchestrator) coordinates multiple analyzers
//	and aggregates their results into unified review reports.
//
//	Analyzer (analyzer.go)  →  Individual PR analysis
//	Orchestrator (review)   →  Multi-analyzer coordination
//
// Design Principles:
//
//   - Advisory-only: All review output is for human decision-making
//   - Read-only: Review system never modifies PR state
//   - Composable: Multiple analyzers can be combined for comprehensive review
//   - Observable: All review decisions are traceable to analyzer outputs
//
// This package is part of the agentic PR review system (v0.1 scope).
package review
