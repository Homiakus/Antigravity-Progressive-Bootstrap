package engineering

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Allowed task and finding statuses for living plan governance.
type TaskStatus string

const (
	TaskStatusTodo       TaskStatus = "TODO"
	TaskStatusReady      TaskStatus = "READY"
	TaskStatusInProgress TaskStatus = "IN_PROGRESS"
	TaskStatusBlocked    TaskStatus = "BLOCKED"
	TaskStatusDone       TaskStatus = "DONE"
	TaskStatusRejected   TaskStatus = "REJECTED"
	TaskStatusSuperseded TaskStatus = "SUPERSEDED"
)

var allowedTaskStatuses = map[TaskStatus]struct{}{
	TaskStatusTodo:       {},
	TaskStatusReady:      {},
	TaskStatusInProgress: {},
	TaskStatusBlocked:    {},
	TaskStatusDone:       {},
	TaskStatusRejected:   {},
	TaskStatusSuperseded: {},
}

type FindingStatus string

const (
	FindingStatusOpen               FindingStatus = "OPEN"
	FindingStatusInProgress         FindingStatus = "IN_PROGRESS"
	FindingStatusResolved           FindingStatus = "RESOLVED"
	FindingStatusAcceptedLimitation FindingStatus = "ACCEPTED_LIMITATION"
	FindingStatusRejected           FindingStatus = "REJECTED"
)

var allowedFindingStatuses = map[FindingStatus]struct{}{
	FindingStatusOpen:               {},
	FindingStatusInProgress:         {},
	FindingStatusResolved:           {},
	FindingStatusAcceptedLimitation: {},
	FindingStatusRejected:           {},
}

type FindingSeverity string

const (
	SeverityCritical      FindingSeverity = "CRITICAL"
	SeverityHigh          FindingSeverity = "HIGH"
	SeverityMedium        FindingSeverity = "MEDIUM"
	SeverityLow           FindingSeverity = "LOW"
	SeverityInformational FindingSeverity = "INFORMATIONAL"
)

var allowedFindingSeverities = map[FindingSeverity]struct{}{
	SeverityCritical:      {},
	SeverityHigh:          {},
	SeverityMedium:        {},
	SeverityLow:           {},
	SeverityInformational: {},
}

var (
	taskDefRE       = regexp.MustCompile(`(?m)^###\s+(T-\d{3,})\s*[-—:]\s*(.*)$`)
	findingDefRE    = regexp.MustCompile(`(?m)^###\s+(F-\d{3,})\s*[-—:]\s*(.*)$`)
	taskStatusRE    = regexp.MustCompile(`(?i)\*\*Status:\*\*\s*([A-Za-z_-]+)`)
	taskPriorityRE  = regexp.MustCompile(`(?i)\*\*Priority:\*\*\s*(P\d+)`)
	taskDepsRE      = regexp.MustCompile(`(?i)Dependencies:\s*([^.\n\r]+)`)
	findingStatusRE = regexp.MustCompile(`(?i)\*\*Status:\*\*\s*([A-Za-z_-]+)`)
	findingSevRE    = regexp.MustCompile(`(?i)\*\*Severity:\*\*\s*([A-Za-z]+)`)
)

type TaskRecord struct {
	ID           string
	Title        string
	Status       TaskStatus
	Priority     string
	Dependencies []string
	LineNumber   int
}

type FindingRecord struct {
	ID         string
	Title      string
	Severity   FindingSeverity
	Status     FindingStatus
	LineNumber int
}

type AuditIssueCategory string

const (
	IssueDuplicateID          AuditIssueCategory = "DUPLICATE_ID"
	IssueInvalidStatus        AuditIssueCategory = "INVALID_STATUS"
	IssueInvalidSeverity      AuditIssueCategory = "INVALID_SEVERITY"
	IssueDanglingDependency   AuditIssueCategory = "DANGLING_DEPENDENCY"
	IssueDependencyCycle      AuditIssueCategory = "DEPENDENCY_CYCLE"
	IssueUnmappedHighFinding  AuditIssueCategory = "UNMAPPED_HIGH_FINDING"
	IssueMissingCheckpoint    AuditIssueCategory = "MISSING_CHECKPOINT"
	IssueMissingPreflightFile AuditIssueCategory = "MISSING_PREFLIGHT_FILE"
	IssueStructuralFormat     AuditIssueCategory = "STRUCTURAL_FORMAT"
)

