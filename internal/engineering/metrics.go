package engineering

// ProcessMetrics provides a machine-readable summary of living plan progress,
// task velocity, findings resolution, and structural health.
type ProcessMetrics struct {
	PlanDigest         string            `json:"planDigest"`
	TotalTasks         int               `json:"totalTasks"`
	DoneTasks          int               `json:"doneTasks"`
	InProgressTasks    int               `json:"inProgressTasks"`
	ReadyTasks         int               `json:"readyTasks"`
	TodoTasks          int               `json:"todoTasks"`
	BlockedTasks       int               `json:"blockedTasks"`
	OtherTasks         int               `json:"otherTasks"`
	CompletionRatePct  float64           `json:"completionRatePct"`
	TotalFindings      int               `json:"totalFindings"`
	OpenFindings       int               `json:"openFindings"`
	ResolvedFindings   int               `json:"resolvedFindings"`
	FindingsBySeverity map[string]int    `json:"findingsBySeverity"`
	FindingsByStatus   map[string]int    `json:"findingsByStatus"`
	AuditIssuesCount   int               `json:"auditIssuesCount"`
	AuditValid         bool              `json:"auditValid"`
	CurrentMilestone   string            `json:"currentMilestone"`
	LatestTaskID       string            `json:"latestTaskId"`
}

// ComputeProcessMetrics computes process metrics from a PlanAuditReport.
func ComputeProcessMetrics(report PlanAuditReport) ProcessMetrics {
	m := ProcessMetrics{
		PlanDigest:         report.PlanDigest,
		TotalTasks:         len(report.Tasks),
		TotalFindings:      len(report.Findings),
		FindingsBySeverity: make(map[string]int),
		FindingsByStatus:   make(map[string]int),
		AuditIssuesCount:   len(report.Issues),
		AuditValid:         !report.HasErrors,
		CurrentMilestone:   report.Checkpoint.Fields["CURRENT QUALIFIED MILESTONE"],
		LatestTaskID:       report.Checkpoint.Fields["COMPLETED THIS ITERATION"],
	}

	for _, task := range report.Tasks {
		switch task.Status {
		case TaskStatusDone:
			m.DoneTasks++
		case TaskStatusInProgress:
			m.InProgressTasks++
		case TaskStatusReady:
			m.ReadyTasks++
		case TaskStatusTodo:
			m.TodoTasks++
		case TaskStatusBlocked:
			m.BlockedTasks++
		default:
			m.OtherTasks++
		}
	}

	if m.TotalTasks > 0 {
		m.CompletionRatePct = (float64(m.DoneTasks) / float64(m.TotalTasks)) * 100.0
	}

	for _, f := range report.Findings {
		sev := string(f.Severity)
		if sev == "" {
			sev = "UNSPECIFIED"
		}
		m.FindingsBySeverity[sev]++

		st := string(f.Status)
		if st == "" {
			st = "UNSPECIFIED"
		}
		m.FindingsByStatus[st]++

		if f.Status == FindingStatusOpen {
			m.OpenFindings++
		} else if f.Status == FindingStatusResolved {
			m.ResolvedFindings++
		}
	}

	return m
}
