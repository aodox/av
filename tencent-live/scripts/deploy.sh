#!/bin/bash

set -e

APP_NAME="tencent-live"
APP_DIR="/opt/tencent-live"
SERVICE_FILE="/etc/systemd/system/${APP_NAME}.service"

show_help() {
    echo "Usage: $0 {install|start|stop|restart|status|logs|uninstall}"
    echo ""
    echo "Commands:"
    echo "  install   - Install the service"
    echo "  start     - Start the service"
    echo "  stop      - Stop the service"
    echo "  restart   - Restart the service"
    echo "  status    - Show service status"
    echo "  logs      - Show service logs"
    echo "  uninstall - Uninstall the service"
}

install_service() {
    echo "Installing ${APP_NAME}..."
    
    mkdir -p ${APP_DIR}/{bin,config,logs}
    
    if [ -f "./bin/${APP_NAME}" ]; then
        cp ./bin/${APP_NAME} ${APP_DIR}/bin/
        chmod +x ${APP_DIR}/bin/${APP_NAME}
    else
        echo "Error: Binary not found. Please build first."
        exit 1
    fi
    
    if [ -f "./config/config.yaml" ]; then
        cp ./config/config.yaml ${APP_DIR}/config/
    fi
    
    cat > ${SERVICE_FILE} << EOF
[Unit]
Description=Tencent Live Stream Service
After=network.target mysql.service redis.service

[Service]
Type=simple
User=root
WorkingDirectory=${APP_DIR}
ExecStart=${APP_DIR}/bin/${APP_NAME} -config ${APP_DIR}/config/config.yaml
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF
    
    systemctl daemon-reload
    systemctl enable ${APP_NAME}
    
    echo "Installation completed!"
    echo "Please edit ${APP_DIR}/config/config.yaml before starting the service."
}

start_service() {
    systemctl start ${APP_NAME}
    echo "${APP_NAME} started."
}

stop_service() {
    systemctl stop ${APP_NAME}
    echo "${APP_NAME} stopped."
}

restart_service() {
    systemctl restart ${APP_NAME}
    echo "${APP_NAME} restarted."
}

show_status() {
    systemctl status ${APP_NAME}
}

show_logs() {
    journalctl -u ${APP_NAME} -f
}

uninstall_service() {
    echo "Uninstalling ${APP_NAME}..."
    
    systemctl stop ${APP_NAME} 2>/dev/null || true
    systemctl disable ${APP_NAME} 2>/dev/null || true
    
    rm -f ${SERVICE_FILE}
    systemctl daemon-reload
    
    echo "Service uninstalled. Data directory ${APP_DIR} preserved."
    echo "Run 'rm -rf ${APP_DIR}' to remove all data."
}

case "$1" in
    install)
        install_service
        ;;
    start)
        start_service
        ;;
    stop)
        stop_service
        ;;
    restart)
        restart_service
        ;;
    status)
        show_status
        ;;
    logs)
        show_logs
        ;;
    uninstall)
        uninstall_service
        ;;
    *)
        show_help
        exit 1
        ;;
esac
