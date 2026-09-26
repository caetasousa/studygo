# O dublê do edital-processor, só para o stack de E2E (ver servidor.mjs).
FROM node:24-alpine
WORKDIR /app
# O contexto é e2e/: a lei que a captura devolve é a mesma fixture dos testes.
COPY duble-processador/servidor.mjs .
COPY fixtures/lei-exemplo.json fixtures/lei-exemplo-v2.json ./fixtures/
USER node
EXPOSE 8000
CMD ["node", "servidor.mjs"]
