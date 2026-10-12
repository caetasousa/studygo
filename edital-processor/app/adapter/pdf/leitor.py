"""O leitor de PDF do serviço: PyMuPDF para validar e ler, pdfplumber para as
tabelas (validacao.py e extracao.py)."""

from __future__ import annotations

from app.adapter.pdf.extracao import extract_native
from app.adapter.pdf.validacao import validate_upload
from app.platform.config import Settings
from app.port.pdf import ExtractionOutput, ValidatedPDF


class LeitorPyMuPDF:
    def __init__(self, settings: Settings) -> None:
        self._settings = settings

    def validar(self, data: bytes, declared_mime: str | None) -> ValidatedPDF:
        return validate_upload(data, declared_mime, self._settings)

    def extrair(self, data: bytes) -> ExtractionOutput:
        return extract_native(data)

    def paginas(self, data: bytes) -> list[str]:
        # O PyMuPDF pesa uns 50 MB e só serve quando chega um PDF: carrega aqui,
        # não no boot do processador.
        import pymupdf

        with pymupdf.open(stream=data, filetype="pdf") as doc:
            return [str(pagina.get_text()) for pagina in doc]
