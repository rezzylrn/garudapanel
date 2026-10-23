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
    if [[ $EUID -ne 0 ]]; then
        log_err "This installer must be run as root:"
        echo "  curl -fsSL https://github.com/${GITHUB_REPO}/releases/latest/download/install.sh -o /tmp/gp-install.sh && sudo bash /tmp/gp-install.sh"
        exit 1
    fi
}

# 2. OS detection
check_os() {
    local os_id=""
    local os_id_like=""
    if [ -f /etc/os-release ]; then
        # shellcheck disable=SC1091
        eval "$(grep -E '^(ID|ID_LIKE)=' /etc/os-release)"
        os_id="${ID:-}"
        os_id_like="${ID_LIKE:-}"
    fi

    local is_supported=0
    case "${os_id}" in
        ubuntu|debian|pop|linuxmint)
            is_supported=1
            ;;
    esac

    if [ "${is_supported}" -eq 0 ]; then
        case " ${os_id_like} " in
            *" ubuntu "*|*" debian "*)
                is_supported=1
                ;;
        esac
    fi

    if [ "${is_supported}" -eq 1 ]; then
        log_ok "Supported OS detected: ${os_id}${os_id_like:+ (derived from ${os_id_like})}"
    else
        log_warn "Untested or unsupported operating system: '${os_id:-unknown}'."
        read -rp "GarudaPanel is built for Ubuntu/Debian and derivatives. Continue anyway? [y/N]: " continue_os
        case "${continue_os}" in
            [yY][eE][sS]|[yY])
                log_info "Proceeding on ${os_id}..."
                ;;
            *)
                log_err "Installation aborted by user."
                exit 1
                ;;
        esac
    fi
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
    local asset_name="garuda"
    case "$(uname -m)" in
        x86_64|amd64)
            arch="amd64"
            asset_name="garuda"
            ;;
        aarch64|arm64)
            arch="arm64"
            asset_name="garuda-linux-arm64"
            ;;
        *)
            arch="amd64"
            asset_name="garuda"
            ;;
    esac

    local download_url="https://github.com/${GITHUB_REPO}/releases/latest/download/${asset_name}"
    log_info "Downloading GarudaPanel binary (${arch}: ${asset_name}) from ${download_url}..."

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
install_docker() {
    log_info "Installing Docker..."

    # Pastikan dependensi awal terpasang
    apt-get update -y || true
    apt-get install -y ca-certificates curl gnupg lsb-release || true

    local os_id=""
    local os_id_like=""
    if [ -f /etc/os-release ]; then
        # shellcheck disable=SC1091
        eval "$(grep -E '^(ID|ID_LIKE)=' /etc/os-release)"
        os_id="${ID:-}"
        os_id_like="${ID_LIKE:-}"
    fi

    # Deteksi turunan Ubuntu (mis. Pop!_OS, Linux Mint, Elementary)
    if [ "${os_id}" = "ubuntu" ] || [[ "${os_id_like}" == *ubuntu* ]]; then
        local codename="${UBUNTU_CODENAME:-}"
        if [ -z "${codename}" ] && command -v lsb_release >/dev/null 2>&1; then
            codename="$(lsb_release -cs)"
        fi
        log_info "Ubuntu/Ubuntu derivative detected (${os_id}, codename: ${codename:-unknown}). Using Docker Ubuntu repository..."

        # Hapus repo docker lama/rusak (misal sisa run get.docker.com yang salah deteksi trixie)
        rm -f /etc/apt/sources.list.d/docker.list /etc/apt/sources.list.d/docker.list.save

        install -m 0755 -d /etc/apt/keyrings
        curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc || true
        chmod a+r /etc/apt/keyrings/docker.asc || true

        local dpkg_arch
        dpkg_arch="$(dpkg --print-architecture 2>/dev/null || echo amd64)"

        echo "deb [arch=${dpkg_arch} signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu ${codename} stable" > /etc/apt/sources.list.d/docker.list

        if apt-get update -y && apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin; then
            log_ok "Docker CE installed from official Ubuntu repository."
        else
            log_warn "Official Docker repository install failed, attempting fallback to docker.io..."
            rm -f /etc/apt/sources.list.d/docker.list
            apt-get update -y || true
            apt-get install -y docker.io || true
        fi
    else
        # Non-Ubuntu (Debian native, dll.) -> coba get.docker.com
        rm -f /etc/apt/sources.list.d/docker.list /etc/apt/sources.list.d/docker.list.save
        if ! (curl -fsSL https://get.docker.com | sh); then
            log_warn "get.docker.com failed, attempting fallback to docker.io..."
            apt-get update -y || true
            apt-get install -y docker.io || true
        fi
    fi

    if command -v systemctl >/dev/null 2>&1; then
        systemctl enable --now docker || true
    fi
    if getent group docker >/dev/null 2>&1; then
        usermod -aG docker "${RUN_USER}" || true
    fi

    if docker info >/dev/null 2>&1; then
        log_ok "Docker verified successfully (docker info ok)."
    else
        log_warn "Docker install verification failed. Please install Docker manually: sudo apt install -y docker.io"
    fi
}

ensure_docker() {
    if command -v docker >/dev/null 2>&1 && docker info >/dev/null 2>&1; then
        log_ok "Docker is already installed and running."
        return 0
    fi

    log_warn "Docker is not installed or not running. Daemon requires Docker to run game server containers."
    read -rp "Install Docker now? [Y/n]: " install_docker_choice
    case "${install_docker_choice}" in
        [nN][oO]|[nN])
            log_warn "Skipping Docker installation. Note: garuda-daemon will fail to start until Docker is installed."
            ;;
        *)
            # Non-fatal wrapper agar gagal install docker tidak meng-abort script (set -e)
            set +e
            install_docker
            set -e
            ;;
    esac
}

