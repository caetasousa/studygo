package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"studygo/internal/domain/prova"
	"studygo/internal/port"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var _ port.ProvaRepository = (*ProvaRepo)(nil)

// Chaves dos advisory locks das provas. A de arquivos é compartilhada com
// scripts/backup-provas.py, que a segura enquanto copia banco e volume: com
// ela presa, a limpeza não apaga um arquivo que o dump ainda referencia.
const (
	lockCriarImportacao    = 704401
	lockReservarImportacao = 704402
	lockArquivosProvas     = 704403
)

// ProvaRepo persiste importações, arquivos e publicações de provas.
//
// O rascunho e o conteúdo publicado são gravados como jsonb com os nomes dos
// campos do domínio. Renomear um campo de prova.Rascunho exige migrar o jsonb
// gravado — e as consultas abaixo que filtram por 'Ano', 'Orgao', 'Cargo' e 'CargoNome'.
type ProvaRepo struct {
	pool *pgxpool.Pool
}

func NewProvaRepo(pool *pgxpool.Pool) *ProvaRepo {
	return &ProvaRepo{pool: pool}
}

const colunasImportacao = `id::text, criador::text, hash, documento::text, gabarito_arquivo, estado,
	versao, etapa, falhas, processado_ms, chamadas, tentativa, erro, regioes, %s,
	coalesce(prova_id::text, ''), criado_em, atualizado_em`

var (
	selecionarImportacao = `SELECT ` + fmt.Sprintf(colunasImportacao, `rascunho`) + ` FROM provas_importacoes`
	selecionarResumo     = `SELECT ` + fmt.Sprintf(colunasImportacao, `rascunho - 'Questoes' - 'Apoios'`) +
		` FROM provas_importacoes`
)

func escanearImportacao(row pgx.Row) (prova.Importacao, error) {
	var (
		i                 prova.Importacao
		regioes, rascunho []byte
	)
	err := row.Scan(
		&i.ID, &i.Criador, &i.Hash, &i.Documento, &i.GabaritoArquivo, &i.Estado,
		&i.Versao, &i.Etapa, &i.Falhas, &i.ProcessadoMS, &i.Chamadas, &i.Tentativa, &i.Erro,
		&regioes, &rascunho, &i.ProvaID, &i.CriadoEm, &i.AtualizadoEm,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return prova.Importacao{}, prova.ErrNaoEncontrada
	}
	if err != nil {
		return prova.Importacao{}, fmt.Errorf("lendo importação de prova: %w", err)
	}
	if err := json.Unmarshal(regioes, &i.Regioes); err != nil {
		return prova.Importacao{}, fmt.Errorf("decodificando regiões: %w", err)
	}
	if err := json.Unmarshal(rascunho, &i.Rascunho); err != nil {
		return prova.Importacao{}, fmt.Errorf("decodificando rascunho: %w", err)
	}

	return i, nil
}

