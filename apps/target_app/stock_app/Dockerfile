# Multi-stage build for the stock_app target app (ECS Fargate).
# The binary is built static (CGO disabled) so it runs in a tiny, secure base image.

# --- build ---
FROM golang:1.26-alpine AS build
WORKDIR /src

# Cache dependencies first (this layer is reused if go.mod/go.sum are unchanged).
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# CGO_ENABLED=0 -> static binary; -ldflags "-s -w" strips symbols for a smaller image.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /stock_app ./src/cmd

# --- run ---
# distroless/static is ~25 MB, no shell, no package manager (good for a demo).
# It runs as non-root; the app only binds 8080 (unprivileged), so this is fine.
FROM gcr.io/distroless/static:nonroot
COPY --from=build /stock_app /stock_app

# The app runs as a plain HTTP server on PORT (default 8080) and drains
# gracefully on SIGTERM (ECS task stop).
EXPOSE 8080
CMD ["/stock_app"]