type AuditIssue struct {
	Category   AuditIssueCategory
	Severity   string // "ERROR" or "WARNING"
	Message    string
	LineNumber int
}

func (i AuditIssue) String() string {
	if i.LineNumber > 0 {
		return fmt.Sprintf("[%s] %s at line %d: %s", i.Severity, i.Category, i.LineNumber, i.Message)
	}
	return fmt.Sprintf("[%s] %s: %s", i.Severity, i.Category, i.Message)
}

type PlanAuditReport struct {
	PlanPath     string
	PlanDigest   string
	Tasks        map[string]TaskRecord
	Findings     map[string]FindingRecord
	DAGOrder     []string
	Issues       []AuditIssue
	HasErrors    bool
	Checkpoint   PlanCheckpoint
}

// AuditPlan audits living plan content for structural integrity, ID uniqueness,
// allowed statuses, DAG acyclicity, finding governance, and checkpoint validity.
func AuditPlan(plan string) (PlanAuditReport, error) {
	sum := sha256.Sum256([]byte(plan))
	digest := hex.EncodeToString(sum[:])

	report := PlanAuditReport{
		PlanDigest: digest,
		Tasks:      make(map[string]TaskRecord),
		Findings:   make(map[string]FindingRecord),
		Issues:     make([]AuditIssue, 0),
	}

	lines := strings.Split(plan, "\n")

	// 1. Parse Tasks and detect duplicates
	for lineIdx, line := range lines {
		trimmed := strings.TrimSpace(line)
		if match := taskHeadingRE.FindStringSubmatch(trimmed); len(match) == 2 {
			id := match[1]
			lineNum := lineIdx + 1
			if _, exists := report.Tasks[id]; exists {
				report.addIssue(IssueDuplicateID, "ERROR", fmt.Sprintf("duplicate task heading %s", id), lineNum)
			} else {
				// Parse task metadata from subsequent lines in the section
				title := ""
				if defMatch := taskDefRE.FindStringSubmatch(trimmed); len(defMatch) == 3 {
					title = strings.TrimSpace(defMatch[2])
				}
				status, priority, deps := parseTaskMeta(lines, lineIdx)
				rec := TaskRecord{
					ID:           id,
					Title:        title,
					Status:       status,
					Priority:     priority,
					Dependencies: deps,
					LineNumber:   lineNum,
				}
				if status == "" {
					report.addIssue(IssueInvalidStatus, "ERROR", fmt.Sprintf("task %s has no **Status:** defined", id), lineNum)
				} else if _, allowed := allowedTaskStatuses[status]; !allowed {
					report.addIssue(IssueInvalidStatus, "ERROR", fmt.Sprintf("task %s has invalid status %q", id, status), lineNum)
				}
				report.Tasks[id] = rec
			}
		}

		if match := findingHeadingRE.FindStringSubmatch(trimmed); len(match) == 2 {
			id := match[1]
			lineNum := lineIdx + 1
			if _, exists := report.Findings[id]; exists {
				report.addIssue(IssueDuplicateID, "ERROR", fmt.Sprintf("duplicate finding heading %s", id), lineNum)
			} else {
				title := ""
				if defMatch := findingDefRE.FindStringSubmatch(trimmed); len(defMatch) == 3 {
					title = strings.TrimSpace(defMatch[2])
				}
				status, sev := parseFindingMeta(lines, lineIdx)
				rec := FindingRecord{
					ID:         id,
					Title:      title,
					Severity:   sev,
					Status:     status,
					LineNumber: lineNum,
				}
				if status == "" {
					report.addIssue(IssueInvalidStatus, "ERROR", fmt.Sprintf("finding %s has no **Status:** defined", id), lineNum)
				} else if _, allowed := allowedFindingStatuses[status]; !allowed {
					report.addIssue(IssueInvalidStatus, "ERROR", fmt.Sprintf("finding %s has invalid status %q", id, status), lineNum)
				}
				if sev != "" {
					if _, allowed := allowedFindingSeverities[sev]; !allowed {
						report.addIssue(IssueInvalidSeverity, "ERROR", fmt.Sprintf("finding %s has invalid severity %q", id, sev), lineNum)
					}
				}
				report.Findings[id] = rec
			}
		}
	}

	// 2. Validate DAG and Dependencies
	adjList := make(map[string][]string)
	inDegree := make(map[string]int)
	for id := range report.Tasks {
		inDegree[id] = 0
		adjList[id] = make([]string, 0)
	}

	for id, task := range report.Tasks {
		for _, dep := range task.Dependencies {
			if _, exists := report.Tasks[dep]; !exists {
				report.addIssue(IssueDanglingDependency, "ERROR", fmt.Sprintf("task %s depends on undeclared task %s", id, dep), task.LineNumber)
			} else {
				adjList[dep] = append(adjList[dep], id)
				inDegree[id]++
			}
		}
	}

	// Kahn's algorithm for topological ordering and cycle detection
	queue := make([]string, 0)
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}
	sort.Strings(queue)

	ordered := make([]string, 0, len(report.Tasks))
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		ordered = append(ordered, curr)

		for _, next := range adjList[curr] {
			inDegree[next]--
			if inDegree[next] == 0 {
				queue = append(queue, next)
			}
		}
		sort.Strings(queue)
	}

	if len(ordered) < len(report.Tasks) {
		report.addIssue(IssueDependencyCycle, "ERROR", "task dependency graph contains cyclic dependencies", 0)
	} else {
		report.DAGOrder = ordered
	}

	// 3. Verify Finding Governance (No unmapped OPEN Critical/High findings)
	for id, f := range report.Findings {
		if (f.Severity == SeverityCritical || f.Severity == SeverityHigh) && f.Status == FindingStatusOpen {
			// Ensure plan mentions finding ID in active tasks or recent log
			if !strings.Contains(plan, id) {
				report.addIssue(IssueUnmappedHighFinding, "WARNING", fmt.Sprintf("%s finding %s is OPEN without active remediation task reference", f.Severity, id), f.LineNumber)
			}
		}
	}

	// 4. Validate Context Compression Checkpoint
	cp, err := ParseLatestPlanCheckpoint(plan)
	if err != nil {
		report.addIssue(IssueMissingCheckpoint, "ERROR", err.Error(), 0)
	} else {
		report.Checkpoint = cp
		for _, reqField := range checkpointRequiredFields {
			if strings.TrimSpace(cp.Fields[reqField]) == "" {
				report.addIssue(IssueMissingCheckpoint, "ERROR", fmt.Sprintf("latest Context Compression Checkpoint missing required field %q", reqField), 0)
			}
		}
	}

	// Determine HasErrors
	for _, issue := range report.Issues {
		if issue.Severity == "ERROR" {
			report.HasErrors = true
			break
		}
	}

	if report.HasErrors {
		return report, fmt.Errorf("plan structural audit failed with %d error(s)", report.errorCount())
	}

	return report, nil
}