func (r *ProvaRepo) Criar(ctx context.Context, i prova.Importacao, limite int) (prova.Importacao, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return prova.Importacao{}, fmt.Errorf("iniciando transação: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback depois do commit é no-op

	// Serializa as criações: a busca pelo hash e a contagem de pendentes só
	// valem se ninguém inserir entre elas e o INSERT.
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, lockCriarImportacao); err != nil {
		return prova.Importacao{}, fmt.Errorf("reservando criação: %w", err)
	}

	if i.Hash != "" {
		existente, err := escanearImportacao(tx.QueryRow(ctx,
			selecionarImportacao+` WHERE hash = $1`, i.Hash,
		))
		if err == nil {
			return existente, nil
		}
		if !errors.Is(err, prova.ErrNaoEncontrada) {
			return prova.Importacao{}, err
		}
	}

	var pendentes int
	if err := tx.QueryRow(ctx,
		`SELECT count(*) FROM provas_importacoes
		  WHERE criador = $1 AND estado IN ('na_fila', 'processando', 'em_revisao')`,
		i.Criador,
	).Scan(&pendentes); err != nil {
		return prova.Importacao{}, fmt.Errorf("contando importações pendentes: %w", err)
	}
	if pendentes >= limite {
		return prova.Importacao{}, prova.ErrLimite
	}

	regioes, err := json.Marshal(i.Regioes)
	if err != nil {
		return prova.Importacao{}, fmt.Errorf("codificando regiões: %w", err)
	}
	rascunho, err := json.Marshal(i.Rascunho)
	if err != nil {
		return prova.Importacao{}, fmt.Errorf("codificando rascunho: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO provas_importacoes
		     (id, criador, hash, documento, gabarito_arquivo, estado, etapa, regioes, rascunho, prova_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NULLIF($10, '')::uuid)`,
		i.ID, i.Criador, i.Hash, i.Documento, i.GabaritoArquivo, i.Estado, i.Etapa,
		regioes, rascunho, i.ProvaID,
	); err != nil {
		return prova.Importacao{}, fmt.Errorf("criando importação de prova: %w", err)
	}

	arquivos := map[string]string{i.Documento: "pdf"}
	if i.GabaritoArquivo != "" {
		arquivos[i.GabaritoArquivo] = "pdf"
	}
	for _, id := range i.Rascunho.Arquivos() {
		arquivos[id] = "png"
	}
	if err := registrarArquivos(ctx, tx, i.ID, arquivos); err != nil {
		return prova.Importacao{}, err
	}

	// A revisão de uma prova publicada herda os arquivos da importação que a
	// gerou: os recortes do conteúdo e as prévias das regiões.
	if i.ProvaID != "" {
		if _, err := tx.Exec(ctx,
			`INSERT INTO provas_importacao_arquivos (importacao_id, arquivo_id)
			 SELECT $1, ia.arquivo_id
			   FROM provas_importacao_arquivos ia
			   JOIN provas_revisoes pr ON pr.importacao_id = ia.importacao_id
			   JOIN provas p ON p.id = pr.prova_id AND p.revisao = pr.revisao
			  WHERE p.id = $2
			 ON CONFLICT DO NOTHING`,
			i.ID, i.ProvaID,
		); err != nil {
			return prova.Importacao{}, fmt.Errorf("herdando arquivos da publicação: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return prova.Importacao{}, fmt.Errorf("confirmando importação: %w", err)
	}

	return r.Obter(ctx, i.ID)
}

func (r *ProvaRepo) Obter(ctx context.Context, id string) (prova.Importacao, error) {
	return escanearImportacao(r.pool.QueryRow(ctx, selecionarImportacao+` WHERE id = $1`, id))
}

func (r *ProvaRepo) ListarResumos(ctx context.Context) ([]prova.Importacao, error) {
	rows, err := r.pool.Query(ctx, selecionarResumo+` ORDER BY criado_em DESC LIMIT 100`)
	if err != nil {
		return nil, fmt.Errorf("listando importações de provas: %w", err)
	}
	defer rows.Close()

	out := []prova.Importacao{}
	for rows.Next() {
		i, err := escanearImportacao(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, i)
	}

	return out, rows.Err()
}

// Salvar grava uma decisão do curador. Ela zera a reserva: se o worker estava
// no meio de uma etapa, o resultado dele deixa de ser aceito.
func (r *ProvaRepo) Salvar(ctx context.Context, i prova.Importacao, versao int) error {
	rascunho, err := json.Marshal(i.Rascunho)
	if err != nil {
		return fmt.Errorf("codificando rascunho: %w", err)
	}
	regioes, err := json.Marshal(i.Regioes)
	if err != nil {
		return fmt.Errorf("codificando regiões: %w", err)
	}
	tag, err := r.pool.Exec(ctx,
		`UPDATE provas_importacoes
		    SET rascunho = $3, estado = $4, erro = $5, gabarito_arquivo = $6, etapa = $7,
		        hash = $8, regioes = $9, versao = versao + 1, atualizado_em = now(),
		        reserva_ate = NULL, tentativa = '', falhas = 0, disponivel_em = now()
		  WHERE id = $1 AND versao = $2 AND estado <> 'publicada'`,
		i.ID, versao, rascunho, i.Estado, i.Erro, i.GabaritoArquivo, i.Etapa, i.Hash, regioes,
	)
	if err != nil {
		return fmt.Errorf("salvando importação de prova: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return prova.ErrConflito
	}

	return nil
}

// Reservar entrega a próxima etapa pendente a uma tentativa nova.
//
// Uma importação por vez no ambiente inteiro: enquanto houver uma reserva
// viva, ninguém mais pega trabalho. Reserva vencida é de um worker que caiu, e
// a importação volta a ser elegível.
func (r *ProvaRepo) Reservar(ctx context.Context) (prova.Importacao, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return prova.Importacao{}, fmt.Errorf("iniciando transação: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback depois do commit é no-op

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, lockReservarImportacao); err != nil {
		return prova.Importacao{}, fmt.Errorf("reservando fila: %w", err)
	}

	var ocupada bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM provas_importacoes
		                 WHERE estado = 'processando' AND reserva_ate > now())`,
	).Scan(&ocupada); err != nil {
		return prova.Importacao{}, fmt.Errorf("consultando fila: %w", err)
	}
	if ocupada {
		return prova.Importacao{}, prova.ErrNaoEncontrada
	}

	i, err := escanearImportacao(tx.QueryRow(ctx, selecionarImportacao+`
		 WHERE (estado = 'na_fila' OR (estado = 'processando' AND reserva_ate <= now()))
		   AND disponivel_em <= now()
		 ORDER BY criado_em
		 LIMIT 1
		 FOR UPDATE SKIP LOCKED`))
	if err != nil {
		return prova.Importacao{}, err
	}

	i.Tentativa = uuid.NewString()
	i.Estado = prova.EstadoProcessando
	i.Chamadas++
	if _, err := tx.Exec(ctx,
		`UPDATE provas_importacoes
		    SET estado = 'processando', tentativa = $2, chamadas = chamadas + 1,
		        reserva_ate = now() + interval '2 minutes'
		  WHERE id = $1`,
		i.ID, i.Tentativa,
	); err != nil {
		return prova.Importacao{}, fmt.Errorf("reservando importação: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return prova.Importacao{}, fmt.Errorf("confirmando reserva: %w", err)
	}

	return i, nil
}

func (r *ProvaRepo) Renovar(ctx context.Context, id, tentativa string) (bool, error) {
	tag, err := r.pool.Exec(ctx,
		`UPDATE provas_importacoes SET reserva_ate = now() + interval '2 minutes'
		  WHERE id = $1 AND tentativa = $2 AND estado = 'processando' AND reserva_ate > now()`,
		id, tentativa,
	)
	if err != nil {
		return false, fmt.Errorf("renovando reserva: %w", err)
	}

	return tag.RowsAffected() == 1, nil
}

func (r *ProvaRepo) ConcluirEtapa(
	ctx context.Context,
	i prova.Importacao,
	etapa int,
	resultado any,
	duracao time.Duration,
) error {
	rascunho, err := json.Marshal(i.Rascunho)
	if err != nil {
		return fmt.Errorf("codificando rascunho: %w", err)
	}
	regioes, err := json.Marshal(i.Regioes)
	if err != nil {
		return fmt.Errorf("codificando regiões: %w", err)
	}
	produzido, err := json.Marshal(resultado)
	if err != nil {
		return fmt.Errorf("codificando resultado da etapa: %w", err)
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("iniciando transação: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback depois do commit é no-op

	tag, err := tx.Exec(ctx,
		`UPDATE provas_importacoes
		    SET rascunho = $3, regioes = $4, etapa = $5, estado = $6, processado_ms = $7,
		        versao = versao + 1, falhas = 0, erro = $8, reserva_ate = NULL, tentativa = '',
		        disponivel_em = now(), atualizado_em = now()
		  WHERE id = $1 AND tentativa = $2 AND estado = 'processando' AND reserva_ate > now()`,
		i.ID, i.Tentativa, rascunho, regioes, i.Etapa, i.Estado, i.ProcessadoMS, i.Erro,
	)
	if err != nil {
		return fmt.Errorf("concluindo etapa: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return prova.ErrConflito
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO provas_etapas (importacao_id, etapa, resultado, duracao_ms)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (importacao_id, etapa)
		 DO UPDATE SET resultado = excluded.resultado, duracao_ms = excluded.duracao_ms,
		               criado_em = now()`,
		i.ID, etapa, produzido, duracao.Milliseconds(),
	); err != nil {
		return fmt.Errorf("registrando etapa: %w", err)
	}

	recortes := map[string]string{}
	for _, id := range i.Rascunho.Arquivos() {
		recortes[id] = "png"
	}
	if err := registrarArquivos(ctx, tx, i.ID, recortes); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("confirmando etapa: %w", err)
	}

	return nil
}

// Excluir apaga a importação; as etapas e os vínculos com arquivos vão junto
// (ON DELETE CASCADE), e os arquivos sem dono saem em LimparReferencias. O
// estado entra na condição porque a reserva do worker não muda a versão.
func (r *ProvaRepo) Excluir(ctx context.Context, i prova.Importacao) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM provas_importacoes WHERE id = $1 AND versao = $2 AND estado = $3`,
		i.ID, i.Versao, i.Estado,
	)
	if err != nil {
		return fmt.Errorf("excluindo importação de prova: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return prova.ErrConflito
	}

	return nil
}

// Falhar devolve a etapa à fila depois da espera ou, com espera zero, marca a
// importação como falha. Quanto esperar, e se vale repetir, decide o serviço.
func (r *ProvaRepo) Falhar(ctx context.Context, i prova.Importacao, msg string, espera time.Duration) error {
	estado := prova.EstadoNaFila
	if espera <= 0 {
		estado = prova.EstadoFalhou
	}

	if _, err := r.pool.Exec(ctx,
		`UPDATE provas_importacoes
		    SET estado = $3, erro = $4, processado_ms = $6, falhas = falhas + 1,
		        reserva_ate = NULL, tentativa = '',
		        disponivel_em = now() + make_interval(secs => $5),
		        versao = versao + 1, atualizado_em = now()
		  WHERE id = $1 AND tentativa = $2 AND estado = 'processando' AND reserva_ate > now()`,
		i.ID, i.Tentativa, estado, msg, espera.Seconds(), i.ProcessadoMS,
	); err != nil {
		return fmt.Errorf("registrando falha da importação: %w", err)
	}

	return nil
}

// registrarArquivos liga arquivos (id → extensão) a uma importação.
func registrarArquivos(ctx context.Context, tx pgx.Tx, importacao string, arquivos map[string]string) error {
	if len(arquivos) == 0 {
		return nil
	}

	lote := &pgx.Batch{}
	for id, ext := range arquivos {
		if _, err := uuid.Parse(id); err != nil {
			return fmt.Errorf("identificador de arquivo inválido %q: %w", id, err)
		}
		lote.Queue(`INSERT INTO provas_arquivos (id, extensao) VALUES ($1, $2) ON CONFLICT DO NOTHING`, id, ext)
		lote.Queue(
			`INSERT INTO provas_importacao_arquivos (importacao_id, arquivo_id) VALUES ($1, $2)
			 ON CONFLICT DO NOTHING`,
			importacao, id,
		)
	}
	if err := tx.SendBatch(ctx, lote).Close(); err != nil {
		return fmt.Errorf("registrando arquivos da importação: %w", err)
	}

	return nil
}

func (r *ProvaRepo) RegistrarArquivo(ctx context.Context, importacao, id, extensao string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("iniciando transação: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback depois do commit é no-op

	if err := registrarArquivos(ctx, tx, importacao, map[string]string{id: extensao}); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *ProvaRepo) Arquivo(ctx context.Context, id string, curador bool) (string, error) {
	var ext string
	err := r.pool.QueryRow(ctx,
		`SELECT a.extensao
		   FROM provas_arquivos a
		  WHERE a.id = $1
		    AND ($2 OR EXISTS (
		        SELECT 1
		          FROM provas_importacao_arquivos ia
		          JOIN provas_revisoes pr ON pr.importacao_id = ia.importacao_id
		          JOIN provas p ON p.id = pr.prova_id AND p.revisao = pr.revisao
		         WHERE ia.arquivo_id = a.id AND p.visivel))`,
		id, curador,
	).Scan(&ext)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", prova.ErrNaoEncontrada
	}
	if err != nil {
		return "", fmt.Errorf("consultando arquivo de prova: %w", err)
	}

	return ext, nil
}

func (r *ProvaRepo) ArquivosDaImportacao(ctx context.Context, id string) ([]string, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT arquivo_id::text FROM provas_importacao_arquivos WHERE importacao_id = $1`, id,
	)
	if err != nil {
		return nil, fmt.Errorf("listando arquivos da importação: %w", err)
	}

	ids, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, fmt.Errorf("lendo arquivos da importação: %w", err)
	}

	return ids, nil
}

// Publicar grava a próxima revisão da prova numa transação só. Repetir a
// publicação da mesma importação devolve a prova já criada.
func (r *ProvaRepo) Publicar(ctx context.Context, i prova.Importacao, usuario string) (string, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("iniciando transação: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback depois do commit é no-op

	atual, err := escanearImportacao(tx.QueryRow(ctx, selecionarImportacao+` WHERE id = $1 FOR UPDATE`, i.ID))
	if err != nil {
		return "", err
	}
	if atual.Estado == prova.EstadoPublicada {
		return atual.ProvaID, nil
	}
	if atual.Versao != i.Versao || atual.Estado != prova.EstadoEmRevisao {
		return "", prova.ErrConflito
	}

	id := i.ProvaID
	if id == "" {
		id = uuid.NewString()
	}
	if _, err := tx.Exec(ctx, `INSERT INTO provas (id) VALUES ($1) ON CONFLICT DO NOTHING`, id); err != nil {
		return "", fmt.Errorf("criando prova: %w", err)
	}
	var revisao int
	if err := tx.QueryRow(ctx,
		`UPDATE provas SET revisao = revisao + 1, visivel = true, publicado_em = now()
		  WHERE id = $1 RETURNING revisao`,
		id,
	).Scan(&revisao); err != nil {
		return "", fmt.Errorf("abrindo revisão da prova: %w", err)
	}

	// A revisão guarda a identificação e o gabarito; questões e textos vão
	// para as tabelas de conteúdo, uma linha por conteúdo diferente.
	identificacao := i.Rascunho
	identificacao.Questoes, identificacao.Apoios = nil, nil
	conteudo, err := json.Marshal(identificacao)
	if err != nil {
		return "", fmt.Errorf("codificando conteúdo: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO provas_revisoes (prova_id, revisao, importacao_id, conteudo, publicado_por)
		 VALUES ($1, $2, $3, $4, $5)`,
		id, revisao, i.ID, conteudo, usuario,
	); err != nil {
		return "", fmt.Errorf("gravando revisão da prova: %w", err)
	}

	lote := &pgx.Batch{}
	for _, q := range i.Rascunho.Questoes {
		c, l := q.Separar()
		bc, err := json.Marshal(c)
		if err != nil {
			return "", fmt.Errorf("codificando questão %d: %w", q.Numero, err)
		}
		bl, err := json.Marshal(l)
		if err != nil {
			return "", fmt.Errorf("codificando questão %d: %w", q.Numero, err)
		}
		// O mesmo conteúdo já gravado — outra revisão, outro cargo — devolve a
		// linha que existe: DO UPDATE é o que faz o RETURNING trazê-la.
		lote.Queue(
			`WITH conteudo AS (
			     INSERT INTO provas_questoes_conteudo (id, impressao, conteudo) VALUES ($1, $2, $3)
			     ON CONFLICT (impressao) DO UPDATE SET impressao = EXCLUDED.impressao
			     RETURNING id)
			 INSERT INTO provas_questoes (prova_id, revisao, numero, disciplina, conteudo_id, lugar)
			 SELECT $4, $5, $6, $7, id, $8 FROM conteudo`,
			uuid.NewString(), c.Impressao(), bc, id, revisao, q.Numero, q.Disciplina, bl,
		)
	}
	for k, a := range i.Rascunho.Apoios {
		c, l := a.Separar()
		bc, err := json.Marshal(c)
		if err != nil {
			return "", fmt.Errorf("codificando material de apoio %s: %w", a.ID, err)
		}
		bl, err := json.Marshal(l)
		if err != nil {
			return "", fmt.Errorf("codificando material de apoio %s: %w", a.ID, err)
		}
		lote.Queue(
			`WITH conteudo AS (
			     INSERT INTO provas_apoios_conteudo (id, impressao, conteudo) VALUES ($1, $2, $3)
			     ON CONFLICT (impressao) DO UPDATE SET impressao = EXCLUDED.impressao
			     RETURNING id)
			 INSERT INTO provas_apoios (prova_id, revisao, identificador, ordem, conteudo_id, lugar)
			 SELECT $4, $5, $6, $7, id, $8 FROM conteudo`,
			uuid.NewString(), c.Impressao(), bc, id, revisao, a.ID, k, bl,
		)
	}
	// Publicada, a importação é histórico: o rascunho fica com a identificação,
	// e o resultado bruto de cada etapa sai — as questões moram na revisão.
	lote.Queue(
		`UPDATE provas_importacoes
		    SET estado = 'publicada', prova_id = $2, versao = versao + 1, atualizado_em = now(),
		        rascunho = rascunho - 'Questoes' - 'Apoios'
		  WHERE id = $1`,
		i.ID, id,
	)
	lote.Queue(`UPDATE provas_etapas SET resultado = '{}' WHERE importacao_id = $1`, i.ID)
	if err := tx.SendBatch(ctx, lote).Close(); err != nil {
		return "", fmt.Errorf("gravando questões da prova: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("confirmando publicação: %w", err)
	}

	return id, nil
}

const selecionarPublicacao = `SELECT p.id::text, p.revisao, pr.conteudo, p.publicado_em
	  FROM provas p
	  JOIN provas_revisoes pr ON pr.prova_id = p.id AND pr.revisao = p.revisao
	 WHERE p.visivel`

func escanearPublicacao(row pgx.Row) (prova.Publicacao, error) {
	var (
		p        prova.Publicacao
		conteudo []byte
	)
	err := row.Scan(&p.ID, &p.Revisao, &conteudo, &p.PublicadoEm)
	if errors.Is(err, pgx.ErrNoRows) {
		return prova.Publicacao{}, prova.ErrNaoEncontrada
	}
	if err != nil {
		return prova.Publicacao{}, fmt.Errorf("lendo prova publicada: %w", err)
	}
	if err := json.Unmarshal(conteudo, &p.Conteudo); err != nil {
		return prova.Publicacao{}, fmt.Errorf("decodificando prova publicada: %w", err)
	}

	return p, nil
}

// Catalogo lista as provas visíveis sem as questões: a lista mostra a
// identificação de cada prova, e o conteúdo vem na consulta de uma.
func (r *ProvaRepo) Catalogo(ctx context.Context, f port.FiltroCatalogo) ([]prova.Publicacao, error) {
	rows, err := r.pool.Query(ctx, selecionarPublicacao+`
		   AND ($1 = '' OR pr.conteudo->>'Ano' = $1)
		   AND ($2 = '' OR pr.conteudo->>'Orgao' ILIKE '%' || $2 || '%')
		   AND ($3 = '' OR pr.conteudo->>'Cargo' ILIKE '%' || $3 || '%'
		                OR pr.conteudo->>'CargoNome' ILIKE '%' || $3 || '%')
		   AND ($4 = '' OR EXISTS (
		        SELECT 1 FROM provas_questoes q
		         WHERE q.prova_id = p.id AND q.revisao = p.revisao
		           AND q.disciplina ILIKE '%' || $4 || '%'))
		 ORDER BY p.publicado_em DESC
		 LIMIT $5 OFFSET $6`,
		f.Ano, f.Orgao, f.Cargo, f.Disciplina, f.Limite, f.Offset,
	)
	if err != nil {
		return nil, fmt.Errorf("consultando catálogo de provas: %w", err)
	}
	defer rows.Close()

	out := []prova.Publicacao{}
	for rows.Next() {
		p, err := escanearPublicacao(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}

	return out, rows.Err()
}

func (r *ProvaRepo) Publicacao(ctx context.Context, id string) (prova.Publicacao, error) {
	p, err := escanearPublicacao(r.pool.QueryRow(ctx, selecionarPublicacao+` AND p.id = $1`, id))
	if err != nil {
		return prova.Publicacao{}, err
	}

	return p, r.completar(ctx, &p)
}

// ProvasDoAno devolve a identificação das provas visíveis da mesma banca e ano.
func (r *ProvaRepo) ProvasDoAno(ctx context.Context, banca string, ano int, exceto string) ([]prova.Publicacao, error) {
	rows, err := r.pool.Query(ctx, selecionarPublicacao+`
		   AND upper(pr.conteudo->>'Banca') = upper($1)
		   AND pr.conteudo->>'Ano' = $2
		   AND p.id::text <> $3
		 ORDER BY p.publicado_em`,
		banca, strconv.Itoa(ano), exceto,
	)
	if err != nil {
		return nil, fmt.Errorf("consultando provas do ano: %w", err)
	}
	defer rows.Close()

	var out []prova.Publicacao
	for rows.Next() {
		p, err := escanearPublicacao(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}

	return out, rows.Err()
}

func (r *ProvaRepo) ImportacoesAtivasDoAno(ctx context.Context, banca string, ano int, exceto string) ([]prova.Importacao, error) {
	rows, err := r.pool.Query(ctx, selecionarResumo+`
		 WHERE estado IN ('na_fila', 'processando', 'em_revisao', 'falhou')
		   AND upper(rascunho->>'Banca') = upper($1)
		   AND rascunho->>'Ano' = $2
		   AND id::text <> $3
		 ORDER BY criado_em`,
		banca, strconv.Itoa(ano), exceto,
	)
	if err != nil {
		return nil, fmt.Errorf("consultando importações ativas do ano: %w", err)
	}
	defer rows.Close()

	var out []prova.Importacao
	for rows.Next() {
		i, err := escanearImportacao(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, i)
	}

	return out, rows.Err()
}

// completar põe na publicação as questões e os textos da revisão dela.
func (r *ProvaRepo) completar(ctx context.Context, p *prova.Publicacao) error {
	questoes, err := r.questoesDaRevisao(ctx, p.ID, p.Revisao, 0, "")
	if err != nil {
		return err
	}
	p.Conteudo.Questoes = questoes

	rows, err := r.pool.Query(ctx,
		`SELECT c.conteudo, a.lugar
		   FROM provas_apoios a
		   JOIN provas_apoios_conteudo c ON c.id = a.conteudo_id
		  WHERE a.prova_id = $1 AND a.revisao = $2
		  ORDER BY a.ordem`,
		p.ID, p.Revisao,
	)
	if err != nil {
		return fmt.Errorf("consultando textos da prova: %w", err)
	}
	defer rows.Close()
	p.Conteudo.Apoios = []prova.Apoio{}
	for rows.Next() {
		var (
			bc, bl []byte
			c      prova.ConteudoDeApoio
			l      prova.LugarDoApoio
		)
		if err := rows.Scan(&bc, &bl); err != nil {
			return fmt.Errorf("lendo texto da prova: %w", err)
		}
		if err := json.Unmarshal(bc, &c); err != nil {
			return fmt.Errorf("decodificando texto da prova: %w", err)
		}
		if err := json.Unmarshal(bl, &l); err != nil {
			return fmt.Errorf("decodificando texto da prova: %w", err)
		}
		p.Conteudo.Apoios = append(p.Conteudo.Apoios, prova.JuntarApoio(c, l))
	}

	return rows.Err()
}

func (r *ProvaRepo) Questoes(ctx context.Context, provaID string, numero int, disciplina string) ([]prova.Questao, error) {
	var revisao int
	err := r.pool.QueryRow(ctx, `SELECT revisao FROM provas WHERE id::text = $1 AND visivel`, provaID).Scan(&revisao)
	if errors.Is(err, pgx.ErrNoRows) {
		return []prova.Questao{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("consultando prova: %w", err)
	}

	return r.questoesDaRevisao(ctx, provaID, revisao, numero, disciplina)
}

func (r *ProvaRepo) questoesDaRevisao(ctx context.Context, provaID string, revisao, numero int, disciplina string) ([]prova.Questao, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT c.conteudo, q.lugar
		   FROM provas_questoes q
		   JOIN provas_questoes_conteudo c ON c.id = q.conteudo_id
		  WHERE q.prova_id::text = $1 AND q.revisao = $2
		    AND ($3 = 0 OR q.numero = $3)
		    AND ($4 = '' OR q.disciplina ILIKE '%' || $4 || '%')
		  ORDER BY q.numero`,
		provaID, revisao, numero, disciplina,
	)
	if err != nil {
		return nil, fmt.Errorf("consultando questões da prova: %w", err)
	}
	defer rows.Close()

	out := []prova.Questao{}
	for rows.Next() {
		var (
			bc, bl []byte
			c      prova.ConteudoDeQuestao
			l      prova.LugarDaQuestao
		)
		if err := rows.Scan(&bc, &bl); err != nil {
			return nil, fmt.Errorf("lendo questão: %w", err)
		}
		if err := json.Unmarshal(bc, &c); err != nil {
			return nil, fmt.Errorf("decodificando questão: %w", err)
		}
		if err := json.Unmarshal(bl, &l); err != nil {
			return nil, fmt.Errorf("decodificando questão: %w", err)
		}
		out = append(out, prova.JuntarQuestao(c, l))
	}

	return out, rows.Err()
}

// QuestoesAvulsas escolhe a ocorrência de cada conteúdo pela estreia da prova
// — a primeira revisão, não a última: republicar a prova não pode trocar a
// ocorrência, porque as respostas do estudante ficam guardadas por prova e
// número, e trocar faria a questão parecer nunca resolvida.
func (r *ProvaRepo) QuestoesAvulsas(ctx context.Context) ([]prova.QuestaoAvulsa, error) {
	rows, err := r.pool.Query(ctx,
		`WITH estreia AS (
		     SELECT prova_id, min(publicado_em) AS em FROM provas_revisoes GROUP BY prova_id
		 ),
		 uma_por_conteudo AS (
		     SELECT DISTINCT ON (q.conteudo_id)
		            q.prova_id, q.numero, q.disciplina,
		            coalesce(q.lugar->>'Resposta', '') AS resposta,
		            coalesce(pr.conteudo->>'Orgao', '') AS orgao,
		            coalesce((pr.conteudo->>'Ano')::int, 0) AS ano,
		            coalesce(pr.conteudo->>'Cargo', '') AS cargo,
		            coalesce(pr.conteudo->>'CargoNome', '') AS cargo_nome,
		            e.em AS estreia
		       FROM provas_questoes q
		       JOIN provas p ON p.id = q.prova_id AND p.revisao = q.revisao AND p.visivel
		       JOIN provas_revisoes pr ON pr.prova_id = p.id AND pr.revisao = p.revisao
		       JOIN estreia e ON e.prova_id = p.id
		      ORDER BY q.conteudo_id, e.em, p.id, q.numero
		 )
		 SELECT prova_id::text, numero, disciplina, resposta, orgao, ano, cargo, cargo_nome
		   FROM uma_por_conteudo
		  ORDER BY ano DESC, estreia, prova_id, numero`,
	)
	if err != nil {
		return nil, fmt.Errorf("consultando questões avulsas: %w", err)
	}
	defer rows.Close()

	out := []prova.QuestaoAvulsa{}
	for rows.Next() {
		var q prova.QuestaoAvulsa
		if err := rows.Scan(&q.ProvaID, &q.Numero, &q.Disciplina, &q.Resposta, &q.Orgao, &q.Ano, &q.Cargo, &q.CargoNome); err != nil {
			return nil, fmt.Errorf("lendo questão avulsa: %w", err)
		}
		out = append(out, q)
	}

	return out, rows.Err()
}

func (r *ProvaRepo) ImportacaoDaPublicacao(ctx context.Context, provaID string) (prova.Importacao, error) {
	var importacao string
	err := r.pool.QueryRow(ctx,
		`SELECT pr.importacao_id::text
		   FROM provas_revisoes pr
		   JOIN provas p ON p.id = pr.prova_id AND p.revisao = pr.revisao
		  WHERE p.id = $1`,
		provaID,
	).Scan(&importacao)
	if errors.Is(err, pgx.ErrNoRows) {
		return prova.Importacao{}, prova.ErrNaoEncontrada
	}
	if err != nil {
		return prova.Importacao{}, fmt.Errorf("consultando origem da publicação: %w", err)
	}

	return r.Obter(ctx, importacao)
}

func (r *ProvaRepo) Retirar(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `UPDATE provas SET visivel = false WHERE id = $1 AND visivel`, id)
	if err != nil {
		return fmt.Errorf("retirando prova: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return prova.ErrNaoEncontrada
	}

	return nil
}

const msgRascunhoExpirado = "Rascunho expirado após 30 dias sem alteração."

func (r *ProvaRepo) Expirar(ctx context.Context, antes time.Time) error {
	if _, err := r.pool.Exec(ctx,
		// Em lote, o mesmo que prova.Importacao.Cancelar: sem o hash, os PDFs
		// voltam a poder ser importados.
		`UPDATE provas_importacoes
		    SET estado = 'cancelada', hash = '', erro = $2, versao = versao + 1, atualizado_em = now()
		  WHERE estado IN ('em_revisao', 'falhou') AND atualizado_em < $1`,
		antes, msgRascunhoExpirado,
	); err != nil {
		return fmt.Errorf("expirando rascunhos de provas: %w", err)
	}

	return nil
}

// LimparReferencias solta os arquivos de importações canceladas há mais de 30
// dias e apaga do banco os que ficaram sem nenhuma importação. Quem publicou
// alguma revisão nunca perde os arquivos.
func (r *ProvaRepo) LimparReferencias(ctx context.Context, antes time.Time) ([]string, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("iniciando transação: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback depois do commit é no-op

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, lockArquivosProvas); err != nil {
		return nil, fmt.Errorf("reservando limpeza: %w", err)
	}

	const canceladasAntigas = `i.estado = 'cancelada' AND i.atualizado_em < $1
		AND NOT EXISTS (SELECT 1 FROM provas_revisoes pr WHERE pr.importacao_id = i.id)`

	if _, err := tx.Exec(ctx,
		`DELETE FROM provas_importacao_arquivos ia USING provas_importacoes i
		  WHERE ia.importacao_id = i.id AND `+canceladasAntigas,
		antes,
	); err != nil {
		return nil, fmt.Errorf("soltando arquivos de rascunhos cancelados: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`UPDATE provas_importacoes i SET rascunho = '{}', regioes = '[]'
		  WHERE `+canceladasAntigas+` AND i.rascunho <> '{}'`,
		antes,
	); err != nil {
		return nil, fmt.Errorf("esvaziando rascunhos cancelados: %w", err)
	}

	rows, err := tx.Query(ctx,
		`DELETE FROM provas_arquivos a
		  WHERE a.criado_em < $1
		    AND NOT EXISTS (SELECT 1 FROM provas_importacao_arquivos ia WHERE ia.arquivo_id = a.id)
		 RETURNING a.id::text || '.' || a.extensao`,
		antes,
	)
	if err != nil {
		return nil, fmt.Errorf("apagando arquivos sem dono: %w", err)
	}
	orfaos, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, fmt.Errorf("lendo arquivos sem dono: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("confirmando limpeza: %w", err)
	}

	return orfaos, nil
}

func (r *ProvaRepo) Anotacoes(ctx context.Context, usuario, provaID string) ([]prova.Anotacao, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT numero, texto, atualizada_em FROM provas_anotacoes
		  WHERE usuario_id = $1 AND prova_id = $2
		  ORDER BY numero`,
		usuario, provaID,
	)
	if err != nil {
		return nil, fmt.Errorf("consultando anotações: %w", err)
	}

	out, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (prova.Anotacao, error) {
		var a prova.Anotacao
		err := row.Scan(&a.Numero, &a.Texto, &a.AtualizadaEm)
		return a, err
	})
	if err != nil {
		return nil, fmt.Errorf("lendo anotações: %w", err)
	}

	return out, nil
}

func (r *ProvaRepo) SalvarAnotacao(ctx context.Context, usuario, provaID string, a prova.Anotacao) (prova.Anotacao, error) {
	if err := r.pool.QueryRow(ctx,
		`INSERT INTO provas_anotacoes (usuario_id, prova_id, numero, texto)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (usuario_id, prova_id, numero)
		 DO UPDATE SET texto = excluded.texto, atualizada_em = now()
		 RETURNING atualizada_em`,
		usuario, provaID, a.Numero, a.Texto,
	).Scan(&a.AtualizadaEm); err != nil {
		return prova.Anotacao{}, fmt.Errorf("gravando anotação: %w", err)
	}

	return a, nil
}

func (r *ProvaRepo) ExcluirAnotacao(ctx context.Context, usuario, provaID string, numero int) error {
	if _, err := r.pool.Exec(ctx,
		`DELETE FROM provas_anotacoes WHERE usuario_id = $1 AND prova_id = $2 AND numero = $3`,
		usuario, provaID, numero,
	); err != nil {
		return fmt.Errorf("apagando anotação: %w", err)
	}

	return nil
}
