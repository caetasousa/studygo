"""O caso de uso do assistente de edital: os três passos que o backend chama.

1. analisar: o PDF (ou o texto colado) vira o documento normalizado, guardado
   como artefato, e o Gemini lê a banca e os cargos;
2. estrutura: a prova de um cargo, conferida pelas regras determinísticas;
3. conteudo: os tópicos das disciplinas escolhidas.

Os passos 2 e 3 só leem o artefato de quem o criou, e só enquanto ele vale.
"""

from __future__ import annotations

import asyncio
from dataclasses import dataclass

from app.domain.edital.document import NormalizedDocument
from app.domain.edital.extraction import Cargo, Conteudo, DocumentInfo, Estrutura, ExtractionResult
from app.domain.edital.validation_rules import validate as run_rules
from app.platform.config import Settings
from app.platform.errors import DocumentExpired, DocumentOwnerMismatch
from app.port.artefatos import Artefatos
from app.port.llm import LLMProvider
from app.port.ocr import Ocr
from app.port.pdf import LeitorDePdf
from app.service.edital import analise, extracao


@dataclass
class Analise:
    document: NormalizedDocument
    ocr_pages: int
    banca: str | None
    cargos: list[Cargo]


class Editais:
    def __init__(
        self,
        settings: Settings,
        artefatos: Artefatos,
        pdf: LeitorDePdf,
        ocr: Ocr,
        llm: LLMProvider,
    ) -> None:
        self._settings = settings
        self._artefatos = artefatos
        self._pdf = pdf
        self._ocr = ocr
        self._llm = llm

    def llm_disponivel(self) -> bool:
        return self._llm.available()

    async def analisar_pdf(
        self,
        data: bytes,
        declared_mime: str | None,
        filename: str,
        dono: str,
        request_id: str | None,
    ) -> Analise:
        # Numa thread porque a análise é SÍNCRONA e cara: ela abre o PDF,
        # extrai texto e chama o Tesseract. Chamada direto de dentro de um
        # `async def`, ela ocupa o event loop do uvicorn do começo ao fim — e
        # com o loop parado nada mais é atendido, nem o /healthz de 3 segundos,
        # que passava a falhar durante toda importação e marcava o container
        # como unhealthy sem ter nada de errado com ele.
        outcome = await asyncio.to_thread(
            analise.analyse,
            data=data,
            declared_mime=declared_mime,
            filename=filename,
            owner_ref=dono,
            settings=self._settings,
            store=self._artefatos,
            pdf=self._pdf,
            ocr=self._ocr,
            request_id=request_id,
        )
        return await self._com_cargos(outcome)

    async def analisar_texto(self, texto: str, dono: str, request_id: str | None) -> Analise:
        # Mesmo motivo do caminho do PDF: sem OCR, mas ainda normalização e
        # classificação sobre um texto que pode ser o edital inteiro colado.
        outcome = await asyncio.to_thread(
            analise.analyse_text,
            text=texto,
            filename="edital-colado.txt",
            owner_ref=dono,
            settings=self._settings,
            store=self._artefatos,
            request_id=request_id,
        )
        return await self._com_cargos(outcome)

    async def _com_cargos(self, outcome: analise.AnalyseOutcome) -> Analise:
        banca: str | None = None
        cargos: list[Cargo] = []
        if self._llm.available():
            banca, cargos = await extracao.extract_cargos(outcome.document, self._llm)
        return Analise(
            document=outcome.document,
            ocr_pages=len(outcome.ocr_page_numbers) if outcome.ocr_ran else 0,
            banca=banca,
            cargos=cargos,
        )

    async def estrutura(self, document_id: str, dono: str, cargo: str) -> Estrutura:
        doc = self.carregar(document_id, dono)
        result = await extracao.extract_estrutura(doc, cargo, self._llm)

        # As regras determinísticas rodam sobre um ExtractionResult: monta um parcial.
        partial = ExtractionResult(
            document=DocumentInfo(
                filename=doc.filename, sha256=doc.sha256, total_pages=doc.total_pages
            ),
            data_prova=result.data_prova,
            grupos_gerais=result.grupos_gerais,
            grupos_especificos=result.grupos_especificos,
            prova_discursiva=result.prova_discursiva,
            duracao=result.duracao,
        )
        result.alerts = run_rules(partial)
        return result

    async def conteudo(
        self, document_id: str, dono: str, cargo: str, disciplinas: list[str]
    ) -> Conteudo:
        doc = self.carregar(document_id, dono)
        return await extracao.extract_conteudo(doc, cargo, disciplinas, self._llm)

    def carregar(self, document_id: str, dono: str) -> NormalizedDocument:
        """O artefato, se ainda vale e é de quem pede. Vencido, ele sai do disco
        na hora — e o próximo pedido já não o encontra."""
        doc = self._artefatos.load(document_id)
        if doc.is_expired:
            self._artefatos.delete(document_id)
            raise DocumentExpired("document artifact has expired")
        if doc.owner_ref != dono:
            raise DocumentOwnerMismatch("document belongs to another user")
        return doc

    def varrer_vencidos(self) -> int:
        return self._artefatos.sweep_expired()
