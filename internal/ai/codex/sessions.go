package codex

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/jyablonski/arc/internal/ai"
)

// Sessions lists local Codex sessions. It reuses LocalUsage for the token math
// (including the cumulative-delta accounting) and a lightweight metadata pass
// for the working directory, resume id, and first prompt. Fully offline.
func (p *HistoryProvider) Sessions(ctx context.Context) ([]ai.SessionSummary, error) {
	home, err := ai.ResolveHomeDir(p.HomeDir)
	if err != nil {
		return nil, err
	}
	root := filepath.Join(home, ".codex", "sessions")
	records, err := p.LocalUsage(ctx, ai.HistoryOptions{})
	if err != nil {
		return nil, err
	}

	byID := ai.FoldSessions(records)
	if _, err := os.Stat(root); err == nil {
		walkErr := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			if !d.Type().IsRegular() || !strings.EqualFold(filepath.Ext(path), ".jsonl") {
				return nil
			}
			id := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
			s := byID[id]
			if s == nil {
				return nil // no token usage recorded for this session
			}
			meta := parseCodexSessionMeta(path)
			if s.Project == "" {
				s.Project = meta.Project
			}
			s.ResumeID = meta.ResumeID // Codex resumes by UUID: codex resume <id>
			s.Title = meta.FirstPrompt
			return nil
		})
		if walkErr != nil {
			return nil, walkErr
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	out := make([]ai.SessionSummary, 0, len(byID))
	for _, s := range byID {
		out = append(out, *s)
	}
	return out, nil
}

type codexSessionMeta struct {
	Project     string
	ResumeID    string
	FirstPrompt string
}

type codexMetaEntry struct {
	Type    string `json:"type"`
	Payload *struct {
		Type    string `json:"type"`
		ID      string `json:"id"`
		Cwd     string `json:"cwd"`
		Message string `json:"message"`
	} `json:"payload"`
}

// parseCodexSessionMeta scans a rollout file once for the session's working
// directory and UUID (from session_meta) and the first real user prompt (from
// an event_msg user_message, skipping the injected environment_context turn).
func parseCodexSessionMeta(path string) codexSessionMeta {
	f, err := os.Open(path)
	if err != nil {
		return codexSessionMeta{}
	}
	defer func() { _ = f.Close() }()

	var meta codexSessionMeta
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var e codexMetaEntry
		if err := json.Unmarshal([]byte(line), &e); err != nil || e.Payload == nil {
			continue
		}
		if e.Type == "session_meta" {
			if meta.Project == "" {
				meta.Project = e.Payload.Cwd
			}
			if meta.ResumeID == "" {
				meta.ResumeID = strings.TrimSpace(e.Payload.ID)
			}
		}
		if meta.FirstPrompt == "" && e.Payload.Type == "user_message" {
			msg := strings.TrimSpace(e.Payload.Message)
			if msg != "" && !strings.Contains(msg, "<environment_context>") {
				meta.FirstPrompt = ai.FirstLine(msg)
			}
		}
		if meta.Project != "" && meta.ResumeID != "" && meta.FirstPrompt != "" {
			break
		}
	}
	return meta
}