# 6. Setup Panel
setup_admin_user() {
    local db_path="${1}"
    echo ""
    log_info "Administrator account setup"
    echo "  Create the administrator account for the GarudaPanel web interface."
    echo ""

    local admin_user=""
    local admin_email=""
    local admin_pass=""
    local admin_pass2=""

    while [ -z "${admin_user}" ]; do
        read -rp "  Username [default: admin]: " admin_user
        admin_user="${admin_user:-admin}"
    done

    while true; do
        read -rp "  Email address: " admin_email
        if [[ "${admin_email}" == *"@"* ]]; then
            break
        fi
        log_warn "Please enter a valid email address."
    done

    while true; do
        read -s -rp "  Password (min 8 characters): " admin_pass
        echo ""
        if [ "${#admin_pass}" -lt 8 ]; then
            log_warn "Password must be at least 8 characters."
            continue
        fi
        read -s -rp "  Confirm password: " admin_pass2
        echo ""
        if [ "${admin_pass}" != "${admin_pass2}" ]; then
            log_warn "Passwords do not match. Please try again."
            continue
        fi
        break
    done

    log_info "Creating administrator account '${admin_user}'..."
    local create_out
    create_out=$("${GARUDA_BIN}" admin create-user --db "${db_path}" \
        --username "${admin_user}" --email "${admin_email}" --password "${admin_pass}" 2>&1) || true

    if echo "${create_out}" | grep -qi "already exists"; then
        log_warn "A user with this username or email already exists. Skipping account creation."
        log_info "Log in with your existing administrator account."
    elif echo "${create_out}" | grep -q "USER CREATED"; then
        log_ok "Administrator account '${admin_user}' created successfully."
    else
        log_warn "Account creation returned an unexpected result:"
        echo "  ${create_out}"
    fi

    chown "${RUN_USER}:${RUN_USER}" "${db_path}" 2>/dev/null || true
    PANEL_ADMIN_USER="${admin_user}"
}

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

    # Administrator account is always set up interactively (pterodactyl-style).
    setup_admin_user "${db_path}"

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
    PANEL_DB_PATH="${db_path}"
}

# 7. Setup Daemon
setup_daemon() {
    local install_mode="${1:-daemon}"
    echo ""
    log_info "Configuring GarudaPanel Daemon (Node Worker)..."
    ensure_docker

    read -rp "Panel URL [default: http://127.0.0.1:8080]: " input_panel_url
    local panel_url="${input_panel_url:-http://127.0.0.1:8080}"

    read -rp "Daemon data directory [default: /var/lib/garudapanel]: " input_daemon_dir
    local daemon_data_dir="${input_daemon_dir:-/var/lib/garudapanel}"

    mkdir -p "${daemon_data_dir}/volumes"

    local has_token=0
    if [ -f "${daemon_data_dir}/node-token" ]; then
        has_token=1
        log_info "Existing node-token found at ${daemon_data_dir}/node-token."
    fi

    local input_join_token=""

    if [ "${install_mode}" = "both" ]; then
        if [ "${has_token}" -eq 1 ]; then
            log_info "Node is already registered (node-token exists). Skipping automatic node provisioning."
        else
            log_info "Waiting for GarudaPanel to be healthy before auto-joining daemon..."
            local wait_secs=0
            local max_wait=30
            local healthy=0
            while [ "${wait_secs}" -lt "${max_wait}" ]; do
                if curl -sf "http://127.0.0.1:8080/" >/dev/null 2>&1; then
                    healthy=1
                    break
                fi
                sleep 1
                wait_secs=$((wait_secs + 1))
            done

            if [ "${healthy}" -eq 1 ]; then
                log_ok "GarudaPanel is healthy."
            else
                log_warn "Panel did not respond healthy on http://127.0.0.1:8080/ within ${max_wait}s. Attempting provisioning anyway..."
            fi

            local node_name="node-$(hostname)"
            local db_target="${PANEL_DB_PATH:-/var/lib/garudapanel/garuda-panel.db}"
            log_info "Auto-provisioning node '${node_name}' via DB (${db_target})..."
            input_join_token=$("${GARUDA_BIN}" admin provision-node --db "${db_target}" --name "${node_name}" 2>/dev/null || true)

            if [ -n "${input_join_token}" ]; then
                log_ok "Generated join token for '${node_name}'."
            else
                log_warn "Failed to auto-provision node via 'garuda admin provision-node'."
            fi
        fi
    else
        if [ "${has_token}" -eq 1 ]; then
            log_info "Node already has node-token. Press enter to skip join, or provide a new token."
        fi
        read -rp "Join token from Panel [leave empty if node is already registered]: " input_join_token
    fi

    # One-shot join if token provided
    if [ -n "${input_join_token}" ]; then
        log_info "Performing one-shot node registration with join token..."
        # Daemon one-shot join: run with timeout to register and persist node-token
        timeout 5s "${GARUDA_BIN}" daemon --panel "${panel_url}" --data "${daemon_data_dir}" --join "${input_join_token}" >/dev/null 2>&1 || true

        if [ -f "${daemon_data_dir}/node-token" ]; then
            log_ok "Node registration successful. Saved node token to ${daemon_data_dir}/node-token."
            has_token=1
        else
            log_warn "One-shot join did not create node-token (panel might be unreachable or token invalid)."
            log_warn "You can retry join manually later: ${GARUDA_BIN} daemon --panel ${panel_url} --data ${daemon_data_dir} --join <TOKEN>"
        fi
    elif [ "${has_token}" -eq 0 ]; then
        echo ""
        log_warn "=========================================================="
        log_warn "WARNING: No join token provided and no node-token exists."
        log_warn "garuda-daemon will NOT run until the node is joined manually."
        log_warn "Run this command once to register the node:"
        log_warn "  ${GARUDA_BIN} daemon --panel ${panel_url} --data ${daemon_data_dir} --join <TOKEN>"
        log_warn "Then restart the daemon:"
        log_warn "  systemctl restart garuda-daemon"
        log_warn "=========================================================="
        echo ""
    fi

    # Systemd unit MUST NOT contain --join token (join token is single-use)
    local daemon_exec="${GARUDA_BIN} daemon --panel ${panel_url} --data ${daemon_data_dir}"

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
    if [ "${has_token}" -eq 1 ]; then
        systemctl restart garuda-daemon || log_warn "garuda-daemon service started (waiting for Docker or network)."
        log_ok "garuda-daemon.service enabled and started."
    else
        log_warn "garuda-daemon.service enabled, but not started yet (waiting for node registration)."
    fi

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
    PANEL_ADMIN_USER=""
    DAEMON_SUMMARY_PANEL=""

    case "${mode}" in
        panel)
            setup_panel
            ;;
        daemon)
            setup_daemon "daemon"
            ;;
        both)
            setup_panel
            setup_daemon "both"
            ;;
    esac

    echo ""
    echo "=================================================="
    echo "              Installation Complete               "
    echo "=================================================="
    echo ""

    if [ "${mode}" = "panel" ] || [ "${mode}" = "both" ]; then
        echo "Panel Service:"
        echo "  Status:   active (garuda-panel.service)"
        echo "  Listen:   ${PANEL_SUMMARY_ADDR}"
        echo "  Admin:    ${PANEL_ADMIN_USER} (use the password you set during installation)"
        echo "  Logs:     journalctl -u garuda-panel -f"
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
    echo "  curl -fsSL https://github.com/${GITHUB_REPO}/releases/latest/download/install.sh -o /tmp/gp-install.sh && sudo bash /tmp/gp-install.sh --uninstall"
    echo "  or locally: sudo /usr/local/bin/install.sh --uninstall"
    echo "=================================================="
}

main "$@"
