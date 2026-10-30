# Egg library

15 egg siap pakai (format PTDL_v2, sama seperti egg Pterodactyl), dikelompokkan
per nest:

| Nest | Egg |
|---|---|
| Minecraft | Paper, Bedrock, Forge |
| Node.js | Node.js, Discord.js Bot |
| Python | Python, Django, Flask |
| Web Hosting | PHP Website, Static Website, Laravel |
| Game Servers | Terraria, Valheim, Rust, FiveM |

## Regenerate

```bash
python3 generate.py   # tulis ulang eggs/*/*.json dari spec di file ini
```

## Import

Admin → Eggs → Import, atau jalankan `~/workspace/garuda-live/eggs-import.py`
(buat nest yang belum ada, lewati egg yang namanya sudah ada).

## Catatan

- `startup` boleh pakai `{{VAR}}` (variabel egg) dan `{{SERVER_PORT}}` /
  `{{SERVER_IP}}` (diisi panel dari alokasi server).
- `config.startup.done`: string yang menandai server sudah jalan. Kalau
  dikosongkan, status langsung `running` saat proses start. **Jangan isi
  done string yang tidak pasti** — kalau string itu tidak pernah muncul di
  output, server terjebak `starting` dan Stop/Restart akan di-block guard.
- Install script jalan di mode Docker. Di mode host-process (VM tanpa Docker),
  file server di-upload lewat file manager.
