package planning

import (
	"context"
	"fmt"
	"math"
	"runtime"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jeffersonnunn/pratc/internal/types"
)

// DEPRECATED: PairwiseExecutor is not wired into the production planning path.
// Production uses internal/filter + internal/planner instead.
// Scheduled for removal in v0.2.
// See: internal/AGENTS.md "planning/ is mostly dead code"
type PairwiseResult struct {
	Repo            string                   `json:"repo"`
	GeneratedAt     string                   `json:"generated_at"`
	PoolSize        int                      `json:"pool_size"`
	TotalPairs      int64                    `json:"total_pairs"`
	Conflicts       []types.ConflictPair     `json:"conflicts"`
	ShardsTotal     int                      `json:"shards_total"`
	ShardsCompleted int                      `json:"shards_completed"`
	EarlyExits      int                      `json:"early_exits"`
	WorkersActive   int                      `json:"workers_active"`
	PairsProcessed  int64                    `json:"pairs_processed"`
	Telemetry       types.OperationTelemetry `json:"telemetry"`
}

type ShardMetrics struct {
	ShardID        int    `json:"shard_id"`
	PairsChecked   int64  `json:"pairs_checked"`
	ConflictsFound int64  `json:"conflicts_found"`
	EarlyExits     int64  `json:"early_exits"`
	DurationMS     int64  `json:"duration_ms"`
	Reason         string `json:"reason"`
}

type ShardConfig struct {
	MaxWorkers         int
	ShardSize          int
	EarlyExitThreshold int
	BackpressureSize   int
}

func DefaultShardConfig() ShardConfig {
	return ShardConfig{
		MaxWorkers:         max(1, runtime.NumCPU()),
		ShardSize:          256,
		EarlyExitThreshold: 100,
		BackpressureSize:   100,
	}
}

func (c ShardConfig) Validate() error {
	if c.MaxWorkers < 1 {
		return fmt.Errorf("max_workers must be >= 1")
	}
	if c.ShardSize < 1 {
		return fmt.Errorf("shard_size must be >= 1")
	}
	if c.EarlyExitThreshold < 0 {
		return fmt.Errorf("early_exit_threshold must be >= 0")
	}
	if c.BackpressureSize < 1 {
		return fmt.Errorf("backpressure_size must be >= 1")
	}
	return nil
}

type PairwiseExecutor struct {
	config ShardConfig
	sem    chan struct{}
}

func NewPairwiseExecutor(config ShardConfig) (*PairwiseExecutor, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	return &PairwiseExecutor{
		config: config,
		sem:    make(chan struct{}, config.MaxWorkers),
	}, nil
}

func NewPairwiseExecutorWithDefaults() (*PairwiseExecutor, error) {
	return NewPairwiseExecutor(DefaultShardConfig())
}

type shardJob struct {
	shardID            int
	startI             int
	endI               int
	startJ             int
	endJ               int
	prs                []types.PR
	conflictChan       chan<- types.ConflictPair
	metricsChan        chan<- ShardMetrics
	ctx                context.Context
	earlyExitThreshold int
}

