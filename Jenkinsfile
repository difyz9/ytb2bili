pipeline {
  agent any

  environment {
    APP_NAME = 'ytb2bili'
    HEALTH_PORT = '8096'
    GIT_URL = 'https://gitee.com/difyz/ytb2bili.git'
    GIT_BRANCH = 'master'
    GIT_CREDENTIAL_ID = 'fbbaef12-c70b-4291-8c44-62e66b6b8d2c'
    
    DEPLOY_HOST = '124.222.202.16'
    DEPLOY_DIR  = "/home/ubuntu/app/${APP_NAME}"
    
    // 使用 Jenkins 凭证 ID（需要在 Jenkins 中配置 Username with password 类型的凭证）
    // DEPLOY_CREDENTIAL_ID = 'shanghai-node01'
    DEPLOY_CREDENTIAL_ID = 'deploy-singapore01-credential'
    
    HEALTH_URL = "http://localhost:${HEALTH_PORT}/api/v1/health"
  }

  options {
    timestamps()
    buildDiscarder(logRotator(numToKeepStr: '3'))
    disableConcurrentBuilds()
  }

  stages {
    stage('拉取代码') {
      steps {
        checkout([$class: 'GitSCM', branches: [[name: "*/${GIT_BRANCH}"]], doGenerateSubmoduleConfigurations: false, extensions: [], submoduleCfg: [], userRemoteConfigs: [[credentialsId: GIT_CREDENTIAL_ID, url: GIT_URL]]])
      }
    }

    stage('编译打包') {
      steps {
        sh 'echo Build ytb2bili'
        sh '/opt/go/bin/go env -w GO111MODULE=on'
        sh '/opt/go/bin/go env -w CGO_ENABLED=0 GOOS=linux GOARCH=amd64'
        sh '/opt/go/bin/go env -w GOPROXY=https://goproxy.cn,direct'
        sh '/opt/go/bin/go version'
        sh "/opt/go/bin/go build -o ${env.APP_NAME} main.go"
      }
    }

    stage('上传发布') {
      steps {
        script {
          withCredentials([usernamePassword(
            credentialsId: env.DEPLOY_CREDENTIAL_ID,
            usernameVariable: 'DEPLOY_USER',
            passwordVariable: 'DEPLOY_PASSWORD'
          )]) {
            def ts = sh(script: 'date +%Y%m%d%H%M%S', returnStdout: true).trim()
            def remote = [:]
            remote.name = 'target-node'
            remote.host = env.DEPLOY_HOST
            remote.user = DEPLOY_USER
            remote.password = DEPLOY_PASSWORD
            remote.port = 22
            remote.allowAnyHosts = true

            echo "部署目标: ${remote.user}@${remote.host} -> ${env.DEPLOY_DIR} 备份时间戳:${ts}"
            
            // 创建部署目录
            sshCommand remote: remote, command: "mkdir -p ${env.DEPLOY_DIR}"
            sshCommand remote: remote, command: "mkdir -p ${env.DEPLOY_DIR}/configs"
            sshCommand remote: remote, command: "mkdir -p ${env.DEPLOY_DIR}/logs"
            sshCommand remote: remote, command: "mkdir -p ${env.DEPLOY_DIR}/data"
            
            // 备份旧版本
            sshCommand remote: remote, command: "if [ -f ${env.DEPLOY_DIR}/${env.APP_NAME} ]; then sudo cp ${env.DEPLOY_DIR}/${env.APP_NAME} ${env.DEPLOY_DIR}/${env.APP_NAME}.${ts}.bak; fi"
            
            // 删除旧程序
            sshCommand remote: remote, command: "sudo rm -rf ${env.DEPLOY_DIR}/${env.APP_NAME}"
            
            // 上传新文件
            //sshPut remote: remote, from: "config-prod.toml", into: "${env.DEPLOY_DIR}/cconfigs/onfig.toml"
            sshPut remote: remote, from: "configs/.env.prod", into: "${env.DEPLOY_DIR}/.env"
            sshPut remote: remote, from: "supervisor-deploy.sh", into: "${env.DEPLOY_DIR}/supervisor-deploy.sh"
            sshPut remote: remote, from: "${env.APP_NAME}", into: "${env.DEPLOY_DIR}/${env.APP_NAME}"
            
            // 设置执行权限
            sshCommand remote: remote, command: "chmod +x ${env.DEPLOY_DIR}/${env.APP_NAME}"
            sshCommand remote: remote, command: "chmod +x ${env.DEPLOY_DIR}/supervisor-deploy.sh"
            sshCommand remote: remote, command: "ls -lh ${env.DEPLOY_DIR}/${env.APP_NAME}"
          }
        }
      }
    }

    stage('Supervisor 部署') {
      steps {
        script {
          withCredentials([usernamePassword(
            credentialsId: env.DEPLOY_CREDENTIAL_ID,
            usernameVariable: 'DEPLOY_USER',
            passwordVariable: 'DEPLOY_PASSWORD'
          )]) {
            def remote = [:]
            remote.name = 'target-node'
            remote.host = env.DEPLOY_HOST
            remote.user = DEPLOY_USER
            remote.password = DEPLOY_PASSWORD
            remote.port = 22
            remote.allowAnyHosts = true

            def supervisorProgram = env.APP_NAME 
            def deployDir = env.DEPLOY_DIR
            def binPath = "${deployDir}/${env.APP_NAME}"
            def logFile = "/var/log/${supervisorProgram}.log"
            def healthUrl = env.HEALTH_URL ?: 'http://localhost:8096/api/v1/health'
            def healthPort = env.HEALTH_PORT ?: '8096'

            def deployScriptTemplate = '''#!/bin/bash

set -e

PROGRAM="%s"
DEPLOY_DIR="%s"
BIN_PATH="%s"
LOG_FILE="%s"
HEALTH_URL="%s"
HEALTH_PORT="%s"

echo "开始执行 Supervisor 部署脚本..."

if [ "\$EUID" -ne 0 ]; then
  echo "请使用 sudo 运行此脚本"
  exit 1
fi

if ! command -v supervisorctl &> /dev/null; then
  echo "安装 Supervisor..."
  apt-get update
  apt-get install -y supervisor
  systemctl enable supervisor
  systemctl start supervisor
fi

echo "停止现有服务 \$PROGRAM"
supervisorctl stop "\$PROGRAM" || true

mkdir -p "\$DEPLOY_DIR"
chmod +x "\$BIN_PATH"

cat >/tmp/"\$PROGRAM".conf <<EOL
[program:\$PROGRAM]
directory = \$DEPLOY_DIR
command = \$BIN_PATH
autostart = true
startsecs = 5
autorestart = true
startretries = 3
user = root
redirect_stderr = true
stdout_logfile_maxbytes = 20MB
stdout_logfile_backups = 20
stdout_logfile = \$LOG_FILE
environment=GIN_MODE=release
EOL

mv /tmp/"\$PROGRAM".conf /etc/supervisor/conf.d/"\$PROGRAM".conf

echo "更新 Supervisor 配置"
supervisorctl reread
supervisorctl update

echo "启动服务 \$PROGRAM"
supervisorctl start "\$PROGRAM"

echo "等待服务健康检查..."
sleep 10

supervisorctl status "\$PROGRAM"

echo "查看最新日志："
tail -30 \$LOG_FILE || echo "日志文件尚未生成"

'''.stripIndent()

            def deployScript = String.format(
              deployScriptTemplate,
              supervisorProgram,
              deployDir,
              binPath,
              logFile,
              healthUrl,
              healthPort
            )

            sshCommand remote: remote, command: "sudo tee /tmp/deploy_supervisor.sh >/dev/null <<'EOF'\n${deployScript}\nEOF\n"
            sshCommand remote: remote, command: "sudo chmod +x /tmp/deploy_supervisor.sh"
            sshCommand remote: remote, command: "sudo /tmp/deploy_supervisor.sh"
            sshCommand remote: remote, command: "sudo rm -f /tmp/deploy_supervisor.sh"
          }
        }
      }
    }

    stage('完成') {
      steps {
        echo "🎉 ytb2bili 部署完成!"
        echo "📋 服务名称: ${env.APP_NAME}"
        echo "🌐 健康检查: ${env.HEALTH_URL}"
        echo "📂 部署目录: ${env.DEPLOY_DIR}"
        
        // 清空工作空间代码
        echo "🧹 开始清空工作空间..."
        echo "📋 当前Jenkins项目: ${env.JOB_NAME}"
        echo "📂 当前工作空间: ${env.WORKSPACE}"
        
        sh """
          echo "🗂️ 清空工作空间源码: ${env.WORKSPACE}"
          # 保留隐藏文件（如.git），只清理源码文件
          find ${env.WORKSPACE} -mindepth 1 -maxdepth 1 ! -name '.*' -exec rm -rf {} +
          echo "✅ 工作空间源码清空完成"
        """
      }
    }
  }

  post {
    success {
      echo '✅ ytb2bili 构建与部署成功'
    }
    failure {
      echo '❌ ytb2bili 构建或部署失败，请检查日志'
    }
  }
}
