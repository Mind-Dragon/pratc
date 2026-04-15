package review

import (
	"testing"

	"github.com/jeffersonnunn/pratc/internal/types"
)

func TestClassifyProblematicPR_FlagsAutomatedDependencySpam(t *testing.T) {
	pr := types.PR{
		Number: 101,
		Title:  "Bump github.com/foo/bar from 1.0.0 to 1.0.1",
		Body:   "",
		Author: "dependabot[bot]",
		IsBot:  true,
		Labels: []string{"deps"},
	}

	result := ClassifyProblematicPR(pr)
	if !result.IsProblematic {
		t.Fatal("expected PR to be classified as problematic")
	}
	if result.ProblemType != "spam" {
		t.Fatalf("problem type = %q, want spam", result.ProblemType)
	}
	if len(result.Reasons) == 0 {
		t.Fatal("expected spam reasons")
	}
}

func TestClassifyProblematicPR_FlagsPromotionalOrPlaceholderNoise(t *testing.T) {
	pr := types.PR{
		Number: 102,
		Title:  "FREE MONEY CLICK NOW!!!",
		Body:   "lorem ipsum",
		Author: "random-user",
		Labels: []string{"x"},
	}

	result := ClassifyProblematicPR(pr)
	if !result.IsProblematic {
		t.Fatal("expected noisy PR to be classified as problematic")
	}
	if result.ProblemType != "spam" && result.ProblemType != "low_quality" {
		t.Fatalf("problem type = %q, want spam or low_quality", result.ProblemType)
	}
}
