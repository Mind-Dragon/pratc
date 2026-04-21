package util

import "testing"

func TestMinHashSignatureDeterministic(t *testing.T) {
	t.Parallel()

	tokens := []string{"auth", "oauth", "login"}
	left := MinHashSignature(tokens, 32)
	right := MinHashSignature(tokens, 32)
	if len(left) != 32 || len(right) != 32 {
		t.Fatalf("signature lengths = %d and %d, want 32", len(left), len(right))
	}
	for i := range left {
		if left[i] != right[i] {
			t.Fatalf("signature mismatch at %d: %d != %d", i, left[i], right[i])
		}
	}
}

func TestMinHashCandidatePairsFindsSimilarSets(t *testing.T) {
	t.Parallel()

	signatures := [][]uint64{
		MinHashSignature([]string{"auth", "oauth", "login", "session", "token", "refresh"}, 64),
		MinHashSignature([]string{"auth", "oauth", "login", "session", "token", "cookie"}, 64),
		MinHashSignature([]string{"cache", "eviction", "memory"}, 64),
	}
	pairs := MinHashCandidatePairs(signatures, 2)
	if len(pairs) == 0 {
		t.Fatal("expected at least one candidate pair")
	}
	found := false
	for _, pair := range pairs {
		if pair[0] == 0 && pair[1] == 1 {
			found = true
		}
		if pair[0] >= pair[1] {
			t.Fatalf("pairs must be ordered and unique, got %v", pair)
		}
	}
	if !found {
		t.Fatalf("expected similar pair (0,1) in candidates, got %v", pairs)
	}
}

func TestMinHashCandidatePairsNoSelfPairs(t *testing.T) {
	t.Parallel()

	signatures := [][]uint64{
		MinHashSignature([]string{"a", "b", "c"}, 32),
		MinHashSignature([]string{"a", "b", "d"}, 32),
	}
	pairs := MinHashCandidatePairs(signatures, 4)
	for _, pair := range pairs {
		if pair[0] == pair[1] {
			t.Fatalf("unexpected self pair: %v", pair)
		}
	}
}
