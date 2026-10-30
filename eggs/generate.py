#!/usr/bin/env python3
"""Generate GarudaPanel egg library (PTDL_v2 JSON) from a compact spec.

Usage: python3 generate.py  -> writes eggs/*.json next to this script.
Setiap egg: name, docker_images (yolks), startup, config.startup.done,
config.stop, variables, install script (jalan di mode Docker; di mode
host-process file server di-upload lewat file manager).
"""
import json
import os

HERE = os.path.dirname(os.path.abspath(__file__))
AUTHOR = "GarudaPanel"


def var(name, desc, env, default="", viewable=True, editable=True):
    return {
        "name": name,
        "description": desc,
        "env_variable": env,
        "default_value": default,
        "user_viewable": viewable,
        "user_editable": editable,
        "rules": "required|string|max:191" if default == "" and env not in (
            "GIT_ADDRESS", "BRANCH", "USERNAME", "ACCESS_TOKEN", "BOT_TOKEN",
            "SERVER_PASSWORD", "FIVEM_LICENSE_KEY") else "nullable|string|max:191",
    }


def egg(name, desc, images, startup, done, stop, variables, install_script,
        install_container="ghcr.io/parkervcp/installers:debian"):
    return {
        "meta": {"version": "PTDL_v2", "update_url": None},
        "exported_at": "2026-10-30T00:00:00+07:00",
        "name": name,
        "author": AUTHOR,
        "description": desc,
        "features": None,
        "docker_images": images,
        "file_denylist": [],
        "startup": startup,
        "config": {
            "files": "{}",
            "startup": {"done": done, "user_interaction": []},
            "logs": {"custom": False, "location": "logs/latest.log"},
            "stop": stop,
        },
        "scripts": {
            "installation": {
                "script": install_script,
                "container": install_container,
                "entrypoint": "bash",
            }
        },
        "variables": variables,
    }


