package claude

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jyablonski/arc/internal/ai"
)

type HistoryProvider struct {
	HomeDir string
}

func (p *HistoryProvider) Name() string { return "claude" }

func (p *HistoryProvider) LocalUsage(ctx context.Context, opts ai.HistoryOptions) ([]ai.TokenRecord, error) {
	home, err := ai.ResolveHomeDir(p.HomeDir)
	if err != nil {
		return nil, err
	}
	paths, err := claudeHistoryPaths(home)
	if err != nil {
		return nil, err
	}
	var out []ai.TokenRecord
	for _, path := range paths {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		out = append(out, parseClaudeHistoryFile(path, opts)...)
	}
	return out, nil
}

func claudeHistoryPaths(home string) ([]string, error) {
	var paths []string
	for _, root := range []string{
		filepath.Join(home, ".claude", "projects"),
		filepath.Join(home, ".claude", "transcripts"),
	} {
		if _, err := os.Stat(root); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.Type().IsRegular() && strings.EqualFold(filepath.Ext(path), ".jsonl") {
				paths = append(paths, path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return paths, nil
}

type claudeHistoryEntry struct {
	Type      string                `json:"type"`
	Timestamp string                `json:"timestamp"`
	RequestID string                `json:"requestId"`
	Message   *claudeHistoryMessage `json:"message"`
}

type claudeHistoryMessage struct {
	Model string              `json:"model"`
	Usage *claudeHistoryUsage `json:"usage"`
	ID    string              `json:"id"`
}

type claudeHistoryUsage struct {
	InputTokens              int64 `json:"input_tokens"`
	OutputTokens             int64 `json:"output_tokens"`
	CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
}

func parseClaudeHistoryFile(path string, opts ai.HistoryOptions) []ai.TokenRecord {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer func() { _ = f.Close() }()

	session := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	var records []ai.TokenRecord
	byDedup := map[string]int{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var entry claudeHistoryEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		if entry.Type != "assistant" || entry.Message == nil || entry.Message.Usage == nil {
			continue
		}
		ts := parseClaudeHistoryTimestamp(entry.Timestamp, path)
		if !opts.Contains(ts) {
			continue
		}
		tokens := ai.TokenBreakdown{
			Input:      max(entry.Message.Usage.InputTokens, 0),
			Output:     max(entry.Message.Usage.OutputTokens, 0),
			CacheRead:  max(entry.Message.Usage.CacheReadInputTokens, 0),
			CacheWrite: max(entry.Message.Usage.CacheCreationInputTokens, 0),
		}
		if tokens.Total() == 0 {
			continue
		}
		record := ai.TokenRecord{
			Provider:  "claude",
			Model:     entry.Message.Model,
			SessionID: session,
			Timestamp: ts,
			Tokens:    tokens,
		}
		if key := claudeDedupKey(entry); key != "" {
			if idx, ok := byDedup[key]; ok {
				records[idx].Tokens = maxBreakdown(records[idx].Tokens, tokens)
				if records[idx].Timestamp.Before(ts) {
					records[idx].Timestamp = ts
				}
				continue
			}
			byDedup[key] = len(records)
		}
		records = append(records, record)
	}
	return records
}

func claudeDedupKey(entry claudeHistoryEntry) string {
	if entry.Message == nil || strings.TrimSpace(entry.Message.ID) == "" {
		return ""
	}
	if strings.TrimSpace(entry.RequestID) != "" {
		return entry.Message.ID + ":" + entry.RequestID
	}
	return "message:" + entry.Message.ID
}

func parseClaudeHistoryTimestamp(raw, path string) time.Time {
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t
	}
	if st, err := os.Stat(path); err == nil {
		return st.ModTime()
	}
	return time.Time{}
}

func maxBreakdown(a, b ai.TokenBreakdown) ai.TokenBreakdown {
	return ai.TokenBreakdown{
		Input:      max(a.Input, b.Input),
		Output:     max(a.Output, b.Output),
		CacheRead:  max(a.CacheRead, b.CacheRead),
		CacheWrite: max(a.CacheWrite, b.CacheWrite),
		Reasoning:  max(a.Reasoning, b.Reasoning),
	}
}
