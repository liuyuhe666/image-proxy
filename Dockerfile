FROM golang:1.23-alpine AS builder

RUN apk add --no-cache \
    git \
    make \
    gcc \
    libc-dev \
    curl \

WORKDIR /root/image-proxy
ENV CGO_ENABLED=0
COPY . .

RUN go mod download -x
RUN go build -o build/image-proxy main.go

FROM alpine:3.20 AS runner

RUN apk add --no-cache \
    ca-certificates  \
    tzdata

WORKDIR /root/image-proxy

COPY --from=builder /root/image-proxy/build/image-proxy .

EXPOSE 8888

ENTRYPOINT ["./image-proxy"]