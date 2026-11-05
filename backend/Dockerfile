FROM golang:1.27.3-bookworm AS development
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .

FROM development AS build
RUN CGO_ENABLED=0 go build -trimpath -o /out/api ./cmd/api

FROM debian:bookworm-slim AS runtime
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --gid 10001 api && useradd --uid 10001 --gid api --no-create-home api
WORKDIR /app
COPY --from=build /out/api ./api
USER 10001:10001
EXPOSE 8080
ENTRYPOINT ["/app/api"]
