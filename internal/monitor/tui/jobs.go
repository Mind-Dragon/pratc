package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbletea"

	"github.com/jeffersonnunn/pratc/internal/monitor/data"
)

// JobsList displays a scrollable list of sync jobs with progress bars.
type JobsList struct {
	jobs   []data.SyncJobView
	cursor int
	offset int
	height int
}

// NewJobsList creates a new JobsList with default height of 10.
func NewJobsList() *JobsList {
	return &JobsList{
		height: 10,
	}
}

// SetJobs updates the job list with new data.
func (j *JobsList) SetJobs(jobs []data.SyncJobView) {
	j.jobs = jobs
	// Ensure cursor is within bounds
	if j.cursor >= len(jobs) {
		if len(jobs) > 0 {
			j.cursor = len(jobs) - 1
		} else {
			j.cursor = 0
		}
	}
	// Ensure offset is valid
	if j.offset > j.cursor {
		j.offset = j.cursor
	}
	j.ensureCursorVisible()
}

// Update handles keyboard input for navigation.
// Supports: ↑/↓ for line navigation with wrap-around, PgUp/PgDn for page scrolling.
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
				j.cursor = len(j.jobs) - 1 // wrap around
			}
			j.ensureCursorVisible()

		case tea.KeyDown:
			j.cursor++
			if j.cursor >= len(j.jobs) {
				j.cursor = 0 // wrap around
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
	}
	return nil
}

// ensureCursorVisible scrolls the viewport to keep the cursor visible.
func (j *JobsList) ensureCursorVisible() {
	if j.cursor < j.offset {
		j.offset = j.cursor
	}
	if j.cursor >= j.offset+j.height {
		j.offset = j.cursor - j.height + 1
	}
}

// View renders the jobs list with progress bars and status badges.
// Each job is displayed as: [STATUS] repo/name  [████████████░░░░]  67%
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

// cursorJob represents a job with its index in the full list.
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
	progressBar := j.renderProgressBar(job.Progress, job.Status)
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
	switch status {
	case data.StatusActive:
		return "\033[36m[ACTIVE]\033[0m"
	case data.StatusPaused:
		return "\033[33m[PAUSED]\033[0m"
	case data.StatusFailed:
		return "\033[31m[FAILED]\033[0m"
	case data.StatusQueued:
		return "\033[90m[QUEUED]\033[0m"
	case data.StatusCompleted:
		return "\033[32m[DONE]\033[0m"
	default:
		return fmt.Sprintf("\033[90m[%s]\033[0m", strings.ToUpper(status))
	}
}

func (j *JobsList) renderProgressBar(progress int, status string) string {
	const barWidth = 40
	filled := (progress * barWidth) / 100

	// Limit filled to barWidth
	if filled > barWidth {
		filled = barWidth
	}

	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
	return "[" + bar + "]"
}

// truncateRepo truncates a repo name to fit within the given width.
func (j *JobsList) truncateRepo(repo string, maxLen int) string {
	if len(repo) <= maxLen {
		return repo
	}
	return repo[:maxLen-2] + ".."
}
