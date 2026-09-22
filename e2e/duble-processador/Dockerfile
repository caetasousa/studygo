# O dublê do edital-processor, só para o stack de E2E (ver servidor.mjs).
FROM node:24-alpine
WORKDIR /app
COPY servidor.mjs .
USER node
EXPOSE 8000
CMD ["node", "servidor.mjs"]