func (e *PairwiseExecutor) ExecuteSharded(ctx context.Context, repo string, prs []types.PR) (*PairwiseResult, error) {
	execStart := time.Now()

	sortedPRs := slices.Clone(prs)
	sort.Slice(sortedPRs, func(i, j int) bool {
		return sortedPRs[i].Number < sortedPRs[j].Number
	})

	n := len(sortedPRs)
	if n == 0 {
		return &PairwiseResult{
			Repo:        repo,
			GeneratedAt: time.Now().UTC().Format(time.RFC3339),
			Telemetry:   e.buildTelemetry(execStart, 0, 0, 0, 0, 0),
		}, nil
	}

	totalPairs := int64(n) * int64(n-1) / 2
	shardSize := e.config.ShardSize
	shardsTotal := int(math.Ceil(float64(n) / float64(shardSize)))

	conflictChan := make(chan types.ConflictPair, e.config.BackpressureSize)
	metricsChan := make(chan ShardMetrics, shardsTotal)
	errChan := make(chan error, shardsTotal)

	var wg sync.WaitGroup
	var pairsProcessed int64
	var earlyExits int32
	var shardsCompleted int32

	worker := func(job shardJob) {
		defer wg.Done()

		e.sem <- struct{}{}
		defer func() { <-e.sem }()

		shardStart := time.Now()
		var pairsChecked int64
		var conflictsFound int64
		var earlyExit bool

		for i := job.startI; i < job.endI && !earlyExit; i++ {
			jStart := max(i+1, job.startJ)
			for j := jStart; j < job.endJ && !earlyExit; j++ {
				select {
				case <-job.ctx.Done():
					metricsChan <- ShardMetrics{
						ShardID:    job.shardID,
						Reason:     "cancelled",
						DurationMS: time.Since(shardStart).Milliseconds(),
					}
					return
				default:
				}

				pairsChecked++
				left := job.prs[i]
				right := job.prs[j]

				if left.BaseBranch == "" || right.BaseBranch == "" || left.BaseBranch != right.BaseBranch {
					continue
				}

				sharedFiles := intersectFilesShared(left.FilesChanged, right.FilesChanged)

				if len(sharedFiles) == 0 && left.Mergeable != "conflicting" && right.Mergeable != "conflicting" {
					continue
				}

				conflictType := "file_overlap"
				reason := fmt.Sprintf("shared files: %s", joinStrings(sharedFiles, ", "))
				if len(sharedFiles) == 0 {
					conflictType = "mergeability_signal"
					reason = "both PRs signal merge conflicts"
				}

				conflict := types.ConflictPair{
					SourcePR:     left.Number,
					TargetPR:     right.Number,
					ConflictType: conflictType,
					FilesTouched: sharedFiles,
					Severity:     "high",
					Reason:       reason,
				}

				select {
				case job.conflictChan <- conflict:
					conflictsFound++
				case <-job.ctx.Done():
					metricsChan <- ShardMetrics{
						ShardID:    job.shardID,
						Reason:     "cancelled",
						DurationMS: time.Since(shardStart).Milliseconds(),
					}
					return
				}

				if job.earlyExitThreshold > 0 && int(conflictsFound) >= job.earlyExitThreshold {
					earlyExit = true
				}
			}
		}

		atomic.AddInt64(&pairsProcessed, pairsChecked)

		reason := "shard_processed"
		if earlyExit {
			reason = "early_exit_threshold"
			atomic.AddInt32(&earlyExits, 1)
		}

		atomic.AddInt32(&shardsCompleted, 1)

		metricsChan <- ShardMetrics{
			ShardID:        job.shardID,
			PairsChecked:   pairsChecked,
			ConflictsFound: conflictsFound,
			EarlyExits:     0,
			DurationMS:     time.Since(shardStart).Milliseconds(),
			Reason:         reason,
		}
	}

	jobs := make([]shardJob, 0, shardsTotal)
	shardID := 0

	for startI := 0; startI < n; startI += shardSize {
		endI := min(startI+shardSize, n)

		job := shardJob{
			shardID:            shardID,
			startI:             startI,
			endI:               endI,
			startJ:             startI + 1,
			endJ:               n,
			prs:                sortedPRs,
			conflictChan:       conflictChan,
			metricsChan:        metricsChan,
			ctx:                ctx,
			earlyExitThreshold: e.config.EarlyExitThreshold,
		}

		jobs = append(jobs, job)
		shardID++
	}

	for _, job := range jobs {
		wg.Add(1)
		go worker(job)
	}

	go func() {
		wg.Wait()
		close(conflictChan)
		close(metricsChan)
		close(errChan)
	}()

	conflicts := make([]types.ConflictPair, 0)
	for conflict := range conflictChan {
		conflicts = append(conflicts, conflict)
	}

	metrics := make([]ShardMetrics, 0)
	for metric := range metricsChan {
		metrics = append(metrics, metric)
	}

	sort.Slice(conflicts, func(i, j int) bool {
		if conflicts[i].SourcePR != conflicts[j].SourcePR {
			return conflicts[i].SourcePR < conflicts[j].SourcePR
		}
		return conflicts[i].TargetPR < conflicts[j].TargetPR
	})

	result := &PairwiseResult{
		Repo:            repo,
		GeneratedAt:     time.Now().UTC().Format(time.RFC3339),
		PoolSize:        n,
		TotalPairs:      totalPairs,
		Conflicts:       conflicts,
		ShardsTotal:     shardsTotal,
		ShardsCompleted: int(atomic.LoadInt32(&shardsCompleted)),
		EarlyExits:      int(atomic.LoadInt32(&earlyExits)),
		WorkersActive:   e.config.MaxWorkers,
		PairsProcessed:  atomic.LoadInt64(&pairsProcessed),
		Telemetry:       e.buildTelemetry(execStart, int64(len(metrics)), int64(atomic.LoadInt32(&earlyExits)), atomic.LoadInt64(&pairsProcessed), int64(len(conflicts)), 0),
	}

	return result, nil
}

