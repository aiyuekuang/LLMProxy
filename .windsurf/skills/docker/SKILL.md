---
name: docker
description: Docker and containerization best practices. Use for Dockerfile creation, docker-compose, multi-stage builds, and container optimization.
---

# Docker Skill

Container best practices for development, building, and deployment.

## When to Use This Skill

- Writing Dockerfiles
- Configuring docker-compose
- Optimizing container images
- Container security
- Multi-stage builds

---

# 📦 Dockerfile Best Practices

## Multi-Stage Build (Go)

```dockerfile
# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/server ./cmd/main.go

# Runtime stage
FROM alpine:3.19

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/server .
COPY --from=builder /app/config.yaml .

# Non-root user
RUN adduser -D -g '' appuser
USER appuser

EXPOSE 8080

ENTRYPOINT ["./server"]
```

## Multi-Stage Build (Node.js)

```dockerfile
# Dependencies stage
FROM node:20-alpine AS deps
WORKDIR /app
COPY package.json yarn.lock ./
RUN yarn install --frozen-lockfile

# Build stage
FROM node:20-alpine AS builder
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .
RUN yarn build

# Runtime stage
FROM node:20-alpine AS runner
WORKDIR /app

ENV NODE_ENV=production

RUN addgroup -g 1001 -S nodejs
RUN adduser -S nextjs -u 1001

COPY --from=builder /app/public ./public
COPY --from=builder --chown=nextjs:nodejs /app/.next/standalone ./
COPY --from=builder --chown=nextjs:nodejs /app/.next/static ./.next/static

USER nextjs

EXPOSE 3000
ENV PORT 3000

CMD ["node", "server.js"]
```

---

# 🔧 Layer Optimization

## Order Matters

```dockerfile
# ✅ Good: Least changing files first
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o app

# ❌ Bad: COPY all invalidates cache
COPY . .
RUN go mod download
RUN go build -o app
```

## Reduce Layers

```dockerfile
# ❌ Bad: Multiple RUN commands
RUN apt-get update
RUN apt-get install -y curl
RUN apt-get install -y git
RUN rm -rf /var/lib/apt/lists/*

# ✅ Good: Single RUN command
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        curl \
        git && \
    rm -rf /var/lib/apt/lists/*
```

## Use .dockerignore

```dockerignore
# Git
.git
.gitignore

# Dependencies
node_modules
vendor

# Build artifacts
dist
build
*.exe

# Development files
*.md
*.log
.env.local
.vscode
.idea

# Tests
*_test.go
__tests__
coverage
```

---

# 🐳 Docker Compose

## Development Setup

```yaml
version: '3.8'

services:
  app:
    build:
      context: .
      dockerfile: Dockerfile.dev
    ports:
      - "8080:8080"
    volumes:
      - .:/app
      - /app/node_modules  # Exclude node_modules
    environment:
      - DATABASE_URL=postgres://user:pass@db:5432/app
      - REDIS_URL=redis://redis:6379
    depends_on:
      db:
        condition: service_healthy
      redis:
        condition: service_started

  db:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: user
      POSTGRES_PASSWORD: pass
      POSTGRES_DB: app
    volumes:
      - postgres_data:/var/lib/postgresql/data
    ports:
      - "5432:5432"
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U user -d app"]
      interval: 5s
      timeout: 5s
      retries: 5

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    volumes:
      - redis_data:/data

volumes:
  postgres_data:
  redis_data:
```

## Production Setup

```yaml
version: '3.8'

services:
  app:
    image: ${REGISTRY}/app:${VERSION}
    deploy:
      replicas: 3
      resources:
        limits:
          cpus: '1'
          memory: 512M
        reservations:
          cpus: '0.5'
          memory: 256M
      restart_policy:
        condition: on-failure
        delay: 5s
        max_attempts: 3
    environment:
      - DATABASE_URL=${DATABASE_URL}
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s
    logging:
      driver: "json-file"
      options:
        max-size: "10m"
        max-file: "3"

  nginx:
    image: nginx:alpine
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./nginx.conf:/etc/nginx/nginx.conf:ro
      - ./certs:/etc/nginx/certs:ro
    depends_on:
      - app
```

---

# 🔒 Security

## Run as Non-Root

```dockerfile
# Create non-root user
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

# Change ownership
COPY --chown=appuser:appgroup . .

# Switch to non-root user
USER appuser
```

## Scan for Vulnerabilities

```bash
# Using Docker Scout
docker scout cves myimage:latest

# Using Trivy
trivy image myimage:latest

# Using Snyk
snyk container test myimage:latest
```

## Read-Only Filesystem

```yaml
services:
  app:
    read_only: true
    tmpfs:
      - /tmp
      - /var/run
```

---

# 📊 Health Checks

```dockerfile
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD curl -f http://localhost:8080/health || exit 1
```

```go
// Health endpoint
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
    checks := map[string]string{
        "database": h.checkDB(),
        "redis":    h.checkRedis(),
    }
    
    healthy := true
    for _, status := range checks {
        if status != "ok" {
            healthy = false
            break
        }
    }
    
    if healthy {
        w.WriteHeader(http.StatusOK)
    } else {
        w.WriteHeader(http.StatusServiceUnavailable)
    }
    json.NewEncoder(w).Encode(checks)
}
```

---

# 🛠️ Useful Commands

```bash
# Build
docker build -t myapp:latest .
docker build --no-cache -t myapp:latest .  # Fresh build

# Run
docker run -d -p 8080:8080 --name myapp myapp:latest
docker run --rm -it myapp:latest /bin/sh  # Interactive

# Logs
docker logs -f myapp
docker logs --tail 100 myapp

# Exec into container
docker exec -it myapp /bin/sh

# Cleanup
docker system prune -a  # Remove all unused
docker volume prune     # Remove unused volumes
docker image prune      # Remove dangling images

# Inspect
docker stats            # Resource usage
docker inspect myapp    # Container details
docker history myapp    # Image layers
```

---

# 📚 References

- [Docker Best Practices](https://docs.docker.com/develop/develop-images/dockerfile_best-practices/)
- [Docker Security](https://docs.docker.com/engine/security/)
- [dmmulroy/cloudflare-skill](https://github.com/dmmulroy/cloudflare-skill)
