package cursor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jyablonski/arc/internal/ai"
)

const dashboardAPIBase = "https://api2.cursor.sh"

type planUsage struct {
	TotalSpend       *float64 `json:"totalSpend"`
	IncludedSpend    *float64 `json:"includedSpend"`
	BonusSpend       *float64 `json:"bonusSpend"`
	Remaining        *float64 `json:"remaining"`
	Limit            *float64 `json:"limit"`
	TotalPercentUsed *float64 `json:"totalPercentUsed"`
	AutoPercentUsed  *float64 `json:"autoPercentUsed"`
	ApiPercentUsed   *float64 `json:"apiPercentUsed"`
}

type spendLimitUsage struct {
	LimitType           *string  `json:"limitType"`
	PooledLimit         *float64 `json:"pooledLimit"`
	PooledUsed          *float64 `json:"pooledUsed"`
	PooledRemaining     *float64 `json:"pooledRemaining"`
	IndividualLimit     *float64 `json:"individualLimit"`
	IndividualUsed      *float64 `json:"individualUsed"`
	IndividualRemaining *float64 `json:"individualRemaining"`
}

type dashboardUsageEnvelope struct {
	Enabled           *bool            `json:"enabled"`
	BillingCycleStart string           `json:"billingCycleStart"`
	BillingCycleEnd   string           `json:"billingCycleEnd"`
	PlanUsage         *planUsage       `json:"planUsage"`
	SpendLimitUsage   *spendLimitUsage `json:"spendLimitUsage"`
}

type planInfoEnvelope struct {
	PlanInfo *struct {
		PlanName            string `json:"planName"`
		IncludedAmountCents int    `json:"includedAmountCents"`
		Price               string `json:"price"`
		BillingCycleEnd     string `json:"billingCycleEnd"`
	} `json:"planInfo"`
}

func connectPOST(ctx context.Context, client *http.Client, jwt, rpcPath string, body []byte) (int, []byte, error) {
	url := dashboardAPIBase + rpcPath
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(jwt))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Connect-Protocol-Version", "1")
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("POST %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b, nil
}

func finitePositive(p *float64) bool {
	return p != nil && !math.IsNaN(*p) && !math.IsInf(*p, 0) && *p > 0
}

func finiteNumber(p *float64) bool {
	return p != nil && !math.IsNaN(*p) && !math.IsInf(*p, 0)
}

func parseBillingInstant(msStr string) *time.Time {
	s := strings.TrimSpace(msStr)
	if s == "" {
		return nil
	}
	ms, err := strconv.ParseInt(s, 10, 64)
	if err != nil || ms <= 0 {
		return nil
	}
	ts := time.UnixMilli(ms).UTC()
	return &ts
}

func teamSignals(u *dashboardUsageEnvelope, planLower string) bool {
	if strings.EqualFold(planLower, "team") {
		return true
	}
	su := u.SpendLimitUsage
	if su == nil {
		return false
	}
	if su.LimitType != nil && strings.EqualFold(*su.LimitType, "team") {
		return true
	}
	return finitePositive(su.PooledLimit)
}

func shouldUseEnterpriseTeamREST(u *dashboardUsageEnvelope, planLower string) bool {
	enabled := u.Enabled == nil || *u.Enabled
	pu := u.PlanUsage
	hasPU := pu != nil
	limitPresent := hasPU && finitePositive(pu.Limit)
	limitMissing := hasPU && !limitPresent

	enterprise := strings.EqualFold(planLower, "enterprise")
	teamNamed := strings.EqualFold(planLower, "team")
	return enabled && (!hasPU || limitMissing) && (enterprise || teamNamed)
}

