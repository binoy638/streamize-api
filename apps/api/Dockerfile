FROM golang:1.23-bookworm AS build

WORKDIR /src/apps/api

COPY apps/api/go.mod apps/api/go.sum* ./
RUN go mod download

COPY apps/api/ ./
RUN CGO_ENABLED=1 GOOS=linux go build -o /out/streamize-api ./cmd/api

FROM debian:bookworm-slim

RUN apt-get update \
	&& apt-get install -y --no-install-recommends ca-certificates ffmpeg \
	&& rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=build /out/streamize-api /usr/local/bin/streamize-api

EXPOSE 8080

ENTRYPOINT ["streamize-api"]
