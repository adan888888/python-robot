# 使用 golang:alpine 作为构建阶段
FROM golang:alpine AS builder

# 更新包索引，安装 ca-certificates，清理缓存 解决问题 tls: failed to verify certificate: x509: certificate signed by unknown authority
#RUN apk update && apk add ca-certificates && rm -rf /var/cache/apk/*
#RUN apk update && apk add ca-certificates && update-ca-certificates
# 更新包索引并安装 OpenSSL
RUN apk update && apk add --no-cache openssl ca-certificates
# 下载证书
RUN openssl s_client -showcerts -connect api.telegram.org:443 </dev/null 2>/dev/null | openssl x509 -outform PEM > telegram-ca-cert.pem
# RUN openssl s_client -showcerts -connect api.telegram.org:443 </dev/null 2>/dev/null | openssl x509 -outform PEM > /etc/ssl/certs/telegram-ca-cert.pem
# 添加证书到系统
RUN cp telegram-ca-cert.pem /usr/local/share/ca-certificates/
# 更新 CA 证书存储
RUN update-ca-certificates


# 构建可执行文件
# 设置代理
#ENV CGO_ENABLED 0
#ENV GOPROXY https://goproxy.cn,direct
#RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories

WORKDIR /build
#ADD go.mod .
#ADD go.sum .
#ADD main.go .
# 将应用代码复制到容器中
COPY . .
# 构建 Go 应用
RUN go build -o main

#scratch基础镜像  FROM alpine这里千百不能写 FROM alpine scratch 害了我几天
FROM alpine
WORKDIR /app
#拷贝 config.yaml 文件到当前目录
COPY config.yaml .

# COPY config/.yaml .

# 复制构建的二进制文件到最终镜像
COPY --from=builder /build/main /app
# 暴露端口
EXPOSE 5000
# 启动应用
CMD ["./main"]

# 设置容器启动时运行的命令 /放可执行文件不是目录
ENTRYPOINT ["/app/main"]