package codex

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/jyablonski/arc/internal/ai"
)

type Provider struct {
	CodexBinary string
	HomeDir     string // overrides ~ for offline health checks (tests)
}

func (p *Provider) Name() string { return "codex" }

func (p *Provider) Usage(ctx context.Context) (ai.UsageReport, error) {
	bin := p.CodexBinary
	if bin == "" {
		bin = "codex"
	}
	cmd := exec.CommandContext(ctx, bin, "app-server")
	cmd.Stderr = nil

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return ai.UsageReport{}, fmt.Errorf("codex app-server stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return ai.UsageReport{}, fmt.Errorf("codex app-server stdout: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return ai.UsageReport{}, fmt.Errorf("start codex app-server: %w — is Codex installed? (npm i -g @openai/codex)", err)
	}

	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	nextID := 1
	write := func(method string, params map[string]any) error {
		id := nextID
		nextID++
		msg := map[string]any{
			"jsonrpc": "2.0",
			"id":      id,
			"method":  method,
		}
		if params != nil {
			msg["params"] = params
		}
		b, err := json.Marshal(msg)
		if err != nil {
			return err
		}
		if _, err := stdin.Write(append(b, '\n')); err != nil {
			return err
		}
		return nil
	}

	readResult := func(wantID int) (json.RawMessage, error) {
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line == "" {
				continue
			}
			var envelope struct {
				ID     int             `json:"id"`
				Result json.RawMessage `json:"result"`
				Error  *struct {
					Message string `json:"message"`
				} `json:"error"`
			}
			if err := json.Unmarshal([]byte(line), &envelope); err != nil {
				continue
			}
			if envelope.ID != wantID {
				continue
			}
			if envelope.Error != nil {
				return nil, fmt.Errorf("rpc error: %s", envelope.Error.Message)
			}
			return envelope.Result, nil
		}
		if err := sc.Err(); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("no JSON-RPC response for id %d", wantID)
	}

	initID := nextID
	if err := write("initialize", map[string]any{
		"clientInfo": map[string]string{"name": "arc", "version": "1.0.0"},
	}); err != nil {
		_ = cmd.Process.Kill()
		return ai.UsageReport{}, err
	}
	if _, err := readResult(initID); err != nil {
		_ = stdin.Close()
		_ = cmd.Process.Kill()
		return ai.UsageReport{}, err
	}

	rlID := nextID
	if err := write("account/rateLimits/read", nil); err != nil {
		_ = cmd.Process.Kill()
		return ai.UsageReport{}, err
	}
	rawRes, err := readResult(rlID)
	_ = stdin.Close()
	_ = cmd.Process.Kill()
	_ = cmd.Wait()

	if err != nil {
		return ai.UsageReport{}, err
	}
	return decodeRateLimitsResult(rawRes)
}

type rateWindow struct {
	UsedPercent        float64 `json:"usedPercent"`
	WindowDurationMins int     `json:"windowDurationMins"`
	ResetsAt           int64   `json:"resetsAt"`
}

type rateLimitBucket struct {
	LimitID              string      `json:"limitId"`
	Primary              *rateWindow `json:"primary"`
	Secondary            *rateWindow `json:"secondary"`
	RateLimitReachedType *string     `json:"rateLimitReachedType"`
}

type rateLimitsReadResult struct {
	RateLimits          *rateLimitBucket           `json:"rateLimits"`
	RateLimitsByLimitID map[string]rateLimitBucket `json:"rateLimitsByLimitId"`
}

func decodeRateLimitsResult(raw json.RawMessage) (ai.UsageReport, error) {
	var res rateLimitsReadResult
	if err := json.Unmarshal(raw, &res); err != nil {
		return ai.UsageReport{}, fmt.Errorf("decode rate limits: %w", err)
	}

	var windows []ai.UsageWindow
	extra := map[string]any{}

	if b, ok := codexRateLimitBucket(res); ok {
		windows = append(windows, windowsFromBucket(b)...)
	}

	if res.RateLimits != nil && res.RateLimits.RateLimitReachedType != nil {
		extra["rate_limit_reached_type"] = *res.RateLimits.RateLimitReachedType
	}

	return ai.UsageReport{Windows: windows, Extra: extra}, nil
}

func codexRateLimitBucket(res rateLimitsReadResult) (rateLimitBucket, bool) {
	if b, ok := res.RateLimitsByLimitID["codex"]; ok {
		return b, true
	}
	if res.RateLimits != nil && (res.RateLimits.LimitID == "" || res.RateLimits.LimitID == "codex") {
		return *res.RateLimits, true
	}
	return rateLimitBucket{}, false
}

func windowsFromBucket(b rateLimitBucket) []ai.UsageWindow {
	var out []ai.UsageWindow
	if b.Primary != nil {
		t := time.Unix(b.Primary.ResetsAt, 0).UTC()
		out = append(out, ai.UsageWindow{
			Label:       rateWindowLabel(b.Primary.WindowDurationMins, "5 hour"),
			PercentUsed: b.Primary.UsedPercent,
			ResetsAt:    &t,
		})
	}
	if b.Secondary != nil {
		t := time.Unix(b.Secondary.ResetsAt, 0).UTC()
		out = append(out, ai.UsageWindow{
			Label:       rateWindowLabel(b.Secondary.WindowDurationMins, "weekly"),
			PercentUsed: b.Secondary.UsedPercent,
			ResetsAt:    &t,
		})
	}
	return out
}

func rateWindowLabel(minutes int, fallback string) string {
	switch minutes {
	case 300:
		return "5 hour"
	case 10080:
		return "weekly"
	case 0:
		return fallback
	default:
		switch {
		case minutes%1440 == 0:
			return fmt.Sprintf("%d day", minutes/1440)
		case minutes%60 == 0:
			return fmt.Sprintf("%d hour", minutes/60)
		default:
			return fmt.Sprintf("%d min", minutes)
		}
	}
}