EGGS = {
    "Minecraft": [
        egg("Paper", "Minecraft Java server (PaperMC) — cepat, plugin-friendly.",
            {"Java 21": "ghcr.io/parkervcp/yolks:java_21",
             "Java 17": "ghcr.io/parkervcp/yolks:java_17"},
            "java -Xms128M -XX:MaxRAMPercentage=95.0 -Dterminal.jline=false "
            "-Dterminal.ansi=true -jar {{SERVER_JARFILE}} nogui",
            "Done (", "stop",
            [var("Server Jar File", "Nama file jar server.", "SERVER_JARFILE", "server.jar"),
             var("Minecraft Version", "Versi Minecraft.", "MINECRAFT_VERSION", "1.21.1"),
             var("Paper Build", "Nomor build Paper (latest = terbaru).", "BUILD_NUMBER", "latest")],
            """#!/bin/bash
# Paper install: unduh jar terbaru dari PaperMC API.
cd /mnt/server
MC=${MINECRAFT_VERSION:-1.21.1}
if [ "${BUILD_NUMBER}" = "latest" ]; then
  BUILD=$(curl -s "https://api.papermc.io/v2/projects/paper/versions/${MC}/builds" | python3 -c "import sys,json;print(json.load(sys.stdin)['builds'][-1]['build'])" 2>/dev/null || echo "")
else
  BUILD=${BUILD_NUMBER}
fi
curl -fsSL -o server.jar "https://api.papermc.io/v2/projects/paper/versions/${MC}/builds/${BUILD}/downloads/paper-${MC}-${BUILD}.jar"
echo "eula=true" > eula.txt
"""),
        egg("Bedrock", "Minecraft Bedrock dedicated server (Windows/Linux/mobile bisa join).",
            {"Bedrock": "ghcr.io/parkervcp/yolks:debian"},
            "./bedrock_server",
            "Server started.", "^C",
            [var("Bedrock Version", "Versi Bedrock server.", "BEDROCK_VERSION", "1.21.50")],
            """#!/bin/bash
cd /mnt/server
# Unduh Bedrock server zip (URL resmi berubah tiap rilis; sesuaikan versi).
curl -fsSL -o bedrock.zip "https://minecraft.azureedge.net/bin-linux/bedrock-server-${BEDROCK_VERSION}.zip"
unzip -o bedrock.zip && rm bedrock.zip
chmod +x bedrock_server
"""),
        egg("Forge", "Minecraft Java server dengan mod Forge.",
            {"Java 17": "ghcr.io/parkervcp/yolks:java_17",
             "Java 21": "ghcr.io/parkervcp/yolks:java_21"},
            "java -Xms128M -XX:MaxRAMPercentage=95.0 -jar {{SERVER_JARFILE}} nogui",
            "Done (", "stop",
            [var("Server Jar File", "Nama file jar Forge.", "SERVER_JARFILE", "forge-1.20.1-47.2.0-universal.jar"),
             var("Forge Version", "Versi Forge (mc-forge).", "FORGE_VERSION", "1.20.1-47.2.0")],
            """#!/bin/bash
cd /mnt/server
MC=$(echo "${FORGE_VERSION}" | cut -d- -f1)
FV=$(echo "${FORGE_VERSION}" | cut -d- -f2-)
curl -fsSL -o forge-installer.jar "https://maven.minecraftforge.net/net/minecraftforge/forge/${MC}-${FV}/forge-${MC}-${FV}-installer.jar"
java -jar forge-installer.jar --installServer
rm forge-installer.jar
echo "eula=true" > eula.txt
"""),
    ],
    "Node.js": [
        egg("Node.js", "Aplikasi Node.js generik (Express, Fastify, bot, API).",
            {"Node.js 20": "ghcr.io/parkervcp/yolks:nodejs_20",
             "Node.js 22": "ghcr.io/parkervcp/yolks:nodejs_22"},
            "{{CMD_RUN}}", "", "^C",
            [var("Run Command", "Perintah jalanin app.", "CMD_RUN", "npm start"),
             var("Git Address", "Repo git (kosong = upload manual).", "GIT_ADDRESS", ""),
             var("Branch", "Branch git.", "BRANCH", "main"),
             var("Username", "Username git (repo privat).", "USERNAME", ""),
             var("Access Token", "Token git (repo privat).", "ACCESS_TOKEN", "", True, True)],
            """#!/bin/bash
cd /mnt/server
if [ -n "${GIT_ADDRESS}" ]; then git clone --branch "${BRANCH:-main}" "${GIT_ADDRESS}" . 2>/dev/null || git pull; fi
[ -f package.json ] && npm install --no-audit --no-fund
"""),
        egg("Discord.js Bot", "Bot Discord (discord.js) — jalanin pakai npm start.",
            {"Node.js 20": "ghcr.io/parkervcp/yolks:nodejs_20",
             "Node.js 22": "ghcr.io/parkervcp/yolks:nodejs_22"},
            "{{CMD_RUN}}", "", "^C",
            [var("Run Command", "Perintah jalanin bot.", "CMD_RUN", "npm start"),
             var("Bot Token", "Token bot Discord.", "BOT_TOKEN", "", False, True),
             var("Git Address", "Repo git bot.", "GIT_ADDRESS", ""),
             var("Branch", "Branch git.", "BRANCH", "main")],
            """#!/bin/bash
cd /mnt/server
if [ -n "${GIT_ADDRESS}" ]; then git clone --branch "${BRANCH:-main}" "${GIT_ADDRESS}" . 2>/dev/null || git pull; fi
[ -f package.json ] && npm install --no-audit --no-fund
"""),
    ],
    "Python": [
        egg("Python", "Aplikasi Python generik (script, bot, worker).",
            {"Python 3.12": "ghcr.io/parkervcp/yolks:python_3.12"},
            "python3 {{MAIN_FILE}}", "", "^C",
            [var("Main File", "File utama yang dijalankan.", "MAIN_FILE", "main.py"),
             var("Requirements File", "File requirements pip.", "REQUIREMENTS_FILE", "requirements.txt")],
            """#!/bin/bash
cd /mnt/server
[ -f "${REQUIREMENTS_FILE:-requirements.txt}" ] && pip install -r "${REQUIREMENTS_FILE:-requirements.txt}"
"""),
        egg("Django", "Web app Django (runserver dev).",
            {"Python 3.12": "ghcr.io/parkervcp/yolks:python_3.12"},
            "python3 manage.py runserver 0.0.0.0:{{SERVER_PORT}}",
            "Starting development server", "^C",
            [var("Requirements File", "File requirements pip.", "REQUIREMENTS_FILE", "requirements.txt")],
            """#!/bin/bash
cd /mnt/server
[ -f "${REQUIREMENTS_FILE:-requirements.txt}" ] && pip install -r "${REQUIREMENTS_FILE:-requirements.txt}"
"""),
        egg("Flask", "Web app Flask (gunicorn).",
            {"Python 3.12": "ghcr.io/parkervcp/yolks:python_3.12"},
            "gunicorn --bind 0.0.0.0:{{SERVER_PORT}} {{APP_MODULE}}:app",
            "Listening at", "^C",
            [var("App Module", "Modul Flask (tanpa .py).", "APP_MODULE", "app"),
             var("Requirements File", "File requirements pip.", "REQUIREMENTS_FILE", "requirements.txt")],
            """#!/bin/bash
cd /mnt/server
pip install gunicorn
[ -f "${REQUIREMENTS_FILE:-requirements.txt}" ] && pip install -r "${REQUIREMENTS_FILE:-requirements.txt}"
"""),
    ],
    "Web Hosting": [
        egg("PHP Website", "Website PHP standar (shared hosting style) — php built-in server.",
            {"PHP 8.2": "ghcr.io/parkervcp/yolks:php_8.2"},
            "php -S 0.0.0.0:{{SERVER_PORT}} -t {{DOC_ROOT}}",
            "Development Server", "^C",
            [var("Document Root", "Folder document root.", "DOC_ROOT", ".")],
            "#!/bin/bash\ncd /mnt/server\necho 'Upload file PHP kamu lewat file manager.'\n"),
        egg("Static Website", "Website statis (HTML/CSS/JS) — python http.server.",
            {"Python 3.12": "ghcr.io/parkervcp/yolks:python_3.12"},
            "python3 -m http.server {{SERVER_PORT}}",
            "Serving HTTP", "^C",
            [var("Document Root", "Folder document root.", "DOC_ROOT", ".")],
            "#!/bin/bash\ncd /mnt/server\necho 'Upload file statis kamu lewat file manager.'\n"),
        egg("Laravel", "Web app Laravel (artisan serve).",
            {"PHP 8.2": "ghcr.io/parkervcp/yolks:php_8.2"},
            "php artisan serve --host 0.0.0.0 --port {{SERVER_PORT}}",
            "Server running on", "^C",
            [var("Requirements", "Jalankan composer install saat install.", "COMPOSER_INSTALL", "1")],
            """#!/bin/bash
cd /mnt/server
if [ -f composer.json ]; then composer install --no-dev --optimize-autoloader; fi
[ -f .env.example ] && [ ! -f .env ] && cp .env.example .env && php artisan key:generate
"""),
    ],
    "Game Servers": [
        egg("Terraria", "Terraria dedicated server (TShock-ready).",
            {"Terraria": "ghcr.io/parkervcp/yolks:terraria"},
            "./TerrariaServer -config serverconfig.txt",
            "", "^C",
            [var("World Name", "Nama world.", "WORLD_NAME", "world"),
             var("Max Players", "Slot pemain.", "MAX_PLAYERS", "8")],
            """#!/bin/bash
cd /mnt/server
# Unduh Terraria server terbaru (versi di URL berubah tiap rilis).
curl -fsSL -o terraria.zip "https://terraria.org/api/download/pc-dedicated-server"
unzip -o terraria.zip
cp -r */Linux/* . 2>/dev/null || true
chmod +x TerrariaServer
"""),
        egg("Valheim", "Valheim dedicated server.",
            {"Valheim": "ghcr.io/parkervcp/yolks:valheim"},
            "./valheim_server.x86_64 -name \"{{SERVER_NAME}}\" -port {{SERVER_PORT}} "
            "-world \"{{WORLD_NAME}}\" -password \"{{SERVER_PASSWORD}}\" -public 1",
            "Game server connected", "^C",
            [var("Server Name", "Nama server di browser.", "SERVER_NAME", "Garuda Valheim"),
             var("World Name", "Nama world.", "WORLD_NAME", "Dedicated"),
             var("Server Password", "Password join (min 5 karakter).", "SERVER_PASSWORD", "garuda123")],
            """#!/bin/bash
cd /mnt/server
steamcmd +force_install_dir /mnt/server +login anonymous +app_update 896660 validate +quit
"""),
        egg("Rust", "Rust dedicated server (Facepunch).",
            {"Rust": "ghcr.io/parkervcp/yolks:rust"},
            "./RustDedicated -batchmode +server.port {{SERVER_PORT}} "
            "+server.hostname \"{{SERVER_NAME}}\" +server.identity \"server\" "
            "+server.maxplayers {{MAX_PLAYERS}} +server.worldsize 3000 +server.seed 12345",
            "Server startup complete", "^C",
            [var("Server Name", "Nama server.", "SERVER_NAME", "Garuda Rust"),
             var("Max Players", "Slot pemain.", "MAX_PLAYERS", "100")],
            """#!/bin/bash
cd /mnt/server
steamcmd +force_install_dir /mnt/server +login anonymous +app_update 258550 validate +quit
"""),
        egg("FiveM", "FiveM (GTA V) server — FXServer.",
            {"FiveM": "ghcr.io/parkervcp/yolks:fivem"},
            "bash run.sh +exec server.cfg",
            "", "^C",
            [var("License Key", "License key dari keymaster.fivem.net.", "FIVEM_LICENSE_KEY", "", True, True)],
            """#!/bin/bash
cd /mnt/server
curl -fsSL -o fx.tar.xz "https://runtime.fivem.net/artifacts/fivem/build_proot_linux/master/latest/fx.tar.xz"
tar -xf fx.tar.xz && rm fx.tar.xz
cat > run.sh <<'EOF'
#!/bin/bash
exec /mnt/server/alpine/opt/cfx-server/FXServer +set citizen_dir /mnt/server/alpine/opt/cfx-server/citizen/ "$@"
EOF
chmod +x run.sh
"""),
    ],
}


def main():
    total = 0
    for nest, eggs in EGGS.items():
        d = os.path.join(HERE, nest.replace(" ", "-").replace(".", "").lower())
        os.makedirs(d, exist_ok=True)
        for e in eggs:
            fname = e["name"].lower().replace(" ", "-").replace(".", "") + ".json"
            with open(os.path.join(d, fname), "w") as f:
                json.dump(e, f, indent=2)
                f.write("\n")
            total += 1
            print(f"{nest}/{fname}")
    print(f"{total} eggs written")


if __name__ == "__main__":
    main()