// AuditPlanFile audits a file on disk and verifies pre-flight file existence for IN_PROGRESS tasks.
func AuditPlanFile(planPath string) (PlanAuditReport, error) {
	absPath, err := filepath.Abs(planPath)
	if err != nil {
		return PlanAuditReport{}, fmt.Errorf("resolve plan path: %w", err)
	}
	data, err := os.ReadFile(absPath)
	if err != nil {
		return PlanAuditReport{}, fmt.Errorf("read plan file: %w", err)
	}

	report, auditErr := AuditPlan(string(data))
	report.PlanPath = absPath

	// Check preflight directory relative to workspace root
	wsRoot := filepath.Dir(absPath)
	preflightDir := filepath.Join(wsRoot, ".engineering", "preflight")

	for id, task := range report.Tasks {
		if task.Status == TaskStatusInProgress {
			pfFile := filepath.Join(preflightDir, fmt.Sprintf("%s.md", id))
			if _, statErr := os.Stat(pfFile); statErr != nil {
				report.addIssue(IssueMissingPreflightFile, "ERROR", fmt.Sprintf("task %s is IN_PROGRESS but pre-flight file %s does not exist", id, pfFile), task.LineNumber)
				report.HasErrors = true
			}
		}
	}

	if report.HasErrors && auditErr == nil {
		return report, fmt.Errorf("plan structural audit failed with %d error(s)", report.errorCount())
	}
	return report, auditErr
}

