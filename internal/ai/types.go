package ai

import "time"

type UsageWindow struct {
	Label       string     `json:"label"`
	PercentUsed float64    `json:"percent_used"`
	ResetsAt    *time.Time `json:"resets_at,omitempty"`
	Detail      string     `json:"detail,omitempty"`
}

type UsageReport struct {
	Windows []UsageWindow  `json:"windows"`
	Extra   map[string]any `json:"extra,omitempty"`
}

type ProviderResult struct {
	Name   string      `json:"name"`
	OK     bool        `json:"ok"`
	Error  string      `json:"error,omitempty"`
	Hint   string      `json:"hint,omitempty"`
	Report UsageReport `json:"report"`
}

type AggregateReport struct {
	FetchedAt time.Time        `json:"fetched_at"`
	Providers []ProviderResult `json:"providers"`
}
