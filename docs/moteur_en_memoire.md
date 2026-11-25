# Le moteur en mémoire : comment un serveur de jeu sauvegarde sans tout recharger

Ce document explique comment le serveur garde l'état du jeu, et pourquoi. Le code est dans
`server/internal/engine/world.go` (l'état en mémoire), `game_loop.go` (le tick) et
`pvp_travel.go` (les flottes en attaque).

## La question de départ

> « Comment un joueur qui fait une action est sauvegardé sans que le moteur recharge tout en boucle ? »

La réponse courte : **le moteur ne relit jamais la base pendant que le jeu tourne.** La base
de données sert de « coffre-fort » : on y range l'état de temps en temps, et on ne la relit
qu'au démarrage. Pendant le jeu, la vérité est en mémoire (des structs Go).

## Avant / après

**Avant**, à chaque tick (10 fois par seconde) :

1. on relisait **toutes** les îles, bâtiments, navires, flottes, joueurs et capitaines en base ;
2. on calculait la production et les déplacements ;
3. on réécrivait (une partie) en base.

Pendant ce temps, les requêtes HTTP lisaient et écrivaient aussi la base. Il y avait donc
deux copies de la vérité qui se marchaient dessus :

```
t0  le tick lit l'île de Bob : 1000 or
t1  un pillage retire 500 or à Bob, en base : 500 or
t2  le tick sauvegarde SA copie de t0 : 1000 or (+ production)
    => les 500 or volés réapparaissent. Duplication de ressources.
```

**Maintenant** :

| Moment | Ce qui se passe |
|---|---|
| Démarrage | `LoadWorld` lit toutes les îles **une seule fois** et les garde en mémoire. |
| Chaque tick (100 ms) | On modifie la mémoire, puis on marque l'île « sale » (`markDirty`). Aucune requête SQL. |
| Toutes les 5 s | `flushLocked` écrit **uniquement** ce qui est sale, dans **une** transaction. |
| Action critique (achat, pillage) | Écrite tout de suite en base, en transaction (voir « la passerelle »). |
| Arrêt propre (Ctrl+C, SIGTERM) | On arrête le HTTP, on arrête le tick, puis on fait une dernière sauvegarde. |

## Les 4 idées à retenir

### 1. Un seul verrou

`World.mu` est un `sync.Mutex`. Le tick, la sauvegarde et les requêtes HTTP le prennent chacun
leur tour. Tant que quelqu'un tient le verrou, personne d'autre ne lit ni ne modifie les îles.
C'est ce qui rend la duplication impossible : le tick ne peut plus travailler sur une copie
périmée pendant qu'un pillage a lieu.

Un seul verrou pour tout le monde, c'est simple et suffisant à notre échelle (SQLite n'avait de
toute façon qu'une connexion). Si un jour il y a beaucoup de joueurs, on pourra passer à un
verrou par île.

### 2. « Sale » (dirty) plutôt qu'« écrire tout de suite »

Écrire en base coûte cher, modifier une variable ne coûte rien. Donc le tick modifie la mémoire
et note seulement *quoi* a changé (`DirtyIsland`, `DirtyFleets`, `DirtyBuildings`,
`DirtyPlayer`). La sauvegarde périodique regroupe tout ça.

**Qu'est-ce qu'on risque ?** Si le serveur plante brutalement (coupure de courant, `kill -9`),
on perd au maximum les 5 dernières secondes de production et de déplacement. Un arrêt propre,
lui, ne perd rien. L'intervalle se règle avec la variable d'environnement
`WORLD_SAVE_INTERVAL_SECONDS`.

### 3. Les événements « rejouables » peuvent attendre

Une recherche ou une construction qui se termine dépend seulement d'une heure de fin. Si on
plante avant de l'avoir sauvegardée, au redémarrage l'heure est toujours dépassée et
l'événement se reproduit. Il n'y a donc aucun risque à attendre la sauvegarde périodique.

À l'inverse, un pillage ou un achat déplace des ressources : s'il était perdu, le joueur
pourrait le refaire, ou des ressources apparaîtraient en double. Ceux-là sont écrits **tout de
suite, en transaction**.

### 4. La passerelle (étape intermédiaire)

Les handlers HTTP (plus de 4000 lignes) lisent et écrivent encore la base directement. Les
réécrire tous d'un coup serait risqué, donc chaque requête passe par `World.Middleware` /
`RunWithDB` :

1. prendre le verrou (le tick est en pause) ;
2. sauvegarder ce qui est sale, pour que le handler lise des valeurs fraîches ;
3. exécuter le handler : ses écritures partent tout de suite en base ;
4. si le handler a écrit (un compteur branché sur GORM le détecte), recharger la mémoire ;
5. rendre le verrou.

Les arrivées de flottes en attaque (combat, pillage, dépôt du butin) utilisent la même
passerelle depuis le tick. Le déplacement, lui, se fait en mémoire.

`/status`, que le client appelle en boucle, lit directement la mémoire : zéro requête SQL dans
le cas normal.

**Prochaine étape possible** : migrer les handlers un par un pour qu'ils modifient la mémoire
directement (et écrivent en base seulement les actions critiques). Quand ce sera fait, la
sauvegarde avant chaque requête disparaîtra.

## Comportements qui changent (des bugs qui disparaissent)

- **Production des ressources.** Avant, le tick relisait les ressources en base à chaque tick
  mais ne les sauvegardait que toutes les 5 s, donc il perdait presque toute la production entre
  deux sauvegardes. En parallèle, `/status` ajoutait sa propre production. Le résultat
  dépendait de la fréquence d'appel de `/status`. Maintenant il n'y a qu'un seul calcul (le
  tick), qui compte exactement le temps écoulé. Les valeurs de production ne changent pas.
- **Poursuites (PvE/PvP).** La copie périmée de l'île réécrivait la position des flottes en
  poursuite à chaque tick, ce qui pouvait annuler leur déplacement. Ça n'arrive plus.
- **Rhum des flottes en attaque.** Deux bouts de code consommaient le rhum de ces flottes. En
  pratique, une des deux consommations était écrasée par la copie périmée. On garde maintenant
  une seule consommation, ce qui donne le même résultat que celui qu'on observait en jeu.
- **Pillage.** L'or volé ne réapparaît plus chez le défenseur (test
  `TestPillageIsNotUndoneByTheTick`).
