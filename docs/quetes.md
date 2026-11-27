# Les quêtes de progression

Le journal de quêtes guide le joueur du premier bâtiment jusqu'à ses premières
victoires en mer. Il y a **135 quêtes** en trois familles :

| Famille | Combien | Comment elles s'ouvrent |
|---|---|---|
| Histoire | 15 | une à la fois, dans l'ordre du fichier |
| Bâtiments | 96 | une par niveau (1 à 8) de chaque bâtiment ; on voit les niveaux atteints et le suivant |
| Défis | 24 | selon le niveau d'Hôtel de Ville (`min_townhall`) et/ou une autre quête (`after`) |

## Où ça se règle

Tout est dans `server/configs/quests.json` (relancer le serveur après une modification) :

- `building_quests` : la règle des quêtes Bâtiments. Chaque niveau rapporte
  `early_refund` (50 %) de son coût jusqu'au niveau `early_max_level` (5, ou
  `townhall_early_max_level` = 6 pour l'Hôtel de Ville), puis `late_refund` (6 %).
  Si tu changes les coûts dans `buildings.json`, les récompenses suivent toutes seules.
- `quests` : les quêtes Histoire et Défis, avec leur texte, leur fiche « Guide »
  (`tutorial`), leur objectif et leur récompense (`wood`, `stone`, `gold`, `rum`, `tickets`).

Types d'objectifs possibles : `building_level`, `buildings_level` (liste ou `["all"]`),
`buildings_built`, `tech_count`, `tech`, `ships_owned` (avec `ship_type` en option),
`fleet_size`, `ship_crewed`, `militia_all_types`, `captains_owned`, `captain_rarity`,
`captain_stars`, `pve_wins` (avec `tier` en option), `counter`, `guild`.
Les compteurs (`counter`) : `captain_pulls`, `militia_recruited`, `fleets_sent`,
`fleets_stationed`, `chat_messages`, `harvested`.

**Attention** : ne change pas l'`id` d'une quête déjà en ligne, sinon les joueurs
pourraient la réclamer une deuxième fois.

## Comment ça marche

- L'avancement est recalculé à partir de l'état réel du joueur (bâtiments,
  recherches, navires, capitaines, victoires PvE…) : une quête ne peut pas être « ratée ».
  Seuls les événements ci-dessus sont comptés à part (colonnes `quest_*` de `players`).
- `GET /quests` donne les quêtes proposées ; `POST /quests/claim` avec `{"id": "..."}`
  ou `{"all": true}` donne la récompense. Une quête réclamée est notée dans
  `player_quest_claims` (une seule fois par joueur).
- Si la récompense ne tient pas dans l'entrepôt, le serveur refuse au lieu de la perdre.
- Les joueurs déjà avancés peuvent réclamer les quêtes qu'ils ont déjà remplies.
