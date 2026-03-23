package planning

import (
	"context"
	"testing"
	"time"

	"github.com/jeffersonnunn/pratc/internal/types"
)

func TestShardConfig_Validate(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		config := ShardConfig{
			MaxWorkers:         4,
			ShardSize:          256,
			EarlyExitThreshold: 100,
			BackpressureSize:   100,
		}
		if err := config.Validate(); err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("max workers zero", func(t *testing.T) {
		config := ShardConfig{
			MaxWorkers:         0,
			ShardSize:          256,
			EarlyExitThreshold: 100,
			BackpressureSize:   100,
		}
		if err := config.Validate(); err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("shard size zero", func(t *testing.T) {
		config := ShardConfig{
			MaxWorkers:         4,
			ShardSize:          0,
			EarlyExitThreshold: 100,
			BackpressureSize:   100,
		}
		if err := config.Validate(); err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("early exit threshold negative", func(t *testing.T) {
		config := ShardConfig{
			MaxWorkers:         4,
			ShardSize:          256,
			EarlyExitThreshold: -1,
			BackpressureSize:   100,
		}
		if err := config.Validate(); err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("backpressure size zero", func(t *testing.T) {
		config := ShardConfig{
			MaxWorkers:         4,
			ShardSize:          256,
			EarlyExitThreshold: 100,
			BackpressureSize:   0,
		}
		if err := config.Validate(); err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestDefaultShardConfig(t *testing.T) {
	config := DefaultShardConfig()
	if config.MaxWorkers < 1 {
		t.Error("expected MaxWorkers >= 1")
	}
	if config.ShardSize != 256 {
		t.Errorf("expected ShardSize 256, got %d", config.ShardSize)
	}
	if config.EarlyExitThreshold != 100 {
		t.Errorf("expected EarlyExitThreshold 100, got %d", config.EarlyExitThreshold)
	}
	if config.BackpressureSize != 100 {
		t.Errorf("expected BackpressureSize 100, got %d", config.BackpressureSize)
	}
}

func TestNewPairwiseExecutor(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		config := ShardConfig{
			MaxWorkers:         4,
			ShardSize:          256,
			EarlyExitThreshold: 100,
			BackpressureSize:   100,
		}
		executor, err := NewPairwiseExecutor(config)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if executor == nil {
			t.Fatal("expected non-nil executor")
		}
	})

	t.Run("invalid config", func(t *testing.T) {
		config := ShardConfig{
			MaxWorkers: 0,
		}
		executor, err := NewPairwiseExecutor(config)
		if err == nil {
			t.Error("expected error, got nil")
		}
		if executor != nil {
			t.Error("expected nil executor on error")
		}
	})
}

func TestNewPairwiseExecutorWithDefaults(t *testing.T) {
	executor, err := NewPairwiseExecutorWithDefaults()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if executor == nil {
		t.Fatal("expected non-nil executor")
	}
}

func TestPairwiseExecutor_ExecuteSharded(t *testing.T) {
	t.Run("empty PR list", func(t *testing.T) {
		executor, _ := NewPairwiseExecutorWithDefaults()
		ctx := context.Background()
		prs := []types.PR{}

		result, err := executor.ExecuteSharded(ctx, "test/repo", prs)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result == nil {
			t.Fatal("expected non-nil result")
		}
		if result.PoolSize != 0 {
			t.Errorf("expected PoolSize 0, got %d", result.PoolSize)
		}
		if len(result.Conflicts) != 0 {
			t.Errorf("expected 0 conflicts, got %d", len(result.Conflicts))
		}
	})

	t.Run("single PR", func(t *testing.T) {
		executor, _ := NewPairwiseExecutorWithDefaults()
		ctx := context.Background()
		prs := []types.PR{
			{
				Number:       1,
				Title:        "PR 1",
				BaseBranch:   "main",
				HeadBranch:   "feature1",
				FilesChanged: []string{"file1.go"},
			},
		}

		result, err := executor.ExecuteSharded(ctx, "test/repo", prs)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.PoolSize != 1 {
			t.Errorf("expected PoolSize 1, got %d", result.PoolSize)
		}
		if len(result.Conflicts) != 0 {
			t.Errorf("expected 0 conflicts, got %d", len(result.Conflicts))
		}
	})

	t.Run("two PRs with file conflict", func(t *testing.T) {
		executor, _ := NewPairwiseExecutorWithDefaults()
		ctx := context.Background()
		prs := []types.PR{
			{
				Number:       1,
				Title:        "PR 1",
				BaseBranch:   "main",
				HeadBranch:   "feature1",
				FilesChanged: []string{"file1.go", "file2.go"},
			},
			{
				Number:       2,
				Title:        "PR 2",
				BaseBranch:   "main",
				HeadBranch:   "feature2",
				FilesChanged: []string{"file2.go", "file3.go"},
			},
		}

		result, err := executor.ExecuteSharded(ctx, "test/repo", prs)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result.Conflicts) != 1 {
			t.Errorf("expected 1 conflict, got %d", len(result.Conflicts))
		} else {
			conflict := result.Conflicts[0]
			if conflict.SourcePR != 1 {
				t.Errorf("expected SourcePR 1, got %d", conflict.SourcePR)
			}
			if conflict.TargetPR != 2 {
				t.Errorf("expected TargetPR 2, got %d", conflict.TargetPR)
			}
			if conflict.ConflictType != "file_overlap" {
				t.Errorf("expected ConflictType file_overlap, got %s", conflict.ConflictType)
			}
		}
	})

	t.Run("PRs with different base branches", func(t *testing.T) {
		executor, _ := NewPairwiseExecutorWithDefaults()
		ctx := context.Background()
		prs := []types.PR{
			{
				Number:       1,
				BaseBranch:   "main",
				FilesChanged: []string{"file1.go"},
			},
			{
				Number:       2,
				BaseBranch:   "develop",
				FilesChanged: []string{"file1.go"},
			},
		}

		result, err := executor.ExecuteSharded(ctx, "test/repo", prs)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result.Conflicts) != 0 {
			t.Errorf("expected 0 conflicts (different base branches), got %d", len(result.Conflicts))
		}
	})

	t.Run("PRs with mergeability signal conflict", func(t *testing.T) {
		executor, _ := NewPairwiseExecutorWithDefaults()
		ctx := context.Background()
		prs := []types.PR{
			{
				Number:       1,
				BaseBranch:   "main",
				Mergeable:    "conflicting",
				FilesChanged: []string{},
			},
			{
				Number:       2,
				BaseBranch:   "main",
				Mergeable:    "conflicting",
				FilesChanged: []string{},
			},
		}

		result, err := executor.ExecuteSharded(ctx, "test/repo", prs)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result.Conflicts) != 1 {
			t.Errorf("expected 1 conflict (mergeability signal), got %d", len(result.Conflicts))
		} else {
			conflict := result.Conflicts[0]
			if conflict.ConflictType != "mergeability_signal" {
				t.Errorf("expected mergeability_signal, got %s", conflict.ConflictType)
			}
		}
	})

	t.Run("deterministic ordering", func(t *testing.T) {
		executor, _ := NewPairwiseExecutorWithDefaults()
		ctx := context.Background()
		prs := []types.PR{
			{Number: 5, BaseBranch: "main", FilesChanged: []string{"a.go"}},
			{Number: 2, BaseBranch: "main", FilesChanged: []string{"a.go"}},
			{Number: 8, BaseBranch: "main", FilesChanged: []string{"a.go"}},
		}

		result1, _ := executor.ExecuteSharded(ctx, "test/repo", prs)
		result2, _ := executor.ExecuteSharded(ctx, "test/repo", prs)

		if len(result1.Conflicts) != len(result2.Conflicts) {
			t.Fatal("conflict count mismatch")
		}
		for i := range result1.Conflicts {
			if result1.Conflicts[i].SourcePR != result2.Conflicts[i].SourcePR {
				t.Errorf("conflict %d: SourcePR mismatch", i)
			}
			if result1.Conflicts[i].TargetPR != result2.Conflicts[i].TargetPR {
				t.Errorf("conflict %d: TargetPR mismatch", i)
			}
		}
	})

	t.Run("telemetry populated", func(t *testing.T) {
		executor, _ := NewPairwiseExecutorWithDefaults()
		ctx := context.Background()
		prs := []types.PR{
			{Number: 1, BaseBranch: "main", FilesChanged: []string{"a.go"}},
			{Number: 2, BaseBranch: "main", FilesChanged: []string{"a.go"}},
		}

		result, _ := executor.ExecuteSharded(ctx, "test/repo", prs)

		if result.Telemetry.PoolStrategy != "parallel_pairwise_sharded" {
			t.Errorf("unexpected PoolStrategy: %s", result.Telemetry.PoolStrategy)
		}
		if result.Telemetry.PairwiseShards <= 0 {
			t.Errorf("expected PairwiseShards > 0, got %d", result.Telemetry.PairwiseShards)
		}
		if latency, ok := result.Telemetry.StageLatenciesMS["pairwise_total_ms"]; !ok || latency < 0 {
			t.Error("expected pairwise_total_ms in latencies")
		}
	})

	t.Run("sharded processing metrics", func(t *testing.T) {
		executor, _ := NewPairwiseExecutorWithDefaults()
		ctx := context.Background()
		prs := make([]types.PR, 100)
		for i := 0; i < 100; i++ {
			prs[i] = types.PR{
				Number:       i + 1,
				BaseBranch:   "main",
				FilesChanged: []string{"file.go"},
			}
		}

		result, err := executor.ExecuteSharded(ctx, "test/repo", prs)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.ShardsTotal <= 0 {
			t.Error("expected ShardsTotal > 0")
		}
		if result.ShardsCompleted != result.ShardsTotal {
			t.Errorf("expected ShardsCompleted %d, got %d", result.ShardsTotal, result.ShardsCompleted)
		}
		if result.WorkersActive != executor.config.MaxWorkers {
			t.Errorf("expected WorkersActive %d, got %d", executor.config.MaxWorkers, result.WorkersActive)
		}
	})
}

func TestPairwiseExecutor_ExecuteWithEarlyExit(t *testing.T) {
	t.Run("empty PR list", func(t *testing.T) {
		executor, _ := NewPairwiseExecutorWithDefaults()
		ctx := context.Background()
		prs := []types.PR{}

		result, err := executor.ExecuteWithEarlyExit(ctx, "test/repo", prs, 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.PoolSize != 0 {
			t.Errorf("expected PoolSize 0, got %d", result.PoolSize)
		}
	})

	t.Run("early exit on threshold", func(t *testing.T) {
		executor, _ := NewPairwiseExecutorWithDefaults()
		ctx := context.Background()

		prs := make([]types.PR, 50)
		for i := 0; i < 50; i++ {
			prs[i] = types.PR{
				Number:       i + 1,
				BaseBranch:   "main",
				FilesChanged: []string{"file.go"},
			}
		}

		result, err := executor.ExecuteWithEarlyExit(ctx, "test/repo", prs, 5)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.EarlyExits != 1 {
			t.Errorf("expected EarlyExits 1, got %d", result.EarlyExits)
		}
	})

	t.Run("no early exit when threshold not reached", func(t *testing.T) {
		executor, _ := NewPairwiseExecutorWithDefaults()
		ctx := context.Background()
		prs := []types.PR{
			{Number: 1, BaseBranch: "main", FilesChanged: []string{"a.go"}},
			{Number: 2, BaseBranch: "main", FilesChanged: []string{"b.go"}},
		}

		result, err := executor.ExecuteWithEarlyExit(ctx, "test/repo", prs, 100)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.EarlyExits != 0 {
			t.Errorf("expected EarlyExits 0, got %d", result.EarlyExits)
		}
	})

	t.Run("threshold zero disables early exit", func(t *testing.T) {
		executor, _ := NewPairwiseExecutorWithDefaults()
		ctx := context.Background()
		prs := make([]types.PR, 20)
		for i := 0; i < 20; i++ {
			prs[i] = types.PR{
				Number:       i + 1,
				BaseBranch:   "main",
				FilesChanged: []string{"file.go"},
			}
		}

		result, err := executor.ExecuteWithEarlyExit(ctx, "test/repo", prs, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.EarlyExits != 0 {
			t.Errorf("expected EarlyExits 0 with threshold=0, got %d", result.EarlyExits)
		}
	})
}

func TestPairwiseExecutor_Cancellation(t *testing.T) {
	t.Run("context cancellation during execution", func(t *testing.T) {
		executor, _ := NewPairwiseExecutorWithDefaults()
		ctx, cancel := context.WithCancel(context.Background())

		prs := make([]types.PR, 100)
		for i := 0; i < 100; i++ {
			prs[i] = types.PR{
				Number:       i + 1,
				BaseBranch:   "main",
				FilesChanged: []string{"file.go"},
			}
		}

		done := make(chan struct{})
		go func() {
			executor.ExecuteSharded(ctx, "test/repo", prs)
			close(done)
		}()

		time.Sleep(10 * time.Millisecond)
		cancel()

		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("execution did not complete after cancellation")
		}
	})
}

func TestPairwiseExecutor_BoundedWorkers(t *testing.T) {
	t.Run("respects MaxWorkers limit", func(t *testing.T) {
		config := ShardConfig{
			MaxWorkers:         2,
			ShardSize:          50,
			EarlyExitThreshold: 0,
			BackpressureSize:   10,
		}
		executor, _ := NewPairwiseExecutor(config)
		ctx := context.Background()

		prs := make([]types.PR, 200)
		for i := 0; i < 200; i++ {
			prs[i] = types.PR{
				Number:       i + 1,
				BaseBranch:   "main",
				FilesChanged: []string{"file.go"},
			}
		}

		result, err := executor.ExecuteSharded(ctx, "test/repo", prs)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.WorkersActive != 2 {
			t.Errorf("expected WorkersActive 2, got %d", result.WorkersActive)
		}
	})
}

func TestShardProcessor_ProcessShard(t *testing.T) {
	t.Run("valid shard processing", func(t *testing.T) {
		executor, _ := NewPairwiseExecutorWithDefaults()
		ctx := context.Background()
		prs := []types.PR{
			{Number: 1, BaseBranch: "main", FilesChanged: []string{"a.go"}},
			{Number: 2, BaseBranch: "main", FilesChanged: []string{"a.go"}},
		}

		processor := executor.GetShardProcessor(ctx, prs)
		conflictChan := make(chan types.ConflictPair, 10)

		metrics, err := processor.ProcessShard(0, 0, 2, conflictChan)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if metrics.ShardID != 0 {
			t.Errorf("expected ShardID 0, got %d", metrics.ShardID)
		}
		if metrics.ConflictsFound != 1 {
			t.Errorf("expected 1 conflict, got %d", metrics.ConflictsFound)
		}
		if metrics.Reason != "shard_processed" {
			t.Errorf("expected reason shard_processed, got %s", metrics.Reason)
		}

		close(conflictChan)
		conflicts := make([]types.ConflictPair, 0)
		for c := range conflictChan {
			conflicts = append(conflicts, c)
		}
		if len(conflicts) != 1 {
			t.Errorf("expected 1 conflict in channel, got %d", len(conflicts))
		}
	})

	t.Run("invalid bounds", func(t *testing.T) {
		executor, _ := NewPairwiseExecutorWithDefaults()
		ctx := context.Background()
		prs := []types.PR{{Number: 1}}

		processor := executor.GetShardProcessor(ctx, prs)
		conflictChan := make(chan types.ConflictPair, 10)

		_, err := processor.ProcessShard(0, 5, 10, conflictChan)
		if err == nil {
			t.Error("expected error for invalid bounds")
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		executor, _ := NewPairwiseExecutorWithDefaults()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		prs := make([]types.PR, 10)
		for i := 0; i < 10; i++ {
			prs[i] = types.PR{Number: i + 1, BaseBranch: "main", FilesChanged: []string{"file.go"}}
		}

		processor := executor.GetShardProcessor(ctx, prs)
		conflictChan := make(chan types.ConflictPair, 10)

		metrics, err := processor.ProcessShard(0, 0, 10, conflictChan)
		if err == nil {
			t.Error("expected error due to cancelled context")
		}
		if metrics.Reason != "cancelled" {
			t.Errorf("expected reason cancelled, got %s", metrics.Reason)
		}
	})
}

func TestIntersectFilesShared(t *testing.T) {
	t.Run("no shared files", func(t *testing.T) {
		left := []string{"a.go", "b.go"}
		right := []string{"c.go", "d.go"}
		result := intersectFilesShared(left, right)
		if len(result) != 0 {
			t.Errorf("expected empty slice, got %v", result)
		}
	})

	t.Run("shared files", func(t *testing.T) {
		left := []string{"a.go", "b.go", "c.go"}
		right := []string{"b.go", "c.go", "d.go"}
		result := intersectFilesShared(left, right)
		if len(result) != 2 {
			t.Fatalf("expected 2 shared files, got %d", len(result))
		}
		if result[0] != "b.go" || result[1] != "c.go" {
			t.Errorf("unexpected shared files: %v", result)
		}
	})

	t.Run("empty left", func(t *testing.T) {
		left := []string{}
		right := []string{"a.go"}
		result := intersectFilesShared(left, right)
		if result != nil {
			t.Errorf("expected nil, got %v", result)
		}
	})

	t.Run("empty right", func(t *testing.T) {
		left := []string{"a.go"}
		right := []string{}
		result := intersectFilesShared(left, right)
		if result != nil {
			t.Errorf("expected nil, got %v", result)
		}
	})

	t.Run("deduplication", func(t *testing.T) {
		left := []string{"a.go", "a.go", "b.go"}
		right := []string{"a.go", "b.go", "b.go"}
		result := intersectFilesShared(left, right)
		if len(result) != 2 {
			t.Errorf("expected 2 unique shared files, got %d: %v", len(result), result)
		}
	})
}

func TestJoinStrings(t *testing.T) {
	t.Run("empty slice", func(t *testing.T) {
		result := joinStrings([]string{}, ", ")
		if result != "" {
			t.Errorf("expected empty string, got %q", result)
		}
	})

	t.Run("single element", func(t *testing.T) {
		result := joinStrings([]string{"a"}, ", ")
		if result != "a" {
			t.Errorf("expected \"a\", got %q", result)
		}
	})

	t.Run("multiple elements", func(t *testing.T) {
		result := joinStrings([]string{"a", "b", "c"}, ", ")
		if result != "a, b, c" {
			t.Errorf("expected \"a, b, c\", got %q", result)
		}
	})
}

func TestPairwiseResult_JSON(t *testing.T) {
	result := PairwiseResult{
		Repo:            "test/repo",
		PoolSize:        100,
		TotalPairs:      4950,
		ShardsTotal:     10,
		ShardsCompleted: 10,
		EarlyExits:      2,
		WorkersActive:   4,
		PairsProcessed:  4950,
	}

	if result.Repo != "test/repo" {
		t.Error("Repo field mismatch")
	}
	if result.TotalPairs != 4950 {
		t.Error("TotalPairs field mismatch")
	}
}

func BenchmarkPairwiseExecutor_ExecuteSharded(b *testing.B) {
	executor, _ := NewPairwiseExecutorWithDefaults()
	ctx := context.Background()

	prs := make([]types.PR, 500)
	for i := 0; i < 500; i++ {
		prs[i] = types.PR{
			Number:       i + 1,
			BaseBranch:   "main",
			FilesChanged: []string{"file.go"},
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		executor.ExecuteSharded(ctx, "test/repo", prs)
	}
}

func BenchmarkPairwiseExecutor_ExecuteWithEarlyExit(b *testing.B) {
	executor, _ := NewPairwiseExecutorWithDefaults()
	ctx := context.Background()

	prs := make([]types.PR, 500)
	for i := 0; i < 500; i++ {
		prs[i] = types.PR{
			Number:       i + 1,
			BaseBranch:   "main",
			FilesChanged: []string{"file.go"},
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		executor.ExecuteWithEarlyExit(ctx, "test/repo", prs, 50)
	}
}
