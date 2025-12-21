# 多阶段构建 Dockerfile for LedgerBot

# 第一阶段：构建阶段
FROM golang:1.21-alpine AS builder

# 安装必要的构建工具
RUN apk add --no-cache git ca-certificates tzdata

# 设置工作目录
WORKDIR /build

# 复制 go mod 文件
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制源代码
COPY . .

# 构建应用
# CGO_ENABLED=0 禁用 CGO，生成静态链接的二进制文件
# -ldflags '-w -s' 减小二进制文件大小
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s' \
    -o ledgerbot \
    .

# 第二阶段：运行阶段
FROM alpine:latest

# 安装必要的运行时依赖（包括 wget 用于健康检查）
RUN apk --no-cache add ca-certificates tzdata wget

# 设置时区（可选，根据需要调整）
ENV TZ=Asia/Shanghai
RUN ln -snf /usr/share/zoneinfo/$TZ /etc/localtime && echo $TZ > /etc/timezone

# 设置默认环境变量
ENV SERVER_PORT=3906
ENV DATA_DIR=/data

# 创建非 root 用户
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

# 设置工作目录
WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /build/ledgerbot .

# 创建数据目录（使用环境变量 DATA_DIR 的默认值 /data）
RUN mkdir -p /data && \
    chown -R appuser:appuser /app /data

# 切换到非 root 用户
USER appuser

# 暴露端口
# 注意：EXPOSE 指令在构建时解析，不能使用环境变量，必须写死端口号
# 如果运行时修改了 SERVER_PORT，需要相应调整 docker run 的 -p 参数
EXPOSE 3906

# 健康检查
# 注意：HEALTHCHECK 的 CMD 在运行时执行，可以使用环境变量 $SERVER_PORT
# 但为了兼容性，这里使用 sh -c 来解析环境变量
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD sh -c "wget --no-verbose --tries=1 --spider http://localhost:${SERVER_PORT:-3906}/health || exit 1"

# 运行应用
CMD ["./ledgerbot"]

