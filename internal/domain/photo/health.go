package photo

import "context"

type HealthChecker interface {
	HealthCheck(ctx context.Context) error
}
