package formula

import (
	"errors"
	"math/big"

	"github.com/jeffersonnunn/pratc/internal/types"
)

var (
	ErrIndexOutOfRange  = errors.New("formula index out of range")
	ErrInvalidSelection = errors.New("invalid formula selection")
	ErrUnsupportedMode  = errors.New("unsupported formula mode")
)

func Count(mode Mode, n, k int) *big.Int {
	if n < 0 || k < 0 {
		return big.NewInt(0)
	}

	switch mode {
	case ModePermutation:
		if k > n {
			return big.NewInt(0)
		}
		result := big.NewInt(1)
		for current := 0; current < k; current++ {
			result.Mul(result, big.NewInt(int64(n-current)))
		}
		return result
	case ModeCombination:
		if k > n {
			return big.NewInt(0)
		}
		if k == 0 || k == n {
			return big.NewInt(1)
		}
		if k > n-k {
			k = n - k
		}

		result := big.NewInt(1)
		for step := 1; step <= k; step++ {
			result.Mul(result, big.NewInt(int64(n-k+step)))
			result.Div(result, big.NewInt(int64(step)))
		}
		return result
	case ModeWithReplacement:
		if k == 0 {
			return big.NewInt(1)
		}
		if n == 0 {
			return big.NewInt(0)
		}
		return new(big.Int).Exp(big.NewInt(int64(n)), big.NewInt(int64(k)), nil)
	default:
		return big.NewInt(0)
	}
}

func GenerateByIndex(mode Mode, pool []types.PR, k int, idx *big.Int) ([]types.PR, error) {
	n := len(pool)
	if idx == nil || idx.Sign() < 0 {
		return nil, ErrIndexOutOfRange
	}
	if k < 0 {
		return nil, ErrInvalidSelection
	}

	total := Count(mode, n, k)
	if total.Sign() == 0 || idx.Cmp(total) >= 0 {
		return nil, ErrIndexOutOfRange
	}

	var indices []int
	switch mode {
	case ModePermutation:
		var err error
		indices, err = permutationIndices(n, k, idx)
		if err != nil {
			return nil, err
		}
	case ModeCombination:
		var err error
		indices, err = combinationIndices(n, k, idx)
		if err != nil {
			return nil, err
		}
	case ModeWithReplacement:
		var err error
		indices, err = replacementIndices(n, k, idx)
		if err != nil {
			return nil, err
		}
	default:
		return nil, ErrUnsupportedMode
	}

	selection := make([]types.PR, 0, len(indices))
	for _, index := range indices {
		selection = append(selection, pool[index])
	}

	return selection, nil
}

func permutationIndices(n, k int, idx *big.Int) ([]int, error) {
	if k > n {
		return nil, ErrInvalidSelection
	}

	available := make([]int, n)
	for i := range available {
		available[i] = i
	}

	remaining := new(big.Int).Set(idx)
	indices := make([]int, 0, k)

	for pos := 0; pos < k; pos++ {
		block := Count(ModePermutation, n-pos-1, k-pos-1)
		if block.Sign() == 0 {
			block = big.NewInt(1)
		}

		choice := new(big.Int)
		offset := new(big.Int)
		choice.QuoRem(remaining, block, offset)
		if !choice.IsInt64() {
			return nil, ErrIndexOutOfRange
		}

		choiceIndex := int(choice.Int64())
		if choiceIndex < 0 || choiceIndex >= len(available) {
			return nil, ErrIndexOutOfRange
		}

		indices = append(indices, available[choiceIndex])
		available = append(available[:choiceIndex], available[choiceIndex+1:]...)
		remaining = offset
	}

	return indices, nil
}

func combinationIndices(n, k int, idx *big.Int) ([]int, error) {
	if k > n {
		return nil, ErrInvalidSelection
	}

	remaining := new(big.Int).Set(idx)
	indices := make([]int, 0, k)
	start := 0

	for pos := 0; pos < k; pos++ {
		found := false
		limit := n - (k - pos)
		for candidate := start; candidate <= limit; candidate++ {
			block := Count(ModeCombination, n-candidate-1, k-pos-1)
			if remaining.Cmp(block) < 0 {
				indices = append(indices, candidate)
				start = candidate + 1
				found = true
				break
			}

			remaining.Sub(remaining, block)
		}

		if !found {
			return nil, ErrIndexOutOfRange
		}
	}

	return indices, nil
}

func replacementIndices(n, k int, idx *big.Int) ([]int, error) {
	if n == 0 && k > 0 {
		return nil, ErrInvalidSelection
	}

	remaining := new(big.Int).Set(idx)
	base := big.NewInt(int64(n))
	indices := make([]int, k)

	for pos := k - 1; pos >= 0; pos-- {
		quotient := new(big.Int)
		remainder := new(big.Int)
		quotient.QuoRem(remaining, base, remainder)
		if !remainder.IsInt64() {
			return nil, ErrIndexOutOfRange
		}
		indices[pos] = int(remainder.Int64())
		remaining = quotient
	}

	return indices, nil
}
