# Build stage
FROM golang:alpine AS builder

WORKDIR /app

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o server ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o ingest ./cmd/ingest

# Production stage
FROM alpine:latest

WORKDIR /app

RUN apk --no-cache add ca-certificates tzdata

COPY --from=builder /app/server /app/server
COPY --from=builder /app/ingest /app/ingest
# Both migration directories are needed:
#   - db/migrations/ is the source of truth (used by `make migrate-*` CLI)
#   - internal/repository/postgres/migrations/ is the embedded copy
#     that the running server reads via //go:embed
COPY --from=builder /app/db/migrations /app/db/migrations
COPY --from=builder /app/internal/repository/postgres/migrations /app/internal/repository/postgres/migrations
COPY --from=builder /app/tailieu /app/tailieu

ENV PORT=8080
ENV DOCUMENTS_DIR=/app/tailieu

EXPOSE 8080

CMD ["/app/server"]
