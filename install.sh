#!/bin/bash

set -e  # 发生错误时退出

# 1. 下载并安装 gpu_user_exporter
INSTALL_DIR="/usr/local/bin"
SERVICE_FILE="/etc/systemd/system/gpu_user_exporter.service"

echo "Downloading gpu_user_exporter..."
wget -O "$INSTALL_DIR/gpu_user_exporter" "https://github.com/MiracleHYH/gpu_user_exporter/releases/download/latest/gpu_user_exporter"
chmod +x "$INSTALL_DIR/gpu_user_exporter"

# 2. 创建 systemd 服务
echo "Creating systemd service..."
cat <<EOF > "$SERVICE_FILE"
[Unit]
Description=GPU User Exporter for Prometheus
After=network.target

[Service]
User=root
Group=root
ExecStart=$INSTALL_DIR/gpu_user_exporter
Restart=always
RestartSec=5
StandardOutput=syslog
StandardError=syslog
SyslogIdentifier=gpu_user_exporter
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
EOF

# 3. 重新加载 systemd 并启动服务
echo "Starting service..."
systemctl daemon-reload
systemctl enable gpu_user_exporter
systemctl restart gpu_user_exporter

echo "Installation completed!"
