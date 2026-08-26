# ==========================================
# STAGE 1: Build Bauhaus Frontend Assets
# ==========================================
FROM node:20-alpine AS frontend-builder
WORKDIR /web

# 配置国内 Alpine 镜像与 npmmirror 镜像源
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories && \
    npm config set registry https://registry.npmmirror.com

# 优先安装依赖
COPY web/package*.json ./
RUN npm install

# 显式拷贝前端源码文件，杜绝宿主机 node_modules 覆盖
COPY web/src ./src
COPY web/index.html ./
COPY web/vite.config.js ./

# 确保可执行权限并完成构建
RUN chmod -R +x node_modules/.bin && npm run build

# ==========================================
# STAGE 2: Build Single Go Binary Executable
# ==========================================
FROM golang:1.23-alpine AS backend-builder
WORKDIR /app

# 配置国内 Alpine 镜像与 GoProxy 国内代理 (goproxy.cn)
ENV GOPROXY=https://goproxy.cn,direct
ENV GO111MODULE=on

RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories && \
    apk add --no-cache git ca-certificates tzdata

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

# 配置国内 Alpine 软件源并安装证书与时区支持
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories && \
    apk add --no-cache ca-certificates tzdata curl && \
    mkdir -p /app/download /app/download/.cache

COPY --from=backend-builder /app/suencomic /app/suencomic

ENV PORT=8090
ENV DOWNLOAD_DIR=/app/download
ENV TZ=Asia/Shanghai

EXPOSE 8090

VOLUME ["/app/download"]

ENTRYPOINT ["/app/suencomic"]
CMD ["-port", "8090"]
