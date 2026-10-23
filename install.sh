#!/usr/bin/env bash
# GarudaPanel Interactive One-Line Installer
# Supported OS: Ubuntu / Debian (x86_64 / arm64)
set -euo pipefail

INSTALL_DIR="/usr/local/bin"
GARUDA_BIN="${INSTALL_DIR}/garuda"
DATA_DIR="/var/lib/garudapanel"
SYSTEMD_DIR="/etc/systemd/system"
RUN_USER="garuda"
GITHUB_REPO="rezzylrn/garudapanel"

# Colors for plain status
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_ok() {
    echo -e "${GREEN}[OK]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_err() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 1. Root check
check_root() {
    if [ "$(id -u)" -ne 0 ]; then
        log_err "This script must be run as root (or via sudo)."
        exit 1
    fi
}

# 2. OS detection
check_os() {
    local os_id=""
    if [ -f /etc/os-release ]; then
        # shellcheck disable=SC1091
        os_id=$(. /etc/os-release && echo "${ID:-}")
    fi

    case "${os_id}" in
        ubuntu|debian)
            log_ok "Supported OS detected: ${os_id}"
            ;;
        *)
            log_warn "Untested or unsupported operating system: '${os_id:-unknown}'."
            read -rp "GarudaPanel is built for Ubuntu/Debian. Continue anyway? [y/N]: " continue_os
            case "${continue_os}" in
                [yY][eE][sS]|[yY])
                    log_info "Proceeding on ${os_id}..."
                    ;;
                *)
                    log_err "Installation aborted by user."
                    exit 1
                    ;;
            esac
            ;;
    esac
}

# 3. System user setup
ensure_user() {
    if ! id -u "${RUN_USER}" >/dev/null 2>&1; then
        log_info "Creating system user '${RUN_USER}'..."
        useradd -r -s /usr/sbin/nologin -d "${DATA_DIR}" "${RUN_USER}" || true
    fi

    if getent group docker >/dev/null 2>&1; then
        usermod -aG docker "${RUN_USER}" || true
    fi

    mkdir -p "${DATA_DIR}"
    chown -R "${RUN_USER}:${RUN_USER}" "${DATA_DIR}"
    chmod 750 "${DATA_DIR}"
}

# 4. Binary download
download_binary() {
    mkdir -p "${INSTALL_DIR}"

    # If running from local source tree with freshly built binary
    if [ -f "./garuda" ]; then
        log_info "Using local binary './garuda' -> ${GARUDA_BIN}"
        cp -f ./garuda "${GARUDA_BIN}"
        chmod +x "${GARUDA_BIN}"
        return 0
    fi

    local arch=""
    case "$(uname -m)" in
        x86_64|amd64) arch="amd64" ;;
        aarch64|arm64) arch="arm64" ;;
        *) arch="amd64" ;;
    esac

    local download_url="https://github.com/${GITHUB_REPO}/releases/latest/download/garuda"
    log_info "Downloading GarudaPanel binary (${arch}) from ${download_url}..."

    local attempt=1
    local max_attempts=3
    local success=0

    while [ "${attempt}" -le "${max_attempts}" ]; do
        log_info "Download attempt ${attempt}/${max_attempts}..."
        if command -v curl >/dev/null 2>&1; then
            if curl -fsSL "${download_url}" -o "${GARUDA_BIN}.tmp"; then
                mv -f "${GARUDA_BIN}.tmp" "${GARUDA_BIN}"
                chmod +x "${GARUDA_BIN}"
                success=1
                break
            fi
        elif command -v wget >/dev/null 2>&1; then
            if wget -qO "${GARUDA_BIN}.tmp" "${download_url}"; then
                mv -f "${GARUDA_BIN}.tmp" "${GARUDA_BIN}"
                chmod +x "${GARUDA_BIN}"
                success=1
                break
            fi
        else
            log_err "Neither curl nor wget is installed."
            exit 1
        fi
        attempt=$((attempt + 1))
        sleep 2
    done

    if [ "${success}" -ne 1 ]; then
        rm -f "${GARUDA_BIN}.tmp"
        log_err "Failed to download GarudaPanel binary after ${max_attempts} attempts."
        exit 1
    fi

    log_ok "Binary installed to ${GARUDA_BIN}"
}

