# Frontend build stage
FROM node:20-alpine AS frontend-builder

WORKDIR /build

# Copy shadcn shared components
COPY shadcn ./shadcn

# Copy frontend source and resolve symlink
COPY frontend ./frontend
RUN rm -f frontend/src/components/ui && cp -r shadcn/ui frontend/src/components/ui
WORKDIR /build/frontend

# Install dependencies from lockfile
RUN npm ci

# Build the frontend application
RUN npm run build

# Backend build stage
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git

WORKDIR /build

# Copy local module dependencies
COPY projections ./projections
COPY hub-client ./hub-client

# Copy go mod files from backend
COPY backend/go.mod backend/go.sum ./backend/
WORKDIR /build/backend
RUN go mod download

# Copy source code
COPY backend/ .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o crm .

# Final stage
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y ca-certificates gosu && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/backend/crm /app/crm

# Copy frontend dist to pb_public (where the backend expects it)
COPY --from=frontend-builder /build/frontend/dist ./pb_public/

# Copy public assets
COPY frontend/public/ ./pb_public/

# Copy entrypoint script
COPY entrypoint.sh /app/entrypoint.sh

# Expose port
EXPOSE 8080

# Create non-root user (entrypoint runs as root, then drops to appuser via gosu)
RUN groupadd -r appuser && useradd -r -g appuser -d /app appuser && chown -R appuser:appuser /app

ENTRYPOINT ["/app/entrypoint.sh"]
CMD ["/app/crm", "serve", "--http=0.0.0.0:8080"]
