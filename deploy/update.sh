#!/usr/bin/env bash
# Met à jour le serveur SeaDogs sur le VPS : récupère le code, recompile,
# redémarre. À lancer en root depuis le dossier du dépôt cloné :
#   sudo ./deploy/update.sh
set -euo pipefail

# Tout est dans une fonction : bash lit ainsi le script en entier avant de
# commencer, et "git pull" peut le modifier sans risque pendant qu'il tourne.
main() {
	REPO_DIR="$(cd "$(dirname "$0")/.." && pwd)"
	APP_DIR=/opt/seadogs

	cd "$REPO_DIR"
	git pull --ff-only

	echo "Compilation..."
	(cd server && CGO_ENABLED=1 go build -o "$REPO_DIR/seadogs-server.new" ./cmd)
	(cd server && CGO_ENABLED=1 go build -o "$REPO_DIR/admin_tool.new" ./cmd/admin_tool)

	echo "Compilation de la version navigateur (/play)..."
	rm -rf "$REPO_DIR/web.new"
	./client/web/build.sh "$REPO_DIR/web.new"

	echo "Sauvegarde de la base avant mise à jour..."
	mkdir -p "$APP_DIR/backups"
	if [ -f "$APP_DIR/seadogs.db" ]; then
		sqlite3 "$APP_DIR/seadogs.db" ".backup '$APP_DIR/backups/seadogs-$(date +%Y%m%d-%H%M%S).db'"
	fi

	echo "Installation..."
	systemctl stop seadogs || true
	install -o seadogs -g seadogs -m 0755 "$REPO_DIR/seadogs-server.new" "$APP_DIR/seadogs-server"
	install -o seadogs -g seadogs -m 0755 "$REPO_DIR/admin_tool.new" "$APP_DIR/admin_tool"
	rm -f "$REPO_DIR/seadogs-server.new" "$REPO_DIR/admin_tool.new"
	rm -rf "$APP_DIR/web"
	mv "$REPO_DIR/web.new" "$APP_DIR/web"
	rm -rf "$APP_DIR/configs"
	cp -r server/configs "$APP_DIR/configs"
	chown -R seadogs:seadogs "$APP_DIR"
	systemctl start seadogs

	echo "Terminé. Logs : journalctl -u seadogs -f"
}

main "$@"
