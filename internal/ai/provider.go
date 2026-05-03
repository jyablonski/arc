package ai

import "context"

type Provider interface {
	Name() string
	Usage(ctx context.Context) (UsageReport, error)
}