func shouldUseUnknownREST(u *dashboardUsageEnvelope, planLower string, planInfoFailed bool) bool {
	enabled := u.Enabled == nil || *u.Enabled
	pu := u.PlanUsage
	limitMissing := pu == nil || !finitePositive(pu.Limit)
	hasTotalPct := pu != nil && finiteNumber(pu.TotalPercentUsed)

	return enabled && limitMissing && !hasTotalPct && strings.TrimSpace(planLower) == "" && planInfoFailed
}

func shouldUseRESTLimitMissingNoTotal(u *dashboardUsageEnvelope, planLower string) bool {
	enabled := u.Enabled == nil || *u.Enabled
	pu := u.PlanUsage
	if !enabled || pu == nil {
		return false
	}
	if finitePositive(pu.Limit) || finiteNumber(pu.TotalPercentUsed) {
		return false
	}
	return strings.TrimSpace(planLower) != ""
}

func shouldUseRESTTeamNoLimit(u *dashboardUsageEnvelope, planLower string) bool {
	if !teamSignals(u, planLower) || strings.EqualFold(planLower, "enterprise") {
		return false
	}
	pu := u.PlanUsage
	if pu == nil {
		return true
	}
	return !finitePositive(pu.Limit)
}

func reportFromDashboard(u *dashboardUsageEnvelope, planLower string, planLabel string) (ai.UsageReport, error) {
	if u.Enabled != nil && !*u.Enabled {
		return ai.UsageReport{}, fmt.Errorf("no active Cursor subscription (dashboard disabled)")
	}
	pu := u.PlanUsage
	if pu == nil {
		return ai.UsageReport{}, fmt.Errorf("cursor dashboard: missing planUsage")
	}

	reset := parseBillingInstant(u.BillingCycleEnd)
	extra := map[string]any{}
	if pc := parseBillingInstant(u.BillingCycleStart); pc != nil {
		extra["billing_cycle_start"] = pc.Format(time.RFC3339)
	}
	if reset != nil {
		extra["billing_cycle_end"] = reset.Format(time.RFC3339)
	}
	if strings.TrimSpace(planLabel) != "" {
		extra["plan"] = planLabel
	}

	hasLimit := finitePositive(pu.Limit)
	hasTotalPct := finiteNumber(pu.TotalPercentUsed)
	if !hasLimit && !hasTotalPct {
		return ai.UsageReport{}, fmt.Errorf("cursor dashboard: planUsage has neither limit nor totalPercentUsed")
	}

	var planUsed float64
	if hasLimit && pu.TotalSpend != nil && finiteNumber(pu.TotalSpend) {
		planUsed = *pu.TotalSpend
	} else if hasLimit && pu.Remaining != nil && pu.Limit != nil {
		planUsed = *pu.Limit - *pu.Remaining
	}

	var totalPct float64
	if hasTotalPct {
		totalPct = *pu.TotalPercentUsed
	} else if hasLimit && *pu.Limit > 0 {
		totalPct = 100 * planUsed / *pu.Limit
	}

	teamLike := teamSignals(u, planLower) && finitePositive(pu.Limit)

	var windows []ai.UsageWindow
	if teamLike {
		usdUsed := centsToUSD(planUsed)
		usdLimit := centsToUSD(*pu.Limit)
		detail := fmt.Sprintf("$%.2f / $%.2f included", usdUsed, usdLimit)
		windows = append(windows, ai.UsageWindow{
			Label:       "Total usage (included)",
			PercentUsed: totalPct,
			ResetsAt:    reset,
			Detail:      detail,
		})
		if finiteNumber(pu.BonusSpend) && *pu.BonusSpend > 0 {
			windows = append(windows, ai.UsageWindow{
				Label:  "Bonus spend",
				Detail: fmt.Sprintf("$%.2f credits", centsToUSD(*pu.BonusSpend)),
			})
		}
	} else {
		windows = append(windows, ai.UsageWindow{
			Label:       "Total",
			PercentUsed: totalPct,
			ResetsAt:    reset,
			Detail:      dashboardSpendDetail(pu),
		})
		if pu.AutoPercentUsed != nil && finiteNumber(pu.AutoPercentUsed) {
			windows = append(windows, ai.UsageWindow{
				Label:       "Auto + Composer",
				PercentUsed: *pu.AutoPercentUsed,
				ResetsAt:    reset,
				Detail:      "% of included Auto/Composer pool (see dashboard)",
			})
		}
		if pu.ApiPercentUsed != nil && finiteNumber(pu.ApiPercentUsed) {
			windows = append(windows, ai.UsageWindow{
				Label:       "API",
				PercentUsed: *pu.ApiPercentUsed,
				ResetsAt:    reset,
				Detail:      "% of included API pool",
			})
		}
	}

	appendOnDemandWindows(&windows, u.SpendLimitUsage)

	sortWindows(windows)
	return ai.UsageReport{Windows: windows, Extra: extra}, nil
}

