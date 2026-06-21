FROM difyz9/ytb2bili-base:latest

ARG APP_BINARY=ytb2bili-linux-arm64

LABEL maintainer="difyz9" \
      image="difyz9/ytb2bili"

WORKDIR /app

# 注入 Asia/Shanghai 时区文件（无需 apt-get tzdata）
RUN mkdir -p /usr/share/zoneinfo/Asia
COPY Shanghai.zone /usr/share/zoneinfo/Asia/Shanghai
RUN ln -sf /usr/share/zoneinfo/Asia/Shanghai /etc/localtime \
    && echo "Asia/Shanghai" > /etc/timezone

# 更新 yt-dlp 至最新版本（修复 YouTube n-challenge 兼容性问题）
RUN yt-dlp -U || pip install -U yt-dlp || true


# 复制与目标平台匹配的可执行文件和配置文件
COPY ${APP_BINARY} ./ytb2bili

COPY config.toml ./config.toml

# Optional: watermark fonts (place ttf/otf under docker/fonts/)
COPY fonts/ ./fonts/

# 创建数据目录并赋予执行权限
RUN mkdir -p ./data ./downloads \
    && chmod +x ./ytb2bili

EXPOSE 8096

CMD ["./ytb2bili"]
