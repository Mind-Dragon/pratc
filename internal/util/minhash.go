package util

import (
	"encoding/binary"
	"hash/fnv"
	"math"
	"sort"
)

const defaultMinHashSeed = uint64(0x9e3779b97f4a7c15)

// MinHashSignature computes a deterministic MinHash signature for a token set.
// Duplicate or empty tokens are ignored.
func MinHashSignature(tokens []string, numPerm int) []uint64 {
	if numPerm <= 0 {
		return nil
	}
	signature := make([]uint64, numPerm)
	for i := range signature {
		signature[i] = math.MaxUint64
	}
	seen := make(map[string]struct{}, len(tokens))
	for _, token := range tokens {
		if token == "" {
			continue
		}
		if _, ok := seen[token]; ok {
			continue
		}
		seen[token] = struct{}{}
		base := hashToken64(token)
		for i := 0; i < numPerm; i++ {
			candidate := mix64(base ^ permutationSalt(i))
			if candidate < signature[i] {
				signature[i] = candidate
			}
		}
	}
	for i := range signature {
		if signature[i] == math.MaxUint64 {
			signature[i] = 0
		}
	}
	return signature
}

// MinHashCandidatePairs returns ordered unique candidate pairs using LSH banding.
func MinHashCandidatePairs(signatures [][]uint64, bands int) [][2]int {
	if len(signatures) < 2 || bands <= 0 {
		return nil
	}
	sigLen := len(signatures[0])
	if sigLen == 0 || sigLen%bands != 0 {
		return nil
	}
	rowsPerBand := sigLen / bands
	buckets := make(map[[2]uint64][]int, len(signatures)*bands)
	pairs := make(map[[2]int]struct{})
	for idx, sig := range signatures {
		if len(sig) != sigLen {
			continue
		}
		for band := 0; band < bands; band++ {
			start := band * rowsPerBand
			end := start + rowsPerBand
			key := [2]uint64{uint64(band), hashUint64Slice(sig[start:end])}
			for _, other := range buckets[key] {
				pair := orderedPair(other, idx)
				if pair[0] != pair[1] {
					pairs[pair] = struct{}{}
				}
			}
			buckets[key] = append(buckets[key], idx)
		}
	}
	if len(signatures) <= 256 {
		for i := 0; i < len(signatures); i++ {
			for j := i + 1; j < len(signatures); j++ {
				if len(signatures[i]) != sigLen || len(signatures[j]) != sigLen {
					continue
				}
				if minHashEstimate(signatures[i], signatures[j]) >= 0.5 {
					pairs[[2]int{i, j}] = struct{}{}
				}
			}
		}
	}
	out := make([][2]int, 0, len(pairs))
	for pair := range pairs {
		out = append(out, pair)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i][0] == out[j][0] {
			return out[i][1] < out[j][1]
		}
		return out[i][0] < out[j][0]
	})
	return out
}

func orderedPair(left, right int) [2]int {
	if left < right {
		return [2]int{left, right}
	}
	return [2]int{right, left}
}

func hashToken64(token string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(token))
	return h.Sum64()
}

func permutationSalt(i int) uint64 {
	return mix64(defaultMinHashSeed + uint64(i+1)*0x517cc1b727220a95)
}

func hashUint64Slice(values []uint64) uint64 {
	h := fnv.New64a()
	var buf [8]byte
	for _, value := range values {
		binary.LittleEndian.PutUint64(buf[:], value)
		_, _ = h.Write(buf[:])
	}
	return h.Sum64()
}

func minHashEstimate(left, right []uint64) float64 {
	if len(left) == 0 || len(right) == 0 || len(left) != len(right) {
		return 0
	}
	matches := 0
	for i := range left {
		if left[i] == right[i] {
			matches++
		}
	}
	return float64(matches) / float64(len(left))
}

func mix64(x uint64) uint64 {
	x += 0x9e3779b97f4a7c15
	x = (x ^ (x >> 30)) * 0xbf58476d1ce4e5b9
	x = (x ^ (x >> 27)) * 0x94d049bb133111eb
	return x ^ (x >> 31)
}
