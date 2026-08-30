package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"annygo/internal/port"
)

// The Files API lets a big PDF be uploaded once and referenced by URI on every
// later call, instead of re-sending megabytes of base64 on each wizard step.
// Uploaded files expire on Google's side after ~48h, which suits a wizard that
// finishes in minutes.
const (
	uploadEndpoint = "https://generativelanguage.googleapis.com/upload/v1beta/files?key=%s"
	filesEndpoint  = "https://generativelanguage.googleapis.com/v1beta/%s?key=%s"
)

type arquivoRemoto struct {
	Nome string // "files/abc123"
	URI  string
	MIME string
}

// enviarArquivo uploads dados with the resumable protocol and waits until the
// file is ACTIVE (usable in generateContent).
func (g *GeminiAnalisador) enviarArquivo(ctx context.Context, dados []byte, mime string) (arquivoRemoto, error) {
	if mime == "" {
		mime = "application/pdf"
	}

	uploadURL, err := g.iniciarUpload(ctx, len(dados), mime)
	if err != nil {
		return arquivoRemoto{}, err
	}

	arq, err := g.finalizarUpload(ctx, uploadURL, dados, mime)
	if err != nil {
		return arquivoRemoto{}, err
	}

	if err := g.aguardarAtivo(ctx, arq.Nome); err != nil {
		return arquivoRemoto{}, err
	}

	return arq, nil
}

func (g *GeminiAnalisador) iniciarUpload(ctx context.Context, tamanho int, mime string) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"file": map[string]any{"display_name": "edital"},
	})

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		fmt.Sprintf(g.uploadURL, g.apiKey),
		bytes.NewReader(body),
	)
	if err != nil {
		return "", fmt.Errorf("building upload start: %w", err)
	}

	req.Header.Set("X-Goog-Upload-Protocol", "resumable")
	req.Header.Set("X-Goog-Upload-Command", "start")
	req.Header.Set("X-Goog-Upload-Header-Content-Length", fmt.Sprint(tamanho))
	req.Header.Set("X-Goog-Upload-Header-Content-Type", mime)
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: iniciando upload: %w", port.ErrProvedorIndisponivel, err)
	}
	defer resp.Body.Close()

	payload, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	if resp.StatusCode != http.StatusOK {
		if culpaDoProvedor(resp.StatusCode) {
			return "", fmt.Errorf("%w: upload start respondeu %d: %s", port.ErrProvedorIndisponivel, resp.StatusCode, snippet(payload))
		}

		return "", fmt.Errorf("upload start respondeu %d: %s", resp.StatusCode, snippet(payload))
	}

	url := resp.Header.Get("X-Goog-Upload-URL")
	if url == "" {
		return "", fmt.Errorf("upload start não devolveu a URL de envio")
	}

	return url, nil
}

func (g *GeminiAnalisador) finalizarUpload(
	ctx context.Context,
	url string,
	dados []byte,
	mime string,
) (arquivoRemoto, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(dados))
	if err != nil {
		return arquivoRemoto{}, fmt.Errorf("building upload finalize: %w", err)
	}

	req.ContentLength = int64(len(dados))
	req.Header.Set("X-Goog-Upload-Offset", "0")
	req.Header.Set("X-Goog-Upload-Command", "upload, finalize")

	resp, err := g.http.Do(req)
	if err != nil {
		return arquivoRemoto{}, fmt.Errorf("%w: enviando arquivo: %w", port.ErrProvedorIndisponivel, err)
	}
	defer resp.Body.Close()

	payload, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	if resp.StatusCode != http.StatusOK {
		if culpaDoProvedor(resp.StatusCode) {
			return arquivoRemoto{}, fmt.Errorf("%w: upload respondeu %d: %s", port.ErrProvedorIndisponivel, resp.StatusCode, snippet(payload))
		}

		return arquivoRemoto{}, fmt.Errorf("upload respondeu %d: %s", resp.StatusCode, snippet(payload))
	}

	var parsed struct {
		File struct {
			Name     string `json:"name"`
			URI      string `json:"uri"`
			MimeType string `json:"mimeType"`
			State    string `json:"state"`
		} `json:"file"`
	}

	if err := json.Unmarshal(payload, &parsed); err != nil {
		return arquivoRemoto{}, fmt.Errorf("decodificando upload: %w", err)
	}

	if parsed.File.URI == "" {
		return arquivoRemoto{}, fmt.Errorf("upload não devolveu uri: %s", snippet(payload))
	}

	if parsed.File.MimeType == "" {
		parsed.File.MimeType = mime
	}

	return arquivoRemoto{Nome: parsed.File.Name, URI: parsed.File.URI, MIME: parsed.File.MimeType}, nil
}

// aguardarAtivo polls files.get until the upload finishes processing.
func (g *GeminiAnalisador) aguardarAtivo(ctx context.Context, nome string) error {
	if nome == "" {
		return nil
	}

	url := fmt.Sprintf(g.filesURL, nome, g.apiKey)

	for range 30 {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return fmt.Errorf("building files.get: %w", err)
		}

		resp, err := g.http.Do(req)
		if err != nil {
			return fmt.Errorf("%w: consultando arquivo: %w", port.ErrProvedorIndisponivel, err)
		}

		payload, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()

		var parsed struct {
			State string `json:"state"`
		}
		_ = json.Unmarshal(payload, &parsed)

		switch strings.ToUpper(parsed.State) {
		case "ACTIVE":
			return nil
		case "FAILED":
			return fmt.Errorf("o Gemini não conseguiu processar o arquivo")
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("%w: %w", port.ErrProvedorIndisponivel, ctx.Err())
		case <-time.After(g.pollInterval):
		}
	}

	return fmt.Errorf("%w: o arquivo não ficou pronto a tempo", port.ErrProvedorIndisponivel)
}
