#!/usr/bin/env bash
# GarudaPanel Installer (SPEC §5.10 & §1.1)
# Single static binary installer & systemd service setup.
set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

echo -e "${CYAN}====================================================${NC}"
echo -e "${CYAN}             GarudaPanel Setup Installer            ${NC}"
echo -e "${CYAN}====================================================${NC}"

if [ "$(id -u)" -ne 0 ]; then
    echo -e "${RED}[ERROR] Script ini harus dijalankan sebagai root (sudo).${NC}"
    exit 1
fi

INSTALL_DIR="/usr/local/bin"
GARUDA_BIN="${INSTALL_DIR}/garuda"
DATA_DIR="/var/lib/garudapanel"
CONFIG_DIR="/etc/garudapanel"
SYSTEMD_DIR="/etc/systemd/system"

mkdir -p "${DATA_DIR}" "${CONFIG_DIR}"

MODE="${1:-}"

if [ -z "${MODE}" ]; then
    echo "Pilih mode instalasi:"
    echo "  1) Panel (Web UI + API Server)"
    echo "  2) Daemon (Worker Node Service)"
    echo "  3) Keduanya (All-in-one di satu mesin)"
    read -rp "Pilihan [1/2/3]: " CHOICE
    case "${CHOICE}" in
        1) MODE="panel" ;;
        2) MODE="daemon" ;;
        3) MODE="both" ;;
        *) echo -e "${RED}Pilihan tidak valid.${NC}"; exit 1 ;;
    esac
fi

# 1. Pastikan binary garuda terinstall
if [ -f "./garuda" ]; then
    echo -e "${BLUE}[INFO] Menyalin binary garuda lokal ke ${GARUDA_BIN}...${NC}"
    cp -f ./garuda "${GARUDA_BIN}"
    chmod +x "${GARUDA_BIN}"
elif command -v garuda >/dev/null 2>&1; then
    echo -e "${BLUE}[INFO] Binary garuda sudah ada di sistem PATH.${NC}"
else
    echo -e "${BLUE}[INFO] Binary lokal tidak ditemukan, mencoba compile atau download...${NC}"
    if command -v go >/dev/null 2>&1; then
        echo -e "${BLUE}[INFO] Mengompilasi binary statis dengan Go...${NC}"
        CGO_ENABLED=0 go build -o "${GARUDA_BIN}" ./cmd/garuda
        chmod +x "${GARUDA_BIN}"
    else
        echo -e "${RED}[ERROR] Binary './garuda' tidak ditemukan dan 'go' compiler tidak tersedia.${NC}"
        echo -e "Silakan build dulu binary dengan 'CGO_ENABLED=0 go build -o garuda ./cmd/garuda'."
        exit 1
    fi
fi

echo -e "${GREEN}[OK] Binary terpasang di ${GARUDA_BIN}${NC}"

# 2. Setup Panel Service
setup_panel() {
    echo -e "${BLUE}[INFO] Menyiapkan systemd service untuk GarudaPanel...${NC}"
    PANEL_PORT="${PANEL_PORT:-8080}"
    PANEL_DB="${DATA_DIR}/garuda-panel.db"

    cat <<EOF > "${SYSTEMD_DIR}/garuda-panel.service"
[Unit]
Description=GarudaPanel - Game Server Management Panel
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=${DATA_DIR}
ExecStart=${GARUDA_BIN} panel --addr :${PANEL_PORT} --db ${PANEL_DB}
Restart=always
RestartSec=5s
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload
    systemctl enable garuda-panel
    echo -e "${GREEN}[OK] Service garuda-panel dibuat di ${SYSTEMD_DIR}/garuda-panel.service${NC}"
    echo -e "${CYAN}Jalankan: systemctl start garuda-panel${NC}"
}

# 3. Setup Daemon Service
setup_daemon() {
    echo -e "${BLUE}[INFO] Menyiapkan systemd service untuk GarudaPanel Daemon...${NC}"
    read -rp "Masukkan URL Panel (default: http://127.0.0.1:8080): " PANEL_URL
    PANEL_URL="${PANEL_URL:-http://127.0.0.1:8080}"

    read -rp "Masukkan Join Token dari Panel (kosongkan jika sudah pernah register): " JOIN_TOKEN

    DAEMON_EXEC="${GARUDA_BIN} daemon --panel ${PANEL_URL} --data ${DATA_DIR}"
    if [ -n "${JOIN_TOKEN}" ]; then
        DAEMON_EXEC="${DAEMON_EXEC} --join ${JOIN_TOKEN}"
    fi

    cat <<EOF > "${SYSTEMD_DIR}/garuda-daemon.service"
[Unit]
Description=GarudaPanel Daemon - Node Worker Service
After=network.target docker.service
Requires=docker.service

[Service]
Type=simple
User=root
WorkingDirectory=${DATA_DIR}
ExecStart=${DAEMON_EXEC}
Restart=always
RestartSec=5s
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload
    systemctl enable garuda-daemon
    echo -e "${GREEN}[OK] Service garuda-daemon dibuat di ${SYSTEMD_DIR}/garuda-daemon.service${NC}"
    echo -e "${CYAN}Jalankan: systemctl start garuda-daemon${NC}"
}

case "${MODE}" in
    panel)
        setup_panel
        ;;
    daemon)
        setup_daemon
        ;;
    both)
        setup_panel
        setup_daemon
        ;;
esac

echo -e "\n${GREEN}Instalasi selesai!${NC}"
echo -e "Status panel:  systemctl status garuda-panel"
echo -e "Status daemon: systemctl status garuda-daemon"
