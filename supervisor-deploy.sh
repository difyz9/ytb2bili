#!/bin/bash

# Supervisor 部署脚本 for ytb2bili-m
set -e

# 获取项目名称
if [ -f "go.mod" ]; then
    PROJECT_NAME=$(grep '^module ' go.mod | awk '{print $2}' | sed 's/.*\///')
else
    PROJECT_NAME="ytb2bili"
fi

echo "开始部署 $PROJECT_NAME with Supervisor..."

# 检查是否以 root 权限运行
if [ "$EUID" -ne 0 ]; then
    echo "请使用 sudo 运行此脚本"
    exit 1
fi

# 安装 Supervisor（如果未安装）
if ! command -v supervisorctl &> /dev/null; then
    echo "安装 Supervisor..."
    apt-get update
    apt-get install -y supervisor
    systemctl enable supervisor
    systemctl start supervisor
fi

# 停止现有服务
echo "停止现有服务..."
supervisorctl stop $PROJECT_NAME || true

# 生成 Supervisor 配置文件
echo "生成 Supervisor 配置文件..."
cat >/tmp/$PROJECT_NAME.conf <<EOL
[program:$PROJECT_NAME]
directory = /home/ubuntu/app/$PROJECT_NAME
command = /home/ubuntu/app/$PROJECT_NAME/$PROJECT_NAME
autostart = true ; 在 supervisord 启动的时候也自动启动
startsecs = 5 ; 启动 5 秒后没有异常退出，就当作已经正常启动了
autorestart = true ; 程序异常退出后自动重启
startretries = 3 ; 启动失败自动重试次数，默认是 3
user = root ; 用哪个用户启动
redirect_stderr = true ; 把 stderr 重定向到 stdout，默认 false
stdout_logfile_maxbytes = 20MB ; stdout 日志文件大小，默认 50MB
stdout_logfile_backups = 20 ; stdout 日志文件备份数
stdout_logfile = /var/log/$PROJECT_NAME.log ; 日志文件
environment=GIN_MODE=release ; 设置 Gin 运行模式
EOL

# 移动配置文件到正确位置
mv /tmp/$PROJECT_NAME.conf /etc/supervisor/conf.d/$PROJECT_NAME.conf

# 更新 Supervisor 配置
echo "更新 Supervisor 配置..."
supervisorctl reread
supervisorctl update

# 启动服务
echo "启动服务..."
supervisorctl start $PROJECT_NAME

# 等待服务启动
echo "等待服务启动..."
sleep 10

# 查看服务状态
echo "查看服务状态..."
supervisorctl status $PROJECT_NAME

# 测试服务是否正常
echo "测试服务健康状态..."

# 从配置文件读取端口号
APP_PORT=$(grep -E "^listen\s*=" config.toml | sed 's/.*=\s*//' | tr -d '"' | sed 's/://' | tr -d ' ' | head -1)
if [ -z "$APP_PORT" ]; then
    APP_PORT=8096  # 默认端口
fi

echo "应用端口: $APP_PORT"

# 健康检查
HEALTH_CHECK_URL="http://localhost:$APP_PORT/api/v1/health"
echo "健康检查 URL: $HEALTH_CHECK_URL"

# 尝试 3 次健康检查
for i in {1..3}; do
    echo "第 $i 次健康检查..."
    if curl -f -s "$HEALTH_CHECK_URL" > /dev/null 2>&1; then
        echo "✅ 健康检查通过"
        curl -s "$HEALTH_CHECK_URL" | head -20
        break
    else
        if [ $i -eq 3 ]; then
            echo "⚠️  健康检查失败，但服务可能正在启动中"
        else
            echo "等待 5 秒后重试..."
            sleep 5
        fi
    fi
done

echo ""
echo "查看日志（最后 50 行）："
tail -50 /var/log/$PROJECT_NAME.log || echo "日志文件尚未生成"

echo ""
echo "查看 Supervisor 状态："
supervisorctl status $PROJECT_NAME

echo ""
echo "🎉 部署完成！"
echo ""
echo "常用命令："
echo "  查看日志: tail -f /var/log/$PROJECT_NAME.log"
echo "  查看状态: supervisorctl status $PROJECT_NAME"
echo "  启动服务: supervisorctl start $PROJECT_NAME"
echo "  停止服务: supervisorctl stop $PROJECT_NAME"
echo "  重启服务: supervisorctl restart $PROJECT_NAME"
