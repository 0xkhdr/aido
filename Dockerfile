# syntax=docker/dockerfile:1

# ---- build stage ----
FROM golang:1.26-alpine AS build
WORKDIR /src

# Cache modules first.
COPY go.mod go.sum* ./
RUN go mod download

COPY . .
# CGO off: modernc.org/sqlite is pure Go, so the binary is fully static.
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/app .

# ---- runtime stage ----
FROM alpine:3.20
RUN adduser -D -u 10001 app
WORKDIR /app
COPY --from=build /out/app /app/app
RUN mkdir -p /app/data && chown -R app:app /app
USER app

ENV ADDR=:8080 DB_PATH=/app/data/app.db
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s \
  CMD wget -qO- http://localhost:8080/healthz || exit 1

ENTRYPOINT ["/app/app"]
