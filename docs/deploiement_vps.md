# Déployer le serveur SeaDogs sur un VPS, en HTTPS

Ce guide part d'un VPS tout neuf sous **Ubuntu** (par exemple un Cloud VPS
Contabo) et finit avec un serveur de jeu joignable en
`https://ton-nom.duckdns.org`, qui redémarre tout seul et sauvegarde sa base.

Le guide est prévu pour **cohabiter avec un autre projet déjà installé sur
le VPS** (un autre jeu, un site...) : SeaDogs a son propre utilisateur, son
propre dossier, son propre port interne et ses propres sauvegardes, et on
n'écrase aucune configuration existante. Lis bien l'étape 0 avant de
commencer.

Compte environ 30 à 45 minutes la première fois. Chaque commande est à copier
telle quelle, sauf les morceaux `EN_MAJUSCULES` à remplacer.

## Ce qu'on construit

```
 Joueur (client)                          VPS
 ───────────────        HTTPS (443)       ┌──────────────────────────────────┐
  https://ton-nom  ─────────────────────▶ │ Caddy  (certificat automatique)  │
  .duckdns.org          chiffré           │   │                              │
                                          │   ▼  127.0.0.1:8090 (local seul) │
                                          │ seadogs-server (service systemd) │
                                          │   │                              │
                                          │   ▼                              │
                                          │ /opt/seadogs/seadogs.db (SQLite) │
                                          └──────────────────────────────────┘
```

- **DuckDNS** donne un nom gratuit (`ton-nom.duckdns.org`) qui pointe vers
  l'IP du VPS. Il en faut un, parce que les certificats HTTPS gratuits se
  donnent pour un nom, pas pour une IP.
- **Caddy** (ou nginx, si l'autre jeu l'utilise déjà) reçoit les joueurs en
  HTTPS. Le certificat Let's Encrypt est obtenu et renouvelé tout seul.
  Un seul programme peut écouter sur les ports 80/443 : si l'autre jeu en a
  déjà un, SeaDogs s'ajoute dedans comme un site de plus.
- **Le serveur Go** n'écoute qu'en local (`127.0.0.1`). Personne ne peut lui
  parler en direct en HTTP non chiffré, tout passe par Caddy.

---

## 0. État des lieux : qu'est-ce qui tourne déjà sur le VPS ?

Connecte-toi au VPS (`ssh root@IP_DU_VPS`) et lance ces trois commandes.
Elles ne modifient rien, elles regardent seulement.

```bash
# Quels programmes écoutent sur quels ports ?
ss -tlnp

# Un reverse proxy est-il déjà installé et lancé ?
systemctl is-active caddy nginx apache2 2>/dev/null

# L'autre jeu tourne-t-il dans Docker ?
docker ps 2>/dev/null
```

Note trois choses, elles serviront plus bas :

1. **Qui écoute sur les ports 80 et 443 ?** Dans la sortie de `ss -tlnp`,
   regarde les lignes `:80` et `:443`. Tu tomberas dans l'un de ces cas :
   - **personne** → cas A de l'étape 9 (on installe Caddy) ;
   - **caddy** → cas B (on ajoute SeaDogs au Caddy existant) ;
   - **nginx** → cas C (on ajoute SeaDogs à nginx) ;
   - **docker-proxy, traefik, apache…** → arrête-toi et demande-moi, on
     adaptera ensemble.
2. **Le port 8090 est-il libre ?** Si une ligne contient `:8090`, choisis un
   autre port libre (par exemple 8091) et utilise-le partout où ce guide
   écrit 8090.
3. **Tous les ports utilisés par l'autre jeu** (par exemple un port de jeu
   en TCP/UDP). Il faudra les garder ouverts dans le pare-feu (étape 3).
   Pour l'UDP : `ss -ulnp`.

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

> ⚠️ `apt upgrade` peut redémarrer certains services, dont ceux de l'autre
> jeu. Fais-le à un moment où personne n'y joue, ou saute-le si tu préfères
> (la ligne `apt install` suffit pour SeaDogs).

`build-essential` installe le compilateur C dont a besoin le pilote SQLite.

## 3. Le pare-feu

Commence par regarder si un pare-feu est déjà actif :

```bash
ufw status
```

**S'il affiche `Status: active`**, il protège déjà l'autre jeu : on ne
touche à rien de ce qui existe, on ajoute juste le web (sans effet si c'est
déjà ouvert) :

```bash
ufw allow 80/tcp
ufw allow 443/tcp
```

**S'il affiche `Status: inactive`**, attention : l'activer bloquerait tout
ce qui n'est pas autorisé, y compris l'autre jeu. Autorise d'abord SSH, le
web, **et chaque port de l'autre jeu noté à l'étape 0**, puis active :

