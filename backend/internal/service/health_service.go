package service

import (
	"context"
	"fmt"
	"time"

	"annygo/internal/port"
)

var _ port.HealthChecker = (*HealthService)(nil)

// HealthService implements port.HealthChecker by pinging the database.
type HealthService struct {
	db port.Pinger
}

func NewHealthService(db port.Pinger) *HealthService {
	return &HealthService{db: db}
}

func (s *HealthService) Check(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	if err := s.db.Ping(ctx); err != nil {
		return fmt.Errorf("pinging database: %w", err)
	}

	return nil
}