# 5. Docker dependency check & install
ensure_docker() {
    if command -v docker >/dev/null 2>&1; then
        log_ok "Docker is already installed."
        return 0
    fi

    log_warn "Docker is not installed. Daemon requires Docker to run game server containers."
    read -rp "Install Docker now via https://get.docker.com? [Y/n]: " install_docker_choice
    case "${install_docker_choice}" in
        [nN][oO]|[nN])
            log_warn "Skipping Docker installation. Note: garuda-daemon will fail to start until Docker is installed."
            ;;
        *)
            log_info "Installing Docker..."
            if command -v curl >/dev/null 2>&1; then
                curl -fsSL https://get.docker.com | sh
            elif command -v wget >/dev/null 2>&1; then
                wget -qO- https://get.docker.com | sh
            else
                log_err "Cannot install Docker: neither curl nor wget found."
                exit 1
            fi
            if command -v systemctl >/dev/null 2>&1; then
                systemctl enable --now docker || true
            fi
            if getent group docker >/dev/null 2>&1; then
                usermod -aG docker "${RUN_USER}" || true
            fi
            log_ok "Docker installed successfully."
            ;;
    esac
}

# 6. Setup Panel
setup_panel() {
    echo ""
    log_info "Configuring GarudaPanel (Web UI + API Server)..."

    read -rp "Listen address [default: :8080]: " input_addr
    local listen_addr="${input_addr:-:8080}"

    read -rp "Data directory [default: /var/lib/garudapanel]: " input_data_dir
    local panel_data_dir="${input_data_dir:-/var/lib/garudapanel}"

    mkdir -p "${panel_data_dir}"
    chown -R "${RUN_USER}:${RUN_USER}" "${panel_data_dir}"
    chmod 750 "${panel_data_dir}"

    local db_path="${panel_data_dir}/garuda-panel.db"

    # Pre-bootstrap admin if database does not exist yet to capture credentials
    local bootstrap_output=""
    if [ ! -f "${db_path}" ]; then
        log_info "Initializing database and generating bootstrap admin credentials..."
        local tmp_log
        tmp_log=$(mktemp)
        timeout 2s "${GARUDA_BIN}" panel --db "${db_path}" --addr 127.0.0.1:0 > "${tmp_log}" 2>&1 || true
        bootstrap_output=$(grep "BOOTSTRAP ADMIN" "${tmp_log}" || true)
        rm -f "${tmp_log}"
        chown "${RUN_USER}:${RUN_USER}" "${db_path}" 2>/dev/null || true
    fi

    log_info "Writing ${SYSTEMD_DIR}/garuda-panel.service..."
    cat <<EOF > "${SYSTEMD_DIR}/garuda-panel.service"
[Unit]
Description=GarudaPanel - Game Server Management Panel
After=network.target

[Service]
Type=simple
User=${RUN_USER}
Group=${RUN_USER}
WorkingDirectory=${panel_data_dir}
ExecStart=${GARUDA_BIN} panel --addr ${listen_addr} --db ${db_path}
Restart=always
RestartSec=5s
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload
    systemctl enable garuda-panel
    systemctl restart garuda-panel
    log_ok "garuda-panel.service enabled and started."

    PANEL_SUMMARY_ADDR="${listen_addr}"
    PANEL_BOOTSTRAP_OUTPUT="${bootstrap_output}"
}

# 7. Setup Daemon
setup_daemon() {
    echo ""
    log_info "Configuring GarudaPanel Daemon (Node Worker)..."
    ensure_docker

    read -rp "Panel URL [default: http://127.0.0.1:8080]: " input_panel_url
    local panel_url="${input_panel_url:-http://127.0.0.1:8080}"

    read -rp "Join token from Panel [leave empty if node is already registered]: " input_join_token

    read -rp "Daemon data directory [default: /var/lib/garudapanel]: " input_daemon_dir
    local daemon_data_dir="${input_daemon_dir:-/var/lib/garudapanel}"

    mkdir -p "${daemon_data_dir}/volumes"

    local daemon_exec="${GARUDA_BIN} daemon --panel ${panel_url} --data ${daemon_data_dir}"
    if [ -n "${input_join_token}" ]; then
        daemon_exec="${daemon_exec} --join ${input_join_token}"
    fi

    log_info "Writing ${SYSTEMD_DIR}/garuda-daemon.service..."
    cat <<EOF > "${SYSTEMD_DIR}/garuda-daemon.service"
[Unit]
Description=GarudaPanel Daemon - Node Worker Service
After=network.target docker.service
Requires=docker.service

[Service]
Type=simple
User=root
WorkingDirectory=${daemon_data_dir}
ExecStart=${daemon_exec}
Restart=always
RestartSec=5s
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload
    systemctl enable garuda-daemon
    systemctl restart garuda-daemon || log_warn "garuda-daemon service started (may wait for valid join token or Docker)."
    log_ok "garuda-daemon.service enabled and started."

    DAEMON_SUMMARY_PANEL="${panel_url}"
}

