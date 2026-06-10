# ── Build stage ──────────────────────────────────────────────────────────────
FROM golang:1.25 AS build

WORKDIR /src

# Cache modules first.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/chat-backend .

# ── Runtime stage ────────────────────────────────────────────────────────────
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app
COPY --from=build /out/chat-backend /app/chat-backend

# RUN_ROLE selects the active role (all|api|ws|worker|scheduler).
ENV RUN_ROLE=all
EXPOSE 8080

USER nonroot:nonroot
ENTRYPOINT ["/app/chat-backend"]
