# Déployer le serveur SeaDogs sur un VPS, en HTTPS

Ce guide part d'un VPS tout neuf sous **Ubuntu** (par exemple un Cloud VPS
Contabo) et finit avec un serveur de jeu joignable en
`https://ton-nom.duckdns.org`, qui redémarre tout seul et sauvegarde sa base.

Compte environ 30 à 45 minutes la première fois. Chaque commande est à copier
telle quelle, sauf les morceaux `EN_MAJUSCULES` à remplacer.

## Ce qu'on construit

```
 Joueur (client)                          VPS
 ───────────────        HTTPS (443)       ┌──────────────────────────────────┐
  https://ton-nom  ─────────────────────▶ │ Caddy  (certificat automatique)  │
  .duckdns.org          chiffré           │   │                              │
                                          │   ▼  127.0.0.1:8080 (local seul) │
                                          │ seadogs-server (service systemd) │
                                          │   │                              │
                                          │   ▼                              │
                                          │ /opt/seadogs/seadogs.db (SQLite) │
                                          └──────────────────────────────────┘
```

- **DuckDNS** donne un nom gratuit (`ton-nom.duckdns.org`) qui pointe vers
  l'IP du VPS. Il en faut un, parce que les certificats HTTPS gratuits se
  donnent pour un nom, pas pour une IP.
- **Caddy** reçoit les joueurs en HTTPS. Il obtient le certificat
  Let's Encrypt et le renouvelle tout seul, tu n'as rien à faire.
- **Le serveur Go** n'écoute qu'en local (`127.0.0.1`). Personne ne peut lui
  parler en direct en HTTP non chiffré, tout passe par Caddy.

---

## 1. Créer l'adresse DuckDNS

1. Va sur <https://www.duckdns.org> et connecte-toi (avec GitHub ou Google).
2. Dans « sub domain », tape le nom voulu (par exemple `seadogs`) puis
   **add domain**.
3. Dans la case « current ip » de ce domaine, mets **l'IP de ton VPS**
   (visible dans le panneau Contabo) puis **update ip**.

Vérifie depuis ton PC (PowerShell ou terminal) :

```
ping seadogs.duckdns.org
```

L'IP affichée doit être celle du VPS. Ça peut prendre une minute.

## 2. Se connecter au VPS et le mettre à jour

Depuis ton PC (PowerShell sous Windows a `ssh` intégré) :

```
ssh root@IP_DU_VPS
```

Puis sur le VPS :

```bash
apt update && apt upgrade -y
apt install -y git build-essential sqlite3 ufw curl debian-keyring debian-archive-keyring apt-transport-https
```

`build-essential` installe le compilateur C dont a besoin le pilote SQLite.

## 3. Le pare-feu

On n'ouvre que trois portes : SSH (pour toi), 80 et 443 (pour Caddy).
**Autorise SSH avant d'activer le pare-feu**, sinon tu te fermes la porte au
nez.

```bash
ufw allow OpenSSH
ufw allow 80/tcp
ufw allow 443/tcp
ufw enable
ufw status
```

Le port 8080 reste fermé : c'est voulu.

## 4. Installer Go

La version de Go fournie par Ubuntu est trop vieille, on prend celle de
go.dev (la même que dans `go.mod`) :

```bash
cd /tmp
curl -LO https://go.dev/dl/go1.25.5.linux-amd64.tar.gz
rm -rf /usr/local/go && tar -C /usr/local -xzf go1.25.5.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> /etc/profile.d/go.sh
source /etc/profile.d/go.sh
go version
```

## 5. Un utilisateur dédié pour le serveur

Le serveur ne tourne pas en `root` : s'il avait une faille, l'attaquant
n'aurait accès qu'à son dossier, pas à toute la machine.

```bash
useradd --system --home /opt/seadogs --shell /usr/sbin/nologin seadogs
mkdir -p /opt/seadogs /etc/seadogs
chown seadogs:seadogs /opt/seadogs
```

## 6. Récupérer le code

```bash
cd /root
git clone https://github.com/TheXmyst/PirateEmpire-Server.git
cd PirateEmpire-Server
```

Si le dépôt est privé, GitHub demande un identifiant : mets ton pseudo GitHub
et, comme mot de passe, un **jeton d'accès personnel** (GitHub → Settings →
Developer settings → Personal access tokens → Fine-grained, accès en lecture
seule à ce dépôt).

## 7. La configuration secrète

```bash
cp deploy/seadogs.env.example /etc/seadogs/seadogs.env
openssl rand -hex 32
```

La deuxième commande affiche une longue clé aléatoire. Ouvre le fichier :

