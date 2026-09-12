package port

import "context"

// Pinger is anything that can verify a live connection — satisfied by
// *pgxpool.Pool.
type Pinger interface {
	Ping(ctx context.Context) error
}

// LeitorDeSchema informa a última migration aplicada no banco.
type LeitorDeSchema interface {
	VersaoSchema(ctx context.Context) (int, error)
}
