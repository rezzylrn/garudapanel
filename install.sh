#!/usr/bin/env bash
# GarudaPanel Installer & Manager (SPEC §5.10, §1.1, §12)
# Single static binary installer, systemd service setup, and clean uninstaller.
set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

echo -e "${CYAN}====================================================${NC}"
echo -e "${CYAN}             GarudaPanel Setup & Installer          ${NC}"
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
RUN_USER="garudapanel"

# Pastikan user sistem 'garudapanel' tersedia untuk menjalankan service dengan hak aman
ensure_user() {
    if ! id -u "${RUN_USER}" >/dev/null 2>&1; then
        echo -e "${BLUE}[INFO] Membuat system user '${RUN_USER}'...${NC}"
        useradd -r -s /usr/sbin/nologin -d "${DATA_DIR}" "${RUN_USER}" || true
    fi

    # Daemon butuh akses ke docker.sock bila mode daemon aktif
    if getent group docker >/dev/null 2>&1; then
        usermod -aG docker "${RUN_USER}" || true
    fi

    mkdir -p "${DATA_DIR}" "${CONFIG_DIR}"
    chown -R "${RUN_USER}:${RUN_USER}" "${DATA_DIR}" "${CONFIG_DIR}"
    chmod 750 "${DATA_DIR}" "${CONFIG_DIR}"
}

# Fungsi uninstall bersih
uninstall_all() {
    echo -e "${YELLOW}[WARN] Memulai proses uninstall GarudaPanel...${NC}"
    
    if systemctl is-active --quiet garuda-panel 2>/dev/null; then
        echo -e "${BLUE}[INFO] Menghentikan garuda-panel.service...${NC}"
        systemctl stop garuda-panel || true
    fi
    if systemctl is-enabled --quiet garuda-panel 2>/dev/null; then
        systemctl disable garuda-panel || true
    fi
    rm -f "${SYSTEMD_DIR}/garuda-panel.service"

    if systemctl is-active --quiet garuda-daemon 2>/dev/null; then
        echo -e "${BLUE}[INFO] Menghentikan garuda-daemon.service...${NC}"
        systemctl stop garuda-daemon || true
    fi
    if systemctl is-enabled --quiet garuda-daemon 2>/dev/null; then
        systemctl disable garuda-daemon || true
    fi
    rm -f "${SYSTEMD_DIR}/garuda-daemon.service"

    systemctl daemon-reload

    if [ -f "${GARUDA_BIN}" ]; then
        echo -e "${BLUE}[INFO] Menghapus binary ${GARUDA_BIN}...${NC}"
        rm -f "${GARUDA_BIN}"
    fi

    echo -en "${YELLOW}Hapus data database & volume di ${DATA_DIR}? (y/N): ${NC}"
    read -r DEL_DATA
    if [[ "${DEL_DATA}" =~ ^[Yy]$ ]]; then
        rm -rf "${DATA_DIR}" "${CONFIG_DIR}"
        echo -e "${GREEN}[OK] Direktori data dihapus.${NC}"
    else
        echo -e "${BLUE}[INFO] Data di ${DATA_DIR} dipertahankan.${NC}"
    fi

    echo -e "${GREEN}[OK] GarudaPanel berhasil di-uninstall sepenuhnya!${NC}"
    exit 0
}

MODE="${1:-}"

if [ "${MODE}" = "uninstall" ]; then
    uninstall_all
fi

if [ -z "${MODE}" ]; then
    echo "Pilih opsi:"
    echo "  1) Install Panel (Web UI + API Server)"
    echo "  2) Install Daemon (Worker Node Service)"
    echo "  3) Install Keduanya (All-in-one di satu mesin)"
    echo "  4) Uninstall GarudaPanel (Service & Binary)"
    read -rp "Pilihan [1/2/3/4]: " CHOICE
    case "${CHOICE}" in
        1) MODE="panel" ;;
        2) MODE="daemon" ;;
        3) MODE="both" ;;
        4) uninstall_all ;;
        *) echo -e "${RED}Pilihan tidak valid.${NC}"; exit 1 ;;
    esac
fi

ensure_user

# 1. Pastikan binary garuda terinstall
if [ -f "./garuda" ]; then
    echo -e "${BLUE}[INFO] Menyalin binary garuda lokal ke ${GARUDA_BIN}...${NC}"
    cp -f ./garuda "${GARUDA_BIN}"
    chmod +x "${GARUDA_BIN}"
elif command -v garuda >/dev/null 2>&1; then
    echo -e "${BLUE}[INFO] Binary garuda sudah ada di sistem PATH.${NC}"
else
    echo -e "${BLUE}[INFO] Binary lokal tidak ditemukan, mencoba compile dengan Go...${NC}"
    if command -v go >/dev/null 2>&1; then
        echo -e "${BLUE}[INFO] Mengompilasi binary statis dengan CGO_ENABLED=0 go build...${NC}"
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
    read -rp "Masukkan port listen panel (default: 8080): " INPUT_PORT
    PANEL_PORT="${INPUT_PORT:-8080}"
    PANEL_DB="${DATA_DIR}/garuda-panel.db"

    cat <<EOF > "${SYSTEMD_DIR}/garuda-panel.service"
[Unit]
Description=GarudaPanel - Game Server Management Panel
After=network.target

[Service]
Type=simple
User=${RUN_USER}
Group=${RUN_USER}
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

echo -e "\n${GREEN}====================================================${NC}"
echo -e "${GREEN}             Instalasi Selesai!                     ${NC}"
echo -e "${GREEN}====================================================${NC}"
if [ "${MODE}" = "panel" ] || [ "${MODE}" = "both" ]; then
    echo -e "Jalankan Panel:   ${CYAN}sudo systemctl start garuda-panel${NC}"
    echo -e "Cek Log Panel:    ${CYAN}sudo journalctl -u garuda-panel -f${NC}"
    echo -e "Catatan: Log admin bootstrap awal dapat dilihat di journalctl saat panel pertama kali jalan."
fi
if [ "${MODE}" = "daemon" ] || [ "${MODE}" = "both" ]; then
    echo -e "Jalankan Daemon:  ${CYAN}sudo systemctl start garuda-daemon${NC}"
    echo -e "Cek Log Daemon:   ${CYAN}sudo journalctl -u garuda-daemon -f${NC}"
fi
echo -e "\nUntuk uninstall di kemudian hari, jalankan: ${YELLOW}sudo ./install.sh uninstall${NC}"