```bash
nano /etc/seadogs/seadogs.env
```

et colle la clé après `AUTH_SECRET=`. Enregistre (`Ctrl+O`, `Entrée`) et
quitte (`Ctrl+X`). Puis protège le fichier :

```bash
chown root:seadogs /etc/seadogs/seadogs.env
chmod 640 /etc/seadogs/seadogs.env
```

> 💡 Sans `AUTH_SECRET`, le serveur refuse de démarrer. C'est exprès : cette
> clé empêche quiconque de fabriquer de faux jetons de connexion.

## 8. Installer le service et lancer le serveur

```bash
cp deploy/seadogs.service /etc/systemd/system/seadogs.service
systemctl daemon-reload
systemctl enable seadogs
./deploy/update.sh
```

`update.sh` compile le serveur, le copie dans `/opt/seadogs` avec ses fichiers
`configs/`, et démarre le service. Vérifie :

```bash
systemctl status seadogs
curl http://127.0.0.1:8080/health
```

Tu dois voir `{"status":"ok"}`. Les logs en direct : `journalctl -u seadogs -f`
(`Ctrl+C` pour sortir).

## 9. Installer Caddy (le HTTPS)

```bash
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | tee /etc/apt/sources.list.d/caddy-stable.list
apt update && apt install -y caddy
```

Mets ta config :

```bash
cp deploy/Caddyfile /etc/caddy/Caddyfile
nano /etc/caddy/Caddyfile        # remplace seadogs.duckdns.org par ton nom
systemctl reload caddy
```

Depuis ton PC, ouvre `https://TON-NOM.duckdns.org/health` dans un navigateur :
le cadenas doit apparaître et la page afficher `{"status":"ok"}`. 🎉

Si ça ne marche pas, regarde `journalctl -u caddy -n 50` : la plupart du
temps, c'est le nom DuckDNS qui ne pointe pas encore vers la bonne IP, ou le
port 80/443 fermé.

## 10. Pointer le client vers le serveur

Dans le `config.json` à côté du client :

```json
{
    "api_url": "https://TON-NOM.duckdns.org"
}
```

Le client affiche un avertissement `[SECURITE]` si l'adresse est en `http://`
(sauf `localhost`), et l'auto-updater refuse de télécharger une mise à jour
en HTTP.

## 11. Te donner les droits admin

Les droits admin ne dépendent plus du pseudo : un compte neuf n'est jamais
admin. Inscris-toi normalement depuis le client, puis sur le VPS :

```bash
systemctl stop seadogs
cd /opt/seadogs
sudo -u seadogs ./admin_tool grant -db seadogs.db -user TON_PSEUDO
systemctl start seadogs
```

On arrête le serveur avant, parce qu'il garde les joueurs en mémoire et
pourrait réécrire l'ancienne valeur par-dessus à sa prochaine sauvegarde.
Reconnecte-toi ensuite dans le client pour que ce soit pris en compte.

---

## Au quotidien

**Mettre à jour le serveur** après un `git push` :

```bash
cd /root/PirateEmpire-Server
./deploy/update.sh
```

Le script sauvegarde la base dans `/opt/seadogs/backups/` avant chaque mise à
jour.

**Sauvegarder la base à la main** :

```bash
sqlite3 /opt/seadogs/seadogs.db ".backup '/opt/seadogs/backups/manuel.db'"
```

Pense à copier de temps en temps ces sauvegardes sur ton PC
(`scp root@IP_DU_VPS:/opt/seadogs/backups/*.db .`) : une sauvegarde qui vit
seulement sur le VPS disparaît avec lui.

**Voir les logs** : `journalctl -u seadogs -f`

**Redémarrer** : `systemctl restart seadogs` (le monde en mémoire est
sauvegardé proprement avant l'arrêt).

## Variables de configuration

| Variable | Rôle | Production |
|---|---|---|
| `AUTH_SECRET` | Clé qui signe les jetons de connexion (32 caractères min.) | **obligatoire** |
| `LISTEN_ADDR` | Adresse d'écoute du serveur Go | `127.0.0.1:8080` |
| `DEV_ROUTES_ENABLED` | Active les routes de triche `/dev/*` (admins seulement) | `false` |
| `DEV_MODE` | Autorise le démarrage sans `AUTH_SECRET` (clé temporaire) | ne pas mettre |
| `WORLD_SAVE_INTERVAL_SECONDS` | Fréquence de sauvegarde du monde | `5` |

En local sur ton PC, lance simplement le serveur avec `DEV_MODE=1`
(PowerShell : `$env:DEV_MODE="1"; go run ./cmd`).
