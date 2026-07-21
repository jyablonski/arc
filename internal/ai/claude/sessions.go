package claude

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/jyablonski/arc/internal/ai"
)

// Sessions lists local Claude Code sessions. It reuses LocalUsage for the token
// math and a lightweight metadata pass for the fields the token path ignores
// (project cwd, session title, first prompt). Fully offline.
func (p *HistoryProvider) Sessions(ctx context.Context) ([]ai.SessionSummary, error) {
	home, err := ai.ResolveHomeDir(p.HomeDir)
	if err != nil {
		return nil, err
	}
	paths, err := claudeHistoryPaths(home)
	if err != nil {
		return nil, err
	}
	records, err := p.LocalUsage(ctx, ai.HistoryOptions{})
	if err != nil {
		return nil, err
	}

	byID := ai.FoldSessions(records)
	for _, path := range paths {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		id := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		s := byID[id]
		if s == nil {
			continue // no assistant token usage in this file; nothing to show
		}
		meta := parseClaudeSessionMeta(path)
		if s.Project == "" {
			s.Project = meta.Project
		}
		s.ResumeID = id // Claude resumes by session id: claude --resume <id>
		title := meta.Title
		if title == "" {
			title = meta.FirstPrompt
		}
		s.Title = title
	}

	out := make([]ai.SessionSummary, 0, len(byID))
	for _, s := range byID {
		out = append(out, *s)
	}
	return out, nil
}

type claudeSessionMeta struct {
	Project     string
	Title       string
	FirstPrompt string
}

type claudeMetaEntry struct {
	Type    string `json:"type"`
	Cwd     string `json:"cwd"`
	AiTitle string `json:"aiTitle"`
	Message *struct {
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

// parseClaudeSessionMeta scans a session file once for the display metadata not
// carried by token records: the working directory, the AI-generated title, and
// the first real user prompt (string content only; list content is tool output).
func parseClaudeSessionMeta(path string) claudeSessionMeta {
	f, err := os.Open(path)
	if err != nil {
		return claudeSessionMeta{}
	}
	defer func() { _ = f.Close() }()

	var meta claudeSessionMeta
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var e claudeMetaEntry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			continue
		}
		if meta.Project == "" && e.Cwd != "" {
			meta.Project = e.Cwd
		}
		if e.Type == "ai-title" && strings.TrimSpace(e.AiTitle) != "" {
			meta.Title = strings.TrimSpace(e.AiTitle)
		}
		if meta.FirstPrompt == "" && e.Type == "user" && e.Message != nil {
			var content string
			if json.Unmarshal(e.Message.Content, &content) == nil && strings.TrimSpace(content) != "" {
				meta.FirstPrompt = ai.FirstLine(content)
			}
		}
		if meta.Project != "" && meta.Title != "" && meta.FirstPrompt != "" {
			break
		}
	}
	return meta
}
