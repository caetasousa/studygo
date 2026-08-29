package port

import "context"

// HealthChecker reports whether the service is able to serve traffic.
type HealthChecker interface {
	Check(ctx context.Context) error
}

// Pinger is anything that can verify a live connection — satisfied by
// *pgxpool.Pool.
type Pinger interface {
	Ping(ctx context.Context) error
}
