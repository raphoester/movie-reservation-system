FROM golang:1.26.2-alpine3.23 AS builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /app

COPY ./ ./

RUN go mod download

RUN CGO_ENABLED=0 GOOS=linux go build -o main ./internal/tooling/cmd/migrate

FROM alpine:3.22

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/main .

RUN mkdir -p /app/configs

COPY --from=builder '/app/internal/tooling/cmd/migrate/configs' /app/configs

# no-op commit marker to trigger migrator rebuild detection
CMD ["./main"]
