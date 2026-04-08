package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/jeffersonnunn/pratc/internal/monitor/data"
)

type TimelinePanel struct {
	buckets []data.ActivityBucket
}

func NewTimelinePanel() *TimelinePanel {
	return &TimelinePanel{
		buckets: nil,
	}
}

func (t *TimelinePanel) SetBuckets(buckets []data.ActivityBucket) {
	t.buckets = buckets
}

func (t *TimelinePanel) View(width int) string {
	if len(t.buckets) == 0 {
		return t.renderEmpty()
	}

	var sb strings.Builder
	sb.WriteString(t.renderHeader())
	sb.WriteString("\n")
	sb.WriteString(t.renderHeatStrip())
	sb.WriteString("\n")
	sb.WriteString(t.renderTimeLabels())

	return sb.String()
}

func (t *TimelinePanel) renderEmpty() string {
	return "[ no activity data ]"
}

func (t *TimelinePanel) renderHeader() string {
	return "TIMELINE - Last 4 Hours"
}

func (t *TimelinePanel) renderHeatStrip() string {
	if len(t.buckets) == 0 {
		return strings.Repeat(" ", 16)
	}

	maxCount := 0
	for _, b := range t.buckets {
		if b.RequestCount > maxCount {
			maxCount = b.RequestCount
		}
	}

	startIdx := 0
	if len(t.buckets) > 16 {
		startIdx = len(t.buckets) - 16
	}

	segmentCount := 16
	if len(t.buckets) < 16 {
		segmentCount = len(t.buckets)
	}

	var result strings.Builder
	for i := 0; i < segmentCount; i++ {
		idx := startIdx + i
		if idx < len(t.buckets) {
			bucket := t.buckets[idx]
			block := t.requestCountToBlock(bucket.RequestCount, maxCount)
			result.WriteString(block)
		} else {
			result.WriteString(" ")
		}
	}

	return result.String()
}

func (t *TimelinePanel) requestCountToBlock(count, maxCount int) string {
	if maxCount == 0 {
		return "\033[36m▁\033[0m"
	}

	ratio := float64(count) / float64(maxCount)
	level := int(ratio * 7)

	if level > 7 {
		level = 7
	}
	if level < 0 {
		level = 0
	}

	blocks := []string{"▁", "▂", "▃", "▄", "▅", "▆", "▇", "█"}

	if level == 0 {
		return "\033[36m▁\033[0m"
	}

	if level >= 5 {
		return fmt.Sprintf("\033[96m%c\033[0m", rune(blocks[level][0]))
	}
	return fmt.Sprintf("\033[36m%s\033[0m", blocks[level])
}

func (t *TimelinePanel) renderTimeLabels() string {
	hour := time.Now().Hour()
	hourStr := fmt.Sprintf("%02d:00", hour)

	labels := [16]string{}
	for i := 0; i < 16; i++ {
		labels[i] = " "
	}

	labels[0] = fmt.Sprintf("%02d:00", (hour+20)%24)
	labels[4] = fmt.Sprintf("%02d:00", (hour+21)%24)
	labels[8] = fmt.Sprintf("%02d:00", (hour+22)%24)
	labels[12] = fmt.Sprintf("%02d:00", (hour+23)%24)
	labels[15] = hourStr

	var line strings.Builder
	for i, lbl := range labels {
		if lbl != " " {
			line.WriteString(lbl)
		} else {
			line.WriteString(" ")
		}
		if i < 15 {
			line.WriteString(" ")
		}
	}

	return line.String()
}
