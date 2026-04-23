package filter

import (
	"time"

	"github.com/jeffersonnunn/pratc/internal/types"
)

type Pipeline struct {
	now         time.Time
	includeBots bool
}

func NewPipeline(now time.Time) *Pipeline {
	return &Pipeline{now: now, includeBots: false}
}

func (p *Pipeline) WithIncludeBots(includeBots bool) *Pipeline {
	p.includeBots = includeBots
	return p
}

// BuildCandidatePool applies the default runtime filtering and priority sort.
//
// It intentionally does not call CapPool or enforce the legacy
// types.DefaultCandidatePoolCap/types.DefaultPoolCap constants. Any caller that
// wants a hard pool limit must invoke CapPool explicitly after sorting.
func (p *Pipeline) BuildCandidatePool(prs []types.PR, clusterByPR map[int]string) (pool []types.PR, rejections []types.PlanRejection) {
	pool, rejections = ApplyFilters(prs, p.includeBots)

	if len(pool) == 0 {
		return pool, rejections
	}

	AssignClusterIDs(pool, clusterByPR)

	pool = SortPoolByPriority(pool, p.now)

	return pool, rejections
}
