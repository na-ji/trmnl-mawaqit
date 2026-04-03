FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o server .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o healthcheck ./cmd/healthcheck

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app
COPY --from=builder /app/server /app/server
COPY --from=builder /app/healthcheck /app/healthcheck
COPY --from=builder /app/templates /app/templates

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
  CMD ["/app/healthcheck"]

EXPOSE 8080
ENTRYPOINT ["/app/server"]
