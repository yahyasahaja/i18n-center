# Multi-stage build for the i18n-center backend image.
#
# Produces ONE image that contains TWO binaries:
#   /usr/local/bin/i18n-center          — Gin HTTP server (default CMD)
#   /usr/local/bin/i18n-center-migrate  — goose migration CLI (run via ArgoCD PreSync Job)
#
# The same image is reused for the `backend` deployment (default entrypoint) and the
# `migrate` Helm "migration"-type Job (overridden via `command:` in the Helm values).
# Migrations are embedded into the binary via //go:embed in migrations/migrations.go,
# so the image is self-contained — no SQL files copied at runtime.

FROM golang:1.25-alpine AS builder

WORKDIR /src

# Cache module downloads in a separate layer.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build both binaries with CGO disabled for a static binary on alpine.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/i18n-center         ./main.go \
 && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/i18n-center-migrate ./cmd/migrate

# ── Final stage ───────────────────────────────────────────────────────────────
FROM alpine:3.20

RUN apk --no-cache add ca-certificates tzdata \
 && addgroup -g 65532 -S app \
 && adduser  -u 65532 -S app -G app

COPY --from=builder /out/i18n-center         /usr/local/bin/i18n-center
COPY --from=builder /out/i18n-center-migrate /usr/local/bin/i18n-center-migrate

USER 65532:65532
WORKDIR /home/app

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/i18n-center"]