func (r *PlanAuditReport) addIssue(cat AuditIssueCategory, sev string, msg string, line int) {
	r.Issues = append(r.Issues, AuditIssue{
		Category:   cat,
		Severity:   sev,
		Message:    msg,
		LineNumber: line,
	})
}

func (r *PlanAuditReport) errorCount() int {
	c := 0
	for _, i := range r.Issues {
		if i.Severity == "ERROR" {
			c++
		}
	}
	return c
}

func parseTaskMeta(lines []string, startLine int) (TaskStatus, string, []string) {
	var status TaskStatus
	var priority string
	var deps []string

	for i := startLine; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if i > startLine && strings.HasPrefix(line, "### ") {
			break
		}
		if status == "" {
			if match := taskStatusRE.FindStringSubmatch(line); len(match) == 2 {
				status = TaskStatus(strings.ToUpper(match[1]))
			}
		}
		if priority == "" {
			if match := taskPriorityRE.FindStringSubmatch(line); len(match) == 2 {
				priority = strings.ToUpper(match[1])
			}
		}
		if len(deps) == 0 {
			if match := taskDepsRE.FindStringSubmatch(line); len(match) == 2 {
				raw := match[1]
				parts := strings.FieldsFunc(raw, func(r rune) bool {
					return r == ',' || r == ';' || r == ' ' || r == '\t'
				})
				for _, p := range parts {
					p = strings.TrimSpace(p)
					if taskIDRE.MatchString(p) {
						deps = append(deps, p)
					}
				}
			}
		}
	}
	return status, priority, deps
}

func parseFindingMeta(lines []string, startLine int) (FindingStatus, FindingSeverity) {
	var status FindingStatus
	var sev FindingSeverity

	for i := startLine; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if i > startLine && strings.HasPrefix(line, "### ") {
			break
		}
		if status == "" {
			if match := findingStatusRE.FindStringSubmatch(line); len(match) == 2 {
				rawUpper := strings.ToUpper(strings.TrimSpace(match[1]))
				if strings.HasPrefix(rawUpper, "RESOLVED") {
					status = FindingStatusResolved
				} else if strings.HasPrefix(rawUpper, "PARTIAL") || strings.HasPrefix(rawUpper, "PLAN") || strings.HasPrefix(rawUpper, "IN_PROGRESS") {
					status = FindingStatusInProgress
				} else if strings.HasPrefix(rawUpper, "OPEN") {
					status = FindingStatusOpen
				} else if strings.HasPrefix(rawUpper, "ACCEPTED") {
					status = FindingStatusAcceptedLimitation
				} else if strings.HasPrefix(rawUpper, "REJECTED") {
					status = FindingStatusRejected
				} else {
					status = FindingStatus(rawUpper)
				}
			}
		}
		if sev == "" {
			if match := findingSevRE.FindStringSubmatch(line); len(match) == 2 {
				rawUpper := strings.ToUpper(strings.TrimSpace(match[1]))
				if strings.HasPrefix(rawUpper, "CRIT") {
					sev = SeverityCritical
				} else if strings.HasPrefix(rawUpper, "HIGH") {
					sev = SeverityHigh
				} else if strings.HasPrefix(rawUpper, "MED") {
					sev = SeverityMedium
				} else if strings.HasPrefix(rawUpper, "LOW") {
					sev = SeverityLow
				} else if strings.HasPrefix(rawUpper, "INFO") {
					sev = SeverityInformational
				} else {
					sev = FindingSeverity(rawUpper)
				}
			}
		}
	}
	return status, sev
}
