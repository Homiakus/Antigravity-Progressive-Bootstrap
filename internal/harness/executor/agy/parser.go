package agy

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	harnessexecutor "github.com/homiakus/agctl/internal/harness/executor"
)

const DefaultMaxLineBytes = 8 * 1024 * 1024

var (
	ErrPermissionDenied = errors.New("AGY headless required tool permission was denied")
	ErrMissingResult    = errors.New("AGY stream ended without terminal result event")
	ErrResultFailed     = errors.New("AGY terminal result was not successful")
)

type Outcome struct {
	SawResult        bool   `json:"sawResult"`
	ResultStatus     string `json:"resultStatus,omitempty"`
	ResultError      string `json:"resultError,omitempty"`
	PermissionDenied bool   `json:"permissionDenied,omitempty"`
	MalformedLines   int    `json:"malformedLines,omitempty"`
	OversizedLines   int    `json:"oversizedLines,omitempty"`
}

type Parser struct {
	mu       sync.Mutex
	maxLine  int
	stdout   lineState
	stderr   lineState
	outcome  Outcome
	finalized bool
}

type lineState struct {
	buffer     []byte
	discarding bool
}

func NewParser(maxLineBytes int) *Parser {
	if maxLineBytes <= 0 {
		maxLineBytes = DefaultMaxLineBytes
	}
	return &Parser{maxLine: maxLineBytes}
}

func (p *Parser) Feed(stream harnessexecutor.Stream, data []byte) {
	if len(data) == 0 {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.finalized {
		return
	}
	state := &p.stdout
	if stream == harnessexecutor.StreamStderr {
		state = &p.stderr
	}
	p.feedLocked(stream, state, data)
}

func (p *Parser) feedLocked(stream harnessexecutor.Stream, state *lineState, data []byte) {
	for len(data) > 0 {
		idx := bytes.IndexByte(data, '\n')
		if idx < 0 {
			if state.discarding {
				return
			}
			if len(state.buffer)+len(data) > p.maxLine {
				state.buffer = nil
				state.discarding = true
				p.outcome.OversizedLines++
				return
			}
			state.buffer = append(state.buffer, data...)
			return
		}

		segment := data[:idx]
		data = data[idx+1:]
		if state.discarding {
			state.discarding = false
			state.buffer = nil
			continue
		}
		if len(state.buffer)+len(segment) > p.maxLine {
			state.buffer = nil
			p.outcome.OversizedLines++
			continue
		}
		line := make([]byte, 0, len(state.buffer)+len(segment))
		line = append(line, state.buffer...)
		line = append(line, segment...)
		state.buffer = nil
		p.observeLineLocked(stream, bytes.TrimSuffix(line, []byte{'\r'}))
	}
}

func (p *Parser) observeLineLocked(stream harnessexecutor.Stream, line []byte) {
	text := strings.TrimSpace(string(line))
	if text == "" {
		return
	}
	if stream == harnessexecutor.StreamStderr {
		if looksLikePermissionDenial(text) {
			p.outcome.PermissionDenied = true
		}
		return
	}

	var event struct {
		Event      string `json:"event"`
		StepUpdate struct {
			StepType string `json:"step_type"`
			ToolInfo struct {
				Error *struct {
					Type    string `json:"type"`
					Message string `json:"message"`
				} `json:"error"`
			} `json:"tool_info"`
		} `json:"step_update"`
		Result struct {
			Status string `json:"status"`
			Error  string `json:"error"`
		} `json:"result"`
	}
	if err := json.Unmarshal(line, &event); err != nil {
		p.outcome.MalformedLines++
		return
	}
	if event.Event == "result" {
		p.outcome.SawResult = true
		p.outcome.ResultStatus = strings.TrimSpace(event.Result.Status)
		p.outcome.ResultError = strings.TrimSpace(event.Result.Error)
	}
	if event.Event == "step_update" && event.StepUpdate.StepType == "tool" && event.StepUpdate.ToolInfo.Error != nil {
		msg := event.StepUpdate.ToolInfo.Error.Type + " " + event.StepUpdate.ToolInfo.Error.Message
		if looksLikePermissionDenial(msg) {
			p.outcome.PermissionDenied = true
		}
	}
}

func (p *Parser) Finalize() Outcome {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.finalized {
		p.flushPartialLocked(harnessexecutor.StreamStdout, &p.stdout)
		p.flushPartialLocked(harnessexecutor.StreamStderr, &p.stderr)
		p.finalized = true
	}
	return p.outcome
}

func (p *Parser) flushPartialLocked(stream harnessexecutor.Stream, state *lineState) {
	if state.discarding {
		state.discarding = false
		state.buffer = nil
		return
	}
	if len(state.buffer) > 0 {
		line := append([]byte(nil), state.buffer...)
		state.buffer = nil
		p.observeLineLocked(stream, line)
	}
}

func (p *Parser) Validate(exitCode int) error {
	outcome := p.Finalize()
	if exitCode != 0 {
		return fmt.Errorf("AGY process exited with code %d", exitCode)
	}
	if outcome.PermissionDenied {
		return ErrPermissionDenied
	}
	if !outcome.SawResult {
		return ErrMissingResult
	}
	if !strings.EqualFold(outcome.ResultStatus, "SUCCESS") {
		if outcome.ResultError != "" {
			return fmt.Errorf("%w: %s", ErrResultFailed, outcome.ResultError)
		}
		return fmt.Errorf("%w: status=%s", ErrResultFailed, outcome.ResultStatus)
	}
	return nil
}

func looksLikePermissionDenial(text string) bool {
	v := strings.ToLower(strings.TrimSpace(text))
	if v == "" {
		return false
	}
	markers := []string{
		"soft-denied", "soft denied", "permission denied", "requires approval",
		"approval required", "not allowed by policy", "tool permission",
	}
	for _, marker := range markers {
		if strings.Contains(v, marker) {
			return true
		}
	}
	return false
}