```bash
ufw allow OpenSSH
ufw allow 80/tcp
ufw allow 443/tcp
ufw allow PORT_DE_L_AUTRE_JEU/tcp      # une ligne par port, /udp si besoin
ufw enable
ufw status
```

Vérifie ensuite que l'autre jeu marche toujours. En cas de doute :
`ufw disable` remet tout comme avant.

Le port interne de SeaDogs (8090) n'est jamais ouvert : c'est voulu, on n'y
accède qu'en passant par le reverse proxy.

## 4. Installer Go

Si Go est déjà installé pour l'autre jeu, regarde sa version :

```bash
go version
```

S'il affiche `go1.25` ou plus, saute cette étape. Sinon, installe la version
de go.dev ci-dessous. Une version plus récente de Go compile sans problème
les projets plus anciens : l'autre jeu continuera de se compiler.

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
curl http://127.0.0.1:8090/health
```

Tu dois voir `{"status":"ok"}`. Les logs en direct : `journalctl -u seadogs -f`
(`Ctrl+C` pour sortir).

## 9. Le HTTPS (selon ce que tu as trouvé à l'étape 0)

Dans les trois cas, à la fin, ouvre `https://TON-NOM.duckdns.org/health`
dans un navigateur depuis ton PC : le cadenas doit apparaître et la page
afficher `{"status":"ok"}`. 🎉 Vérifie aussi que l'autre jeu répond toujours.

### Cas A : rien n'écoute sur 80/443 → installer Caddy

```bash
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | tee /etc/apt/sources.list.d/caddy-stable.list
apt update && apt install -y caddy
cp deploy/Caddyfile /etc/caddy/Caddyfile
nano /etc/caddy/Caddyfile        # remplace seadogs.duckdns.org par ton nom
systemctl reload caddy
```

### Cas B : Caddy sert déjà l'autre jeu → ajouter un bloc

**Ne copie pas** le fichier par-dessus, ça effacerait la config de l'autre
jeu. On fait une sauvegarde, puis on ajoute le bloc SeaDogs à la fin :

```bash
cp /etc/caddy/Caddyfile /etc/caddy/Caddyfile.avant-seadogs
grep -v '^#' deploy/Caddyfile >> /etc/caddy/Caddyfile
nano /etc/caddy/Caddyfile        # vérifie la fin du fichier, mets ton nom
caddy validate --config /etc/caddy/Caddyfile
systemctl reload caddy
```

`caddy validate` vérifie la syntaxe avant de recharger : s'il affiche une
erreur, rien n'a changé pour l'autre jeu. Pour revenir en arrière :
`cp /etc/caddy/Caddyfile.avant-seadogs /etc/caddy/Caddyfile && systemctl reload caddy`.

### Cas C : nginx sert déjà l'autre jeu → ajouter un site nginx

On n'installe pas Caddy (il se battrait avec nginx pour les ports 80/443).
On ajoute un fichier de site à part, sans toucher à ceux de l'autre jeu, et
`certbot` s'occupe du certificat :

```bash
apt install -y certbot python3-certbot-nginx
cp deploy/nginx-seadogs.conf /etc/nginx/sites-available/seadogs
nano /etc/nginx/sites-available/seadogs     # mets ton nom (et ton port)
ln -s /etc/nginx/sites-available/seadogs /etc/nginx/sites-enabled/seadogs
nginx -t
systemctl reload nginx
certbot --nginx -d TON-NOM.duckdns.org
```

`nginx -t` vérifie la config avant de recharger. `certbot` te pose deux ou
trois questions (e-mail pour les alertes d'expiration, acceptation des
conditions), puis ajoute tout seul la partie HTTPS et le renouvellement
automatique. Pour retirer SeaDogs : `rm /etc/nginx/sites-enabled/seadogs && systemctl reload nginx`.

### Si ça ne marche pas

La plupart du temps, c'est le nom DuckDNS qui ne pointe pas encore vers la
bonne IP, ou le port 80/443 fermé dans le pare-feu. Les logs :
`journalctl -u caddy -n 50` ou `journalctl -u nginx -n 50`.

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
jour. Il ne touche qu'au service `seadogs` et au dossier `/opt/seadogs` :
l'autre jeu n'est jamais arrêté.

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
| `LISTEN_ADDR` | Adresse d'écoute du serveur Go | `127.0.0.1:8090` (un port libre) |
| `DEV_ROUTES_ENABLED` | Active les routes de triche `/dev/*` (admins seulement) | `false` |
| `DEV_MODE` | Autorise le démarrage sans `AUTH_SECRET` (clé temporaire) | ne pas mettre |
| `WORLD_SAVE_INTERVAL_SECONDS` | Fréquence de sauvegarde du monde | `5` |

En local sur ton PC, lance simplement le serveur avec `DEV_MODE=1`
(PowerShell : `$env:DEV_MODE="1"; go run ./cmd`).
