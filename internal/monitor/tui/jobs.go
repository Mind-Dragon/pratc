package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbletea"

	"github.com/jeffersonnunn/pratc/internal/monitor/data"
)

type JobsList struct {
	jobs       []data.SyncJobView
	cursor     int
	offset     int
	height     int
	shimmerPos int
}

func NewJobsList() *JobsList {
	return &JobsList{
		height:     10,
		shimmerPos: 0,
	}
}

func (j *JobsList) SetJobs(jobs []data.SyncJobView) {
	j.jobs = jobs
	if j.cursor >= len(jobs) {
		if len(jobs) > 0 {
			j.cursor = len(jobs) - 1
		} else {
			j.cursor = 0
		}
	}
	if j.offset > j.cursor {
		j.offset = j.cursor
	}
	j.ensureCursorVisible()
}

func (j *JobsList) Update(msg tea.Msg) tea.Cmd {
	if len(j.jobs) == 0 {
		return nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyUp:
			j.cursor--
			if j.cursor < 0 {
				j.cursor = len(j.jobs) - 1
			}
			j.ensureCursorVisible()

		case tea.KeyDown:
			j.cursor++
			if j.cursor >= len(j.jobs) {
				j.cursor = 0
			}
			j.ensureCursorVisible()

		case tea.KeyPgUp:
			j.cursor -= j.height
			if j.cursor < 0 {
				j.cursor = 0
			}
			j.offset = j.cursor
			j.ensureCursorVisible()

		case tea.KeyPgDown:
			j.cursor += j.height
			if j.cursor >= len(j.jobs) {
				j.cursor = len(j.jobs) - 1
			}
			j.offset = j.cursor
			j.ensureCursorVisible()
		}
	case TickMsg:
		j.shimmerPos = (j.shimmerPos + 10) % 100
	}
	return nil
}

func (j *JobsList) ensureCursorVisible() {
	if j.cursor < j.offset {
		j.offset = j.cursor
	}
	if j.cursor >= j.offset+j.height {
		j.offset = j.cursor - j.height + 1
	}
}

func (j *JobsList) View(width int) string {
	if len(j.jobs) == 0 {
		return j.renderEmpty()
	}

	var sb strings.Builder
	visibleJobs := j.visibleJobs()

	for idx, cj := range visibleJobs {
		isCursor := cj.index == j.cursor
		line := j.renderJob(cj.job, isCursor, width)
		sb.WriteString(line)
		if idx < len(visibleJobs)-1 {
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

type cursorJob struct {
	job   data.SyncJobView
	index int
}

func (j *JobsList) visibleJobs() []cursorJob {
	var visible []cursorJob
	end := j.offset + j.height
	if end > len(j.jobs) {
		end = len(j.jobs)
	}
	for i := j.offset; i < end; i++ {
		visible = append(visible, cursorJob{
			job:   j.jobs[i],
			index: i,
		})
	}
	return visible
}

func (j *JobsList) renderEmpty() string {
	return "[ no jobs ]"
}

func (j *JobsList) renderJob(job data.SyncJobView, isCursor bool, width int) string {
	statusBadge := j.statusBadge(job.Status)
	repoName := j.truncateRepo(job.Repo, 20)
	progressBar := j.renderProgressBarWithShimmer(job.Progress, job.Status)
	progressPct := fmt.Sprintf("%3d%%", job.Progress)

	var sb strings.Builder
	if isCursor {
		sb.WriteString("> ")
	} else {
		sb.WriteString("  ")
	}
	sb.WriteString(statusBadge)
	sb.WriteString(" ")
	sb.WriteString(repoName)
	sb.WriteString("  ")
	sb.WriteString(progressBar)
	sb.WriteString(" ")
	sb.WriteString(progressPct)

	return sb.String()
}

func (j *JobsList) statusBadge(status string) string {
	color := GetStatusColor(status)
	reset := ColorReset
	return color + "[" + strings.ToUpper(status) + "]" + reset
}

func (j *JobsList) renderProgressBarWithShimmer(progress int, status string) string {
	const barWidth = 40
	filled := (progress * barWidth) / 100

	if filled > barWidth {
		filled = barWidth
	}

	isActive := status == data.StatusActive || status == "in_progress" || status == "running"

	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)

	if isActive {
		shimmerStart := j.shimmerPos / 3
		shimmerEnd := shimmerStart + 5
		if shimmerEnd > barWidth {
			shimmerEnd = barWidth
		}

		var shimmerBar strings.Builder
		for i := 0; i < barWidth; i++ {
			if i >= shimmerStart && i < shimmerEnd && i < filled {
				shimmerBar.WriteString("\033[96m█\033[0m")
			} else if i < filled {
				shimmerBar.WriteString("█")
			} else {
				shimmerBar.WriteString("░")
			}
		}
		bar = shimmerBar.String()
	}

	return "[" + bar + "]"
}

func (j *JobsList) truncateRepo(repo string, maxLen int) string {
	if len(repo) <= maxLen {
		return repo
	}
	return repo[:maxLen-2] + ".."
}