func (e *PairwiseExecutor) ExecuteWithEarlyExit(ctx context.Context, repo string, prs []types.PR, globalThreshold int) (*PairwiseResult, error) {
	execStart := time.Now()

	sortedPRs := slices.Clone(prs)
	sort.Slice(sortedPRs, func(i, j int) bool {
		return sortedPRs[i].Number < sortedPRs[j].Number
	})

	n := len(sortedPRs)
	if n == 0 {
		return &PairwiseResult{
			Repo:        repo,
			GeneratedAt: time.Now().UTC().Format(time.RFC3339),
			Telemetry:   e.buildTelemetry(execStart, 0, 0, 0, 0, 0),
		}, nil
	}

	totalPairs := int64(n) * int64(n-1) / 2
	conflictChan := make(chan types.ConflictPair, e.config.BackpressureSize)
	doneChan := make(chan struct{})

	var conflictsFound int32
	var pairsProcessed int64
	var earlyExitTriggered int32

	var collectedConflicts []types.ConflictPair
	var mu sync.Mutex

	go func() {
		for conflict := range conflictChan {
			mu.Lock()
			collectedConflicts = append(collectedConflicts, conflict)
			mu.Unlock()

			if globalThreshold > 0 && atomic.LoadInt32(&conflictsFound) >= int32(globalThreshold) {
				atomic.StoreInt32(&earlyExitTriggered, 1)
			}
		}
		close(doneChan)
	}()

	var wg sync.WaitGroup
	sem := make(chan struct{}, e.config.MaxWorkers)
	chunkSize := e.config.ShardSize
	shardsTotal := 0
	var shardsCompleted int32

	for startI := 0; startI < n; startI += chunkSize {
		endI := min(startI+chunkSize, n)
		shardsTotal++

		if atomic.LoadInt32(&earlyExitTriggered) == 1 {
			break
		}

		wg.Add(1)
		sem <- struct{}{}

		go func(sI, eI int) {
			defer func() {
				<-sem
				wg.Done()
			}()

			var localPairs int64
			var localConflicts int64

			for i := sI; i < eI; i++ {
				if atomic.LoadInt32(&earlyExitTriggered) == 1 {
					return
				}

				for j := i + 1; j < n; j++ {
					select {
					case <-ctx.Done():
						return
					default:
					}

					localPairs++
					left := sortedPRs[i]
					right := sortedPRs[j]

					if left.BaseBranch == "" || right.BaseBranch == "" || left.BaseBranch != right.BaseBranch {
						continue
					}

					sharedFiles := intersectFilesShared(left.FilesChanged, right.FilesChanged)

					if len(sharedFiles) == 0 && left.Mergeable != "conflicting" && right.Mergeable != "conflicting" {
						continue
					}

					conflictType := "file_overlap"
					reason := fmt.Sprintf("shared files: %s", joinStrings(sharedFiles, ", "))
					if len(sharedFiles) == 0 {
						conflictType = "mergeability_signal"
						reason = "both PRs signal merge conflicts"
					}

					conflict := types.ConflictPair{
						SourcePR:     left.Number,
						TargetPR:     right.Number,
						ConflictType: conflictType,
						FilesTouched: sharedFiles,
						Severity:     "high",
						Reason:       reason,
					}

					select {
					case conflictChan <- conflict:
						localConflicts++
						atomic.AddInt32(&conflictsFound, 1)
					case <-ctx.Done():
						return
					}
				}
			}

			atomic.AddInt64(&pairsProcessed, localPairs)
			atomic.AddInt32(&shardsCompleted, 1)
		}(startI, endI)
	}

	wg.Wait()
	close(conflictChan)
	<-doneChan

	sort.Slice(collectedConflicts, func(i, j int) bool {
		if collectedConflicts[i].SourcePR != collectedConflicts[j].SourcePR {
			return collectedConflicts[i].SourcePR < collectedConflicts[j].SourcePR
		}
		return collectedConflicts[i].TargetPR < collectedConflicts[j].TargetPR
	})

	earlyExits := 0
	if atomic.LoadInt32(&earlyExitTriggered) == 1 {
		earlyExits = 1
	}

	result := &PairwiseResult{
		Repo:            repo,
		GeneratedAt:     time.Now().UTC().Format(time.RFC3339),
		PoolSize:        n,
		TotalPairs:      totalPairs,
		Conflicts:       collectedConflicts,
		ShardsTotal:     shardsTotal,
		ShardsCompleted: int(atomic.LoadInt32(&shardsCompleted)),
		EarlyExits:      earlyExits,
		WorkersActive:   e.config.MaxWorkers,
		PairsProcessed:  atomic.LoadInt64(&pairsProcessed),
		Telemetry:       e.buildTelemetry(execStart, int64(shardsTotal), int64(earlyExits), atomic.LoadInt64(&pairsProcessed), int64(len(collectedConflicts)), 0),
	}

	return result, nil
}

func (e *PairwiseExecutor) GetShardProcessor(ctx context.Context, prs []types.PR) *ShardProcessor {
	return &ShardProcessor{
		executor: e,
		prs:      prs,
		ctx:      ctx,
	}
}

type ShardProcessor struct {
	executor *PairwiseExecutor
	prs      []types.PR
	ctx      context.Context
}

