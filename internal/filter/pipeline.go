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

func (p *Pipeline) BuildCandidatePool(prs []types.PR, clusterByPR map[int]string) (pool []types.PR, rejections []types.PlanRejection) {
	pool, rejections = ApplyFilters(prs, p.includeBots)

	if len(pool) == 0 {
		return pool, rejections
	}

	AssignClusterIDs(pool, clusterByPR)

	pool = SortPoolByPriority(pool, p.now)

	cappedPool, capRejections := CapPool(pool, types.DefaultPoolCap)
	pool = cappedPool
	rejections = append(rejections, capRejections...)

	return pool, rejections
}
