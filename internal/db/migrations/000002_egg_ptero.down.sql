-- Rollback for 000002_egg_ptero: drop the added columns.
-- Data migrated into docker_images/config_stop is lost on downgrade;
-- original docker_image/stop_signal columns were left untouched.

ALTER TABLE eggs DROP COLUMN install_entrypoint;
ALTER TABLE eggs DROP COLUMN install_container;
ALTER TABLE eggs DROP COLUMN config_logs;
ALTER TABLE eggs DROP COLUMN config_files;
ALTER TABLE eggs DROP COLUMN config_stop;
ALTER TABLE eggs DROP COLUMN config_startup_done;
ALTER TABLE eggs DROP COLUMN docker_images;
ALTER TABLE eggs DROP COLUMN description;
ALTER TABLE eggs DROP COLUMN author;
