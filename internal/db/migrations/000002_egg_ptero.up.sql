-- GarudaPanel: Pterodactyl-style egg format (user request 2026-10-23).
-- Adds author/description, multi-image docker_images JSON, config block
-- (startup done text, stop command, files, logs) and install container/entrypoint.
-- Old columns (docker_image, stop_signal) are kept for rollback safety but
-- no longer read by the application.

ALTER TABLE eggs ADD COLUMN author TEXT NOT NULL DEFAULT '';
ALTER TABLE eggs ADD COLUMN description TEXT NOT NULL DEFAULT '';
ALTER TABLE eggs ADD COLUMN docker_images TEXT NOT NULL DEFAULT '{}';
ALTER TABLE eggs ADD COLUMN config_startup_done TEXT NOT NULL DEFAULT '';
ALTER TABLE eggs ADD COLUMN config_stop TEXT NOT NULL DEFAULT '^C';
ALTER TABLE eggs ADD COLUMN config_files TEXT NOT NULL DEFAULT '{}';
ALTER TABLE eggs ADD COLUMN config_logs TEXT NOT NULL DEFAULT '{}';
ALTER TABLE eggs ADD COLUMN install_container TEXT NOT NULL DEFAULT '';
ALTER TABLE eggs ADD COLUMN install_entrypoint TEXT NOT NULL DEFAULT 'ash';

-- Migrate existing single docker_image into docker_images JSON.
UPDATE eggs
SET docker_images = '{"Default": "' || replace(docker_image, '"', '\"') || '"}'
WHERE docker_image <> '' AND docker_images = '{}';

-- Map old stop_signal to config_stop where it carries meaning:
-- Pterodactyl uses "^C" for SIGINT; keep SIGTERM as-is otherwise.
UPDATE eggs SET config_stop = '^C' WHERE stop_signal = 'SIGINT';
