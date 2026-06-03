package codex

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

func (p *HistoryProvider) Name() string { return "codex" }

func (p *HistoryProvider) LocalUsage(ctx context.Context, opts ai.HistoryOptions) ([]ai.TokenRecord, error) {
	home, err := ai.ResolveHomeDir(p.HomeDir)
	if err != nil {
		return nil, err
	}
	root := filepath.Join(home, ".codex", "sessions")
	if _, err := os.Stat(root); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []ai.TokenRecord
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if d.Type().IsRegular() && strings.EqualFold(filepath.Ext(path), ".jsonl") {
			out = append(out, parseCodexHistoryFile(path, opts)...)
		}
		return nil
	})
	return out, err
}

type codexHistoryEntry struct {
	Type      string               `json:"type"`
	Timestamp string               `json:"timestamp"`
	Payload   *codexHistoryPayload `json:"payload"`
}

type codexHistoryPayload struct {
	Type          string          `json:"type"`
	Model         string          `json:"model"`
	ModelName     string          `json:"model_name"`
	ModelInfo     *codexModelInfo `json:"model_info"`
	ModelProvider string          `json:"model_provider"`
	Info          *codexInfo      `json:"info"`
}

type codexModelInfo struct {
	Slug string `json:"slug"`
}

type codexInfo struct {
	Model           string           `json:"model"`
	ModelName       string           `json:"model_name"`
	LastTokenUsage  *codexTokenUsage `json:"last_token_usage"`
	TotalTokenUsage *codexTokenUsage `json:"total_token_usage"`
}

type codexTokenUsage struct {
	InputTokens           int64 `json:"input_tokens"`
	OutputTokens          int64 `json:"output_tokens"`
	CachedInputTokens     int64 `json:"cached_input_tokens"`
	CacheReadInputTokens  int64 `json:"cache_read_input_tokens"`
	ReasoningOutputTokens int64 `json:"reasoning_output_tokens"`
	TotalTokens           int64 `json:"total_tokens"`
}

type codexTotals struct {
	Input     int64
	Output    int64
	Cached    int64
	Reasoning int64
}

func parseCodexHistoryFile(path string, opts ai.HistoryOptions) []ai.TokenRecord {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer func() { _ = f.Close() }()

	session := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	fallback := fileModTime(path)
	state := codexHistoryState{}
	var records []ai.TokenRecord
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var entry codexHistoryEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil || entry.Payload == nil {
			continue
		}
		payload := entry.Payload
		model := codexModelFromPayload(payload)
		if model != "" {
			state.CurrentModel = model
		}
		if entry.Type != "event_msg" || payload.Type != "token_count" || payload.Info == nil {
			continue
		}
		infoModel := codexModelFromInfo(payload.Info)
		if infoModel != "" {
			state.CurrentModel = infoModel
		}
		tokens, nextTotals, ok := state.tokensFromInfo(payload.Info)
		if !ok || tokens.Total() == 0 {
			continue
		}
		state.PreviousTotals = nextTotals
		ts := parseCodexHistoryTimestamp(entry.Timestamp, fallback)
		if !opts.Contains(ts) {
			continue
		}
		records = append(records, ai.TokenRecord{
			Provider:  "codex",
			Model:     firstNonEmpty(state.CurrentModel, "unknown"),
			SessionID: session,
			Timestamp: ts,
			Tokens:    tokens,
		})
	}
	return records
}

type codexHistoryState struct {
	CurrentModel   string
	PreviousTotals *codexTotals
}

func (s *codexHistoryState) tokensFromInfo(info *codexInfo) (ai.TokenBreakdown, *codexTotals, bool) {
	total := totalsFromUsage(info.TotalTokenUsage)
	last := totalsFromUsage(info.LastTokenUsage)

	switch {
	case total != nil && last != nil:
		if s.PreviousTotals != nil && *total == *s.PreviousTotals {
			return ai.TokenBreakdown{}, s.PreviousTotals, false
		}
		return last.intoTokens(), total, true
	case total != nil && s.PreviousTotals != nil:
		delta, ok := total.deltaFrom(*s.PreviousTotals)
		if !ok {
			return ai.TokenBreakdown{}, total, false
		}
		return delta.intoTokens(), total, true
	case total != nil:
		return total.intoTokens(), total, true
	case last != nil:
		next := last
		if s.PreviousTotals != nil {
			sum := s.PreviousTotals.add(*last)
			next = &sum
		}
		return last.intoTokens(), next, true
	default:
		return ai.TokenBreakdown{}, s.PreviousTotals, false
	}
}

func totalsFromUsage(u *codexTokenUsage) *codexTotals {
	if u == nil {
		return nil
	}
	cached := max(u.CachedInputTokens, u.CacheReadInputTokens)
	return &codexTotals{
		Input:     max(u.InputTokens, 0),
		Output:    max(u.OutputTokens, 0),
		Cached:    max(cached, 0),
		Reasoning: max(u.ReasoningOutputTokens, 0),
	}
}

func (t codexTotals) deltaFrom(previous codexTotals) (codexTotals, bool) {
	if t.Input < previous.Input || t.Output < previous.Output || t.Cached < previous.Cached || t.Reasoning < previous.Reasoning {
		return codexTotals{}, false
	}
	return codexTotals{
		Input:     t.Input - previous.Input,
		Output:    t.Output - previous.Output,
		Cached:    t.Cached - previous.Cached,
		Reasoning: t.Reasoning - previous.Reasoning,
	}, true
}

func (t codexTotals) add(other codexTotals) codexTotals {
	return codexTotals{
		Input:     t.Input + other.Input,
		Output:    t.Output + other.Output,
		Cached:    t.Cached + other.Cached,
		Reasoning: t.Reasoning + other.Reasoning,
	}
}

func (t codexTotals) intoTokens() ai.TokenBreakdown {
	cached := t.Cached
	if cached > t.Input {
		cached = t.Input
	}
	return ai.TokenBreakdown{
		Input:     max(t.Input-cached, 0),
		Output:    max(t.Output, 0),
		CacheRead: max(cached, 0),
		Reasoning: max(t.Reasoning, 0),
	}
}

func codexModelFromPayload(p *codexHistoryPayload) string {
	if p == nil {
		return ""
	}
	if p.ModelInfo != nil && strings.TrimSpace(p.ModelInfo.Slug) != "" {
		return p.ModelInfo.Slug
	}
	return firstNonEmpty(p.Model, p.ModelName)
}

func codexModelFromInfo(info *codexInfo) string {
	if info == nil {
		return ""
	}
	return firstNonEmpty(info.Model, info.ModelName)
}

func parseCodexHistoryTimestamp(raw string, fallback time.Time) time.Time {
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t
	}
	return fallback
}

func fileModTime(path string) time.Time {
	if st, err := os.Stat(path); err == nil {
		return st.ModTime()
	}
	return time.Time{}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
