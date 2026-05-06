package ai

import "context"

//go:generate go tool moq -rm -out provider_moq.go . Provider

type Provider interface {
	Name() string
	Usage(ctx context.Context) (UsageReport, error)
}
