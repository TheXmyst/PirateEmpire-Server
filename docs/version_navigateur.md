# Version navigateur (WebAssembly)

Le même code que le jeu PC est compilé pour le navigateur (`GOOS=js GOARCH=wasm`).
Les joueurs jouent sur **https://seadogs.duckdns.org/play** (l'adresse du site
redirige vers `/play` dès que la version web est installée).

## Comment ça marche

- `client/web/build.sh` compile le jeu et prépare un dossier avec `index.html`
  (écran de chargement), `wasm_exec.js`, `game.wasm` (+ `game.wasm.gz`),
  `build-info.json` et une copie de `resources/`.
- Le serveur Go sert ce dossier sur `/play` (`server/internal/webclient`). Il le
  cherche dans `WEB_DIR`, ou par défaut dans `web/` à côté du serveur
  (en production : `/opt/seadogs/web`). Même site que l'API : pas de CORS,
  pas de changement dans Caddy.
- `deploy/update.sh` compile et installe la version web à chaque mise à jour.

## Différences avec la version PC

Le code spécifique est dans `platform_desktop.go` (PC) et `platform_js.go`
(navigateur) :

| | PC | Navigateur |
|---|---|---|
| Ressources | tout dans le .exe | images utiles dans le jeu, **musique téléchargée à part** |
| Adresse du serveur | `config.json` | le site qui a servi la page |
| « Se souvenir de moi », chat, amis | fichiers à côté du jeu | stockage du navigateur (localStorage) |
| Mot de passe mémorisé | oui | **non** (seul le pseudo est gardé) |
| Mise à jour auto | oui (Windows) | inutile : recharger la page suffit |

La musique se charge en arrière-plan sur les deux versions : le jeu démarre
tout de suite et la musique commence dès qu'elle est prête.

**Si tu ajoutes une image chargée au démarrage**, ajoute-la aussi dans la
liste `//go:embed` de `client/internal/game/platform_js.go`. Si tu oublies, ça
marche quand même (le jeu la télécharge depuis le serveur), juste un peu plus
lentement.

## Tester chez toi

```bash
./client/web/build.sh                 # crée client/web/dist/
cd server
WEB_DIR=../client/web/dist DEV_MODE=1 go run ./cmd
# puis ouvre http://localhost:8080/play/
```

## Limites connues

- Sur téléphone : se joue en paysage (un message demande de tourner le
  téléphone). Tap = clic, un doigt qui glisse déplace la carte ou fait
  défiler une fenêtre, deux doigts pour zoomer. Toucher un champ de texte
  ouvre une barre de saisie en haut avec le clavier du téléphone. Des boutons
  à droite remplacent les touches : FERMER (Échap), PLEIN ECRAN (F11, pas
  disponible sur iPhone) et DEV (F1, admins). Le code est dans
  `client/internal/game/input.go`, `touch_ui.go` et `softkeyboard_js.go`.
- Sur petit écran, l'interface (prévue pour 1280x720) est réduite pour tenir
  entièrement : les textes sont petits.
- Les navigateurs bloquent le son tant que le joueur n'a pas cliqué : la
  musique démarre au premier clic.

## Client PC (Windows) et mises à jour automatiques

`deploy/update.sh` compile aussi le jeu pour Windows depuis le VPS et le publie :

- `https://seadogs.duckdns.org/download/SeaDogs.exe` : le lien à donner aux joueurs ;
- `https://seadogs.duckdns.org/version` : la dernière version (commit du dossier `client/`), l'adresse du fichier et son empreinte SHA-256.

Le fichier est rangé dans `/opt/seadogs/download` (variable `DOWNLOAD_DIR` pour changer de dossier).
La version ne change que si un commit touche `client/` : un déploiement du serveur seul ne fait rien retélécharger.

Au lancement, le client PC compare sa version à `/version`. S'il est en retard, il propose « Installer » :
il télécharge le nouveau .exe, vérifie l'empreinte, renomme l'ancien en `.old` (effacé au lancement suivant),
met le nouveau à sa place et se relance. Un client compilé à la main (version `dev`) ne se met jamais à jour tout seul.