func centsToUSD(c float64) float64 {
	return c / 100
}

func dashboardSpendDetail(pu *planUsage) string {
	var parts []string
	if pu.IncludedSpend != nil && finiteNumber(pu.IncludedSpend) {
		parts = append(parts, fmt.Sprintf("included spend $%.2f", centsToUSD(*pu.IncludedSpend)))
	}
	if pu.Limit != nil && finiteNumber(pu.Limit) {
		parts = append(parts, fmt.Sprintf("limit $%.2f", centsToUSD(*pu.Limit)))
	}
	if pu.Remaining != nil && finiteNumber(pu.Remaining) {
		parts = append(parts, fmt.Sprintf("remaining $%.2f", centsToUSD(*pu.Remaining)))
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " · ")
}

func appendOnDemandWindows(w *[]ai.UsageWindow, su *spendLimitUsage) {
	if su == nil {
		return
	}
	limit := 0.0
	remaining := 0.0
	if finitePositive(su.IndividualLimit) {
		limit = *su.IndividualLimit
		if su.IndividualRemaining != nil {
			remaining = *su.IndividualRemaining
		}
	} else if finitePositive(su.PooledLimit) {
		limit = *su.PooledLimit
		if su.PooledRemaining != nil {
			remaining = *su.PooledRemaining
		}
	}
	if limit <= 0 {
		return
	}
	used := limit - remaining
	if used < 0 {
		used = 0
	}
	*w = append(*w, ai.UsageWindow{
		Label:       "On-demand budget",
		PercentUsed: 100 * used / limit,
		Detail:      fmt.Sprintf("$%.2f / $%.2f", centsToUSD(used), centsToUSD(limit)),
	})
}

func fetchDashboardAndPlan(ctx context.Context, client *http.Client, jwt string) (*dashboardUsageEnvelope, string, bool, error) {
	st, dashBody, err := connectPOST(ctx, client, jwt, "/aiserver.v1.DashboardService/GetCurrentPeriodUsage", []byte("{}"))
	if err != nil {
		return nil, "", false, err
	}
	if st < 200 || st >= 300 {
		return nil, "", false, fmt.Errorf("GetCurrentPeriodUsage: HTTP %d — %s", st, truncate(string(dashBody), 400))
	}
	var env dashboardUsageEnvelope
	if err := json.Unmarshal(dashBody, &env); err != nil {
		return nil, "", false, fmt.Errorf("decode dashboard usage: %w", err)
	}

	var planLabel string
	planOK := false
	pst, planBody, perr := connectPOST(ctx, client, jwt, "/aiserver.v1.DashboardService/GetPlanInfo", []byte("{}"))
	if perr == nil && pst >= 200 && pst < 300 {
		var pinfo planInfoEnvelope
		if err := json.Unmarshal(planBody, &pinfo); err == nil && pinfo.PlanInfo != nil {
			planLabel = strings.TrimSpace(pinfo.PlanInfo.PlanName)
			planOK = planLabel != ""
		}
	}

	return &env, planLabel, planOK, nil
}