func (p *ShardProcessor) ProcessShard(shardID int, startI, endI int, conflictChan chan<- types.ConflictPair) (ShardMetrics, error) {
	shardStart := time.Now()
	n := len(p.prs)

	if startI < 0 || endI > n || startI >= endI {
		return ShardMetrics{
			ShardID:    shardID,
			Reason:     "invalid_bounds",
			DurationMS: time.Since(shardStart).Milliseconds(),
		}, fmt.Errorf("invalid shard bounds: [%d, %d) for %d PRs", startI, endI, n)
	}

	var pairsChecked int64
	var conflictsFound int64
	earlyExit := false

	p.executor.sem <- struct{}{}
	defer func() { <-p.executor.sem }()

	for i := startI; i < endI && !earlyExit; i++ {
		select {
		case <-p.ctx.Done():
			return ShardMetrics{
				ShardID:      shardID,
				PairsChecked: pairsChecked,
				Reason:       "cancelled",
				DurationMS:   time.Since(shardStart).Milliseconds(),
			}, p.ctx.Err()
		default:
		}

		for j := i + 1; j < n; j++ {
			select {
			case <-p.ctx.Done():
				return ShardMetrics{
					ShardID:      shardID,
					PairsChecked: pairsChecked,
					Reason:       "cancelled",
					DurationMS:   time.Since(shardStart).Milliseconds(),
				}, p.ctx.Err()
			default:
			}

			pairsChecked++
			left := p.prs[i]
			right := p.prs[j]

			if left.BaseBranch == "" || right.BaseBranch == "" || left.BaseBranch != right.BaseBranch {
				continue
			}

			sharedFiles := intersectFilesShared(left.FilesChanged, right.FilesChanged)

			if len(sharedFiles) == 0 && left.Mergeable != "conflicting" && right.Mergeable != "conflicting" {
				continue
			}

			conflictType := "file_overlap"
			reason := fmt.Sprintf("shared files: %s", joinStrings(sharedFiles, ", "))
			if len(sharedFiles) == 0 {
				conflictType = "mergeability_signal"
				reason = "both PRs signal merge conflicts"
			}

			conflict := types.ConflictPair{
				SourcePR:     left.Number,
				TargetPR:     right.Number,
				ConflictType: conflictType,
				FilesTouched: sharedFiles,
				Severity:     "high",
				Reason:       reason,
			}

			select {
			case conflictChan <- conflict:
				conflictsFound++
			case <-p.ctx.Done():
				return ShardMetrics{
					ShardID:      shardID,
					PairsChecked: pairsChecked,
					Reason:       "cancelled",
					DurationMS:   time.Since(shardStart).Milliseconds(),
				}, p.ctx.Err()
			}

			if p.executor.config.EarlyExitThreshold > 0 && int(conflictsFound) >= p.executor.config.EarlyExitThreshold {
				earlyExit = true
			}
		}
	}

	reason := "shard_processed"
	if earlyExit {
		reason = "early_exit_threshold"
	}

	return ShardMetrics{
		ShardID:        shardID,
		PairsChecked:   pairsChecked,
		ConflictsFound: conflictsFound,
		DurationMS:     time.Since(shardStart).Milliseconds(),
		Reason:         reason,
	}, nil
}

func (e *PairwiseExecutor) buildTelemetry(execStart time.Time, shardsTotal, earlyExits, pairsProcessed, conflictsFound, graphDeltaEdges int64) types.OperationTelemetry {
	return types.OperationTelemetry{
		PoolStrategy:    "parallel_pairwise_sharded",
		PoolSizeBefore:  0,
		PoolSizeAfter:   0,
		GraphDeltaEdges: int(graphDeltaEdges),
		DecayPolicy:     "none",
		PairwiseShards:  int(shardsTotal),
		StageLatenciesMS: map[string]int{
			"pairwise_total_ms": int(time.Since(execStart).Milliseconds()),
		},
		StageDropCounts: map[string]int{
			"pairs_processed": int(pairsProcessed),
			"conflicts_found": int(conflictsFound),
			"early_exits":     int(earlyExits),
		},
	}
}

func intersectFilesShared(left, right []string) []string {
	if len(left) == 0 || len(right) == 0 {
		return nil
	}

	rightSet := make(map[string]struct{}, len(right))
	for _, path := range right {
		rightSet[path] = struct{}{}
	}

	shared := make([]string, 0)
	for _, path := range left {
		if _, ok := rightSet[path]; ok {
			shared = append(shared, path)
		}
	}

	sort.Strings(shared)
	return slices.Compact(shared)
}

func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(strs[0])
	for _, s := range strs[1:] {
		b.WriteString(sep)
		b.WriteString(s)
	}
	return b.String()
}
