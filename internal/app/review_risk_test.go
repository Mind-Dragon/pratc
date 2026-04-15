package app

import (
	"testing"

	"github.com/jeffersonnunn/pratc/internal/types"
)

func TestBuildRiskBucketsCountsUniquePRsByAnalyzerCategory(t *testing.T) {
	results := []types.ReviewResult{
		{
			PRNumber: 1,
			AnalyzerFindings: []types.AnalyzerFinding{
				{AnalyzerName: "security"},
				{AnalyzerName: "security"},
				{AnalyzerName: "reliability"},
			},
		},
		{
			PRNumber: 2,
			AnalyzerFindings: []types.AnalyzerFinding{
				{AnalyzerName: "performance"},
			},
		},
		{
			PRNumber: 3,
			AnalyzerFindings: []types.AnalyzerFinding{
				{AnalyzerName: "security"},
				{AnalyzerName: "performance"},
			},
		},
	}

	buckets := buildRiskBuckets(results)
	if got := bucketCount(buckets, "security_risk"); got != 2 {
		t.Fatalf("security_risk = %d, want 2", got)
	}
	if got := bucketCount(buckets, "reliability_risk"); got != 1 {
		t.Fatalf("reliability_risk = %d, want 1", got)
	}
	if got := bucketCount(buckets, "performance_risk"); got != 2 {
		t.Fatalf("performance_risk = %d, want 2", got)
	}
}

func bucketCount(buckets []types.BucketCount, want string) int {
	for _, bucket := range buckets {
		if bucket.Bucket == want {
			return bucket.Count
		}
	}
	return 0
}
