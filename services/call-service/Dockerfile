FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod ./
RUN go mod download || true

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o call_service ./cmd/main.go

FROM alpine:3.19

WORKDIR /app
RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /app/call_service .

EXPOSE 8081

CMD ["./call_service"]