# 8. Uninstaller
uninstall_all() {
    echo ""
    log_warn "Starting uninstallation of GarudaPanel..."

    if systemctl is-active --quiet garuda-panel 2>/dev/null; then
        log_info "Stopping garuda-panel.service..."
        systemctl stop garuda-panel || true
    fi
    if systemctl is-enabled --quiet garuda-panel 2>/dev/null; then
        systemctl disable garuda-panel || true
    fi
    rm -f "${SYSTEMD_DIR}/garuda-panel.service"

    if systemctl is-active --quiet garuda-daemon 2>/dev/null; then
        log_info "Stopping garuda-daemon.service..."
        systemctl stop garuda-daemon || true
    fi
    if systemctl is-enabled --quiet garuda-daemon 2>/dev/null; then
        systemctl disable garuda-daemon || true
    fi
    rm -f "${SYSTEMD_DIR}/garuda-daemon.service"

    systemctl daemon-reload 2>/dev/null || true

    if [ -f "${GARUDA_BIN}" ]; then
        log_info "Removing binary ${GARUDA_BIN}..."
        rm -f "${GARUDA_BIN}"
    fi

    read -rp "Delete all databases and volume data in ${DATA_DIR}? [y/N]: " del_data
    case "${del_data}" in
        [yY][eE][sS]|[yY])
            rm -rf "${DATA_DIR}"
            log_ok "Data directory removed."
            ;;
        *)
            log_info "Data directory preserved at ${DATA_DIR}."
            ;;
    esac

    log_ok "GarudaPanel has been uninstalled."
    exit 0
}

# Entrypoint
main() {
    local action="${1:-}"

    if [ "${action}" = "--uninstall" ] || [ "${action}" = "uninstall" ]; then
        check_root
        uninstall_all
    fi

    check_root
    check_os

    echo "=================================================="
    echo "         GarudaPanel Interactive Installer        "
    echo "=================================================="
    echo ""
    echo "Select component to install:"
    echo "  1) Panel only (Web UI + API Server)"
    echo "  2) Daemon only (Node Worker Service)"
    echo "  3) All-in-one (Panel + Daemon on this server)"
    echo "  4) Uninstall GarudaPanel"
    echo ""

    local choice=""
    while true; do
        read -rp "Enter choice [1-4]: " choice
        case "${choice}" in
            1) mode="panel"; break ;;
            2) mode="daemon"; break ;;
            3) mode="both"; break ;;
            4) uninstall_all ;;
            *) echo "Invalid choice. Please enter 1, 2, 3, or 4." ;;
        esac
    done

    ensure_user
    download_binary

    PANEL_SUMMARY_ADDR=""
    PANEL_BOOTSTRAP_OUTPUT=""
    DAEMON_SUMMARY_PANEL=""

    case "${mode}" in
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

    echo ""
    echo "=================================================="
    echo "              Installation Complete               "
    echo "=================================================="
    echo ""

    if [ "${mode}" = "panel" ] || [ "${mode}" = "both" ]; then
        echo "Panel Service:"
        echo "  Status:  active (garuda-panel.service)"
        echo "  Listen:  ${PANEL_SUMMARY_ADDR}"
        if [ -n "${PANEL_BOOTSTRAP_OUTPUT}" ]; then
            echo ""
            echo "Admin Credentials:"
            echo "  ${PANEL_BOOTSTRAP_OUTPUT}"
            echo "  (Save this password now; it will not be displayed again)"
        else
            echo "  Admin credentials: see journalctl -u garuda-panel"
        fi
        echo "  Logs:    journalctl -u garuda-panel -f"
        echo ""
    fi

    if [ "${mode}" = "daemon" ] || [ "${mode}" = "both" ]; then
        echo "Daemon Service:"
        echo "  Status:  active (garuda-daemon.service)"
        echo "  Panel:   ${DAEMON_SUMMARY_PANEL}"
        echo "  Logs:    journalctl -u garuda-daemon -f"
        echo ""
    fi

    echo "Uninstall command:"
    echo "  curl -fsSL https://github.com/${GITHUB_REPO}/releases/latest/download/install.sh | bash -s -- --uninstall"
    echo "  or locally: sudo /usr/local/bin/install.sh --uninstall"
    echo "=================================================="
}

main "$@"
