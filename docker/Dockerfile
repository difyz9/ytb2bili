FROM difyz9/ytb2bili-base

LABEL maintainer="difyz9" \
      image="difyz9/ytb2bili" \
      version="v0.0.19"

# Docker 自动设置目标架构 (amd64 / arm64)
ARG TARGETARCH
ARG VERSION=v0.0.19

WORKDIR /app

# 从 GitHub Releases 下载对应架构的压缩包并解压
RUN ARCHIVE="ytb2bili-linux-${TARGETARCH}.tar.gz" \
    && URL="https://github.com/difyz9/ytb2bili/releases/download/${VERSION}/${ARCHIVE}" \
    && echo "Downloading ${URL}" \
    && curl -fsSL "${URL}" -o "/tmp/${ARCHIVE}" \
    && tar -xzf "/tmp/${ARCHIVE}" -C /tmp \
    && mv "/tmp/ytb2bili-linux-${TARGETARCH}/ytb2bili-linux-${TARGETARCH}" ./ytb2bili \
    && rm -rf "/tmp/${ARCHIVE}" "/tmp/ytb2bili-linux-${TARGETARCH}" \
    && chmod +x ./ytb2bili

# 创建数据目录
RUN mkdir -p ./logs ./downloads

EXPOSE 8096

CMD ["./ytb2bili"]
