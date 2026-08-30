# ---- deps: shared by the dev and build stages ----
FROM node:24-alpine AS deps

WORKDIR /app

COPY package.json package-lock.json ./
RUN npm ci

# ---- dev: vite with hot reload, used by docker-compose.override.yml ----
# The source is bind-mounted over /app at run time, so this stage carries only
# node_modules — which must NOT be shadowed by the mount (see the override's
# anonymous volume on /app/node_modules).
FROM node:24-alpine AS dev

WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .

EXPOSE 5173
# --host binds 0.0.0.0: without it vite listens on loopback inside the container
# and the port publish reaches nothing.
CMD ["npm", "run", "dev", "--", "--host", "0.0.0.0"]

# ---- build: the static site ----
FROM node:24-alpine AS build

WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .
RUN npm run build

# ---- prod: nginx serving the built SPA ----
FROM nginx:1.27-alpine

COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/build /usr/share/nginx/html

EXPOSE 5173
