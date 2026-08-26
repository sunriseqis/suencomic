# ==========================================
# STAGE 1: Build Bauhaus Frontend Assets
# ==========================================
FROM node:20-alpine AS frontend-builder
WORKDIR /web
COPY web/package*.json ./
RUN npm install
COPY web/ ./
RUN npm run build

# ==========================================
# STAGE 2: Build Single Go Binary Executable
# ==========================================
FROM golang:1.23-alpine AS backend-builder
WORKDIR /app
RUN apk add --no-cache git ca-certificates tzdata

COPY go.mod go.sum* ./
RUN go mod download

COPY . .
COPY --from=frontend-builder /web/dist ./web/dist

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o suencomic .

# ==========================================
# STAGE 3: Minimal Production Runtime
# ==========================================
FROM alpine:3.20
WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata curl && \
    mkdir -p /app/download /app/download/.cache

COPY --from=backend-builder /app/suencomic /app/suencomic

ENV PORT=8090
ENV DOWNLOAD_DIR=/app/download

EXPOSE 8090

VOLUME ["/app/download"]

ENTRYPOINT ["/app/suencomic"]
CMD ["-port", "8090"]
