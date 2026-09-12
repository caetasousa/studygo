package service

import (
	"context"
	"fmt"
	"time"

	"studygo/internal/port"
)

// HealthService responde se o processo pode atender e o que está atendendo.
type HealthService struct {
	db     port.Pinger
	schema port.LeitorDeSchema
	versao string
	deploy string
}

// Saude é o que o /health reporta.
//
// Versão e schema andam juntos de propósito: um rollback troca a versão e
// deixa o schema onde está, e só com os dois lado a lado dá para saber se o
// código que voltou entende o banco que ficou.
type Saude struct {
	Versao string
	Deploy string
	Schema int
}

func NewHealthService(db port.Pinger, schema port.LeitorDeSchema, versao, deploy string) *HealthService {
	return &HealthService{db: db, schema: schema, versao: versao, deploy: deploy}
}

func (s *HealthService) Check(ctx context.Context) (Saude, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	if err := s.db.Ping(ctx); err != nil {
		return Saude{}, fmt.Errorf("pinging database: %w", err)
	}

	schema, err := s.schema.VersaoSchema(ctx)
	if err != nil {
		return Saude{}, fmt.Errorf("lendo a versão do schema: %w", err)
	}

	return Saude{Versao: s.versao, Deploy: s.deploy, Schema: schema}, nil
}
