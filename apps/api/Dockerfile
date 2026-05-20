# syntax=docker/dockerfile:1

# Stage 1: build the React/Vite frontend.
FROM node:20-bookworm-slim AS webbuild

WORKDIR /web

COPY apps/web/package.json apps/web/package-lock.json ./
RUN npm ci

COPY apps/web/ ./
RUN npm run build

# Stage 2: build the Go API with the frontend embedded.
FROM golang:1.23-bookworm AS build

WORKDIR /src/apps/api

COPY apps/api/go.mod apps/api/go.sum* ./
RUN go mod download

COPY apps/api/ ./
# Replace the placeholder SPA with the compiled Vite output before `go build`
# so the //go:embed directive in internal/webui picks up the real assets.
COPY --from=webbuild /web/dist ./internal/webui/static
RUN CGO_ENABLED=1 GOOS=linux go build -o /out/streamize-api ./cmd/api

# Stage 3: minimal runtime image.
FROM debian:bookworm-slim

RUN apt-get update \
	&& apt-get install -y --no-install-recommends ca-certificates ffmpeg \
	&& rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=build /out/streamize-api /usr/local/bin/streamize-api

EXPOSE 8080

ENTRYPOINT ["streamize-api"]
