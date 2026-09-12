package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"studygo/internal/service"
)

// O /health é lido por gente, não pelo frontend: é o `make health` que diz qual
// publicação está no ar e em que migration o banco parou. Estes testes prendem
// esses campos, que o snapshot de contrato não cobre.

type pingerFake struct{ err error }

func (p pingerFake) Ping(context.Context) error { return p.err }

type schemaFake struct {
	versao int
	err    error
}

func (s schemaFake) VersaoSchema(context.Context) (int, error) { return s.versao, s.err }

func chamarHealth(t *testing.T, svc *service.HealthService) (int, map[string]any) {
	t.Helper()

	rec := httptest.NewRecorder()
	NewHealthHandler(svc, quietoHTTP()).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))

	var corpo map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &corpo); err != nil {
		t.Fatalf("corpo não é JSON: %v — %s", err, rec.Body.String())
	}

	return rec.Code, corpo
}

func TestHealth_dizQualVersaoEstaNoAr(t *testing.T) {
	t.Parallel()

	svc := service.NewHealthService(pingerFake{}, schemaFake{versao: 3}, "v2026.09.12", "1234567")

	codigo, corpo := chamarHealth(t, svc)

	if codigo != http.StatusOK {
		t.Fatalf("status = %d, quer 200", codigo)
	}

	quer := map[string]any{"status": "ok", "versao": "v2026.09.12", "deploy": "1234567", "schema": 3.0}
	for campo, valor := range quer {
		if corpo[campo] != valor {
			t.Errorf("%s = %v, quer %v", campo, corpo[campo], valor)
		}
	}
}

// Fora da pipeline não há deploy: o campo some em vez de aparecer vazio.
func TestHealth_semDeployOmiteOCampo(t *testing.T) {
	t.Parallel()

	svc := service.NewHealthService(pingerFake{}, schemaFake{versao: 3}, "dev", "")

	_, corpo := chamarHealth(t, svc)

	if _, tem := corpo["deploy"]; tem {
		t.Errorf("deploy apareceu fora da pipeline: %v", corpo)
	}
}

// Ler o schema é uma ida ao banco como o ping. Se falhar, o processo não está
// saudável — e é o 503 que faz o deploy.yml devolver a versão anterior.
func TestHealth_falhaAoLerOSchemaDerrubaOHealth(t *testing.T) {
	t.Parallel()

	svc := service.NewHealthService(pingerFake{}, schemaFake{err: errors.New("relation does not exist")}, "v1", "1")

	codigo, corpo := chamarHealth(t, svc)

	if codigo != http.StatusServiceUnavailable {
		t.Errorf("status = %d, quer 503", codigo)
	}

	if corpo["status"] != "unavailable" {
		t.Errorf("status = %v, quer unavailable", corpo["status"])
	}
}
