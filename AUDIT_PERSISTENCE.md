# Audit du Modèle de Persistance - Progression et Écritures DB

## Résumé du Modèle Actuel

Le serveur utilise un modèle **"timestamp-based"** pour les timers de progression :
- **Buildings** : `Constructing bool` + `FinishTime time.Time` (timestamp absolu)
- **Research** : `ResearchingTechID string` + `ResearchFinishTime time.Time` (timestamp absolu)
- **Ships** : `State string` + `FinishTime time.Time` (timestamp absolu)

Les ressources sont calculées **on-demand** à partir de `Island.LastUpdated` et des taux de production, puis **persistées** après calcul.

**Fréquence d'écriture DB** : 
- **Game Loop** : Toutes les **1 seconde** (ticker) → écriture `Island` pour chaque île
- **Handlers API** : À chaque action (build, upgrade, research, ship construction) → écriture transactionnelle

---

## 1. BUILDINGS (Construction/Upgrade)

### Modèle de Timer

**Fichier** : `server/internal/domain/models.go` (lignes 120-129)

```go
type Building struct {
    ID           uuid.UUID `json:"id"`
    IslandID     uuid.UUID `json:"island_id"`
    Type         string    `json:"type"`
    Level        int       `json:"level"`
    X            float64   `json:"x"`
    Y            float64   `json:"y"`
    Constructing bool      `json:"constructing"`  // Flag: en construction ou non
    FinishTime   time.Time `json:"finish_time"`   // Timestamp absolu de fin
}
```

**Type de modèle** : **`started_at + ends_at` (timestamp absolu)**
- Pas de `started_at` explicite, mais `FinishTime` est calculé comme `time.Now().Add(duration)`
- Le timer est vérifié par comparaison : `time.Now().After(b.FinishTime)`

### Vérification de Completion

**Fichier** : `server/internal/engine/game_loop.go` (lignes 193-200)

```go
if b.Constructing {
    if time.Now().After(b.FinishTime) {
        b.Constructing = false
        b.Level++
        db.Save(b)  // Écriture uniquement à la completion
    } else {
        continue  // Skip production si en construction
    }
}
```

**Fréquence de vérification** : Toutes les 1 seconde (game loop tick)

### Démarrage de Construction

**Fichier** : `server/internal/api/handlers.go`

**Build (nouveau bâtiment)** : Lignes 614-623
```go
building := domain.Building{
    Constructing: true,
    FinishTime:   time.Now().Add(time.Duration(buildDuration) * time.Second),
}
tx.Create(&building)  // Écriture transactionnelle
```

**Upgrade (bâtiment existant)** : Lignes 752-753
```go
building.Constructing = true
building.FinishTime = time.Now().Add(time.Duration(buildDuration) * time.Second)
tx.Save(&building)  // Écriture transactionnelle
```

---

## 2. TECHNOLOGY RESEARCH

### Modèle de Timer

**Fichier** : `server/internal/domain/models.go` (lignes 44-47)

```go
type Player struct {
    ResearchingTechID         string    `json:"researching_tech_id"`
    ResearchFinishTime        time.Time `json:"research_finish_time"`  // Timestamp absolu
    ResearchTotalDurationSeconds float64 `json:"current_research_total_duration_seconds"`  // Durée totale (pour UI)
}
```

**Type de modèle** : **`started_at + ends_at` (timestamp absolu)**
- Pas de `started_at` explicite, mais `ResearchFinishTime` est calculé comme `time.Now().Add(duration)`
- Le timer est vérifié par comparaison : `time.Now().After(player.ResearchFinishTime)`

### Vérification de Completion

**Fichier** : `server/internal/engine/game_loop.go` (lignes 83-128)

```go
if island.Player.ResearchingTechID != "" && !island.Player.ResearchFinishTime.IsZero() {
    if now.After(island.Player.ResearchFinishTime) {
        // Refetch Player from DB (fresh state)
        var freshPlayer domain.Player
        db.First(&freshPlayer, "id = ?", island.Player.ID)
        
        // Unlock tech, clear research fields
        freshPlayer.ResearchingTechID = ""
        freshPlayer.ResearchFinishTime = time.Time{}
        db.Save(&freshPlayer)  // Écriture uniquement à la completion
    }
}
```

**Fichier** : `server/internal/api/handlers.go` (lignes 374-404) - **LAZY UPDATE on Read**

```go
// Dans GetStatus() - vérification lazy lors de la lecture
if player.ResearchingTechID != "" && !player.ResearchFinishTime.IsZero() {
    if time.Now().After(player.ResearchFinishTime) {
        // Unlock tech, clear research fields
        db.Save(&player)  // Écriture uniquement à la completion
    }
}
```

**Fréquence de vérification** : 
- Game Loop : Toutes les 1 seconde
- GetStatus : À chaque appel API (lazy update)

### Démarrage de Research

**Fichier** : `server/internal/api/handlers.go` (lignes 1082-1094)

```go
finishTime := time.Now().Add(time.Duration(finalDuration) * time.Second)
player.ResearchingTechID = req.TechID
player.ResearchFinishTime = finishTime
player.ResearchTotalDurationSeconds = finalDuration

tx.Save(player)  // Écriture transactionnelle
```

---

## 3. SHIP CONSTRUCTION

### Modèle de Timer

**Fichier** : `server/internal/domain/models.go` (lignes 203-225)

```go
type Ship struct {
    ID         uuid.UUID `json:"id"`
    PlayerID   uuid.UUID `json:"player_id"`
    IslandID   uuid.UUID `json:"island_id"`
    FleetID    *uuid.UUID `json:"fleet_id"`
    Name       string    `json:"name"`
    Type       string    `json:"type"`
    State      string    `json:"state"`        // "UnderConstruction" ou "Ready"
    FinishTime time.Time `json:"finish_time"`  // Timestamp absolu de fin
}
```

**Type de modèle** : **`started_at + ends_at` (timestamp absolu)**
- Pas de `started_at` explicite, mais `FinishTime` est calculé comme `time.Now().Add(duration)`
- Le timer est vérifié par comparaison : `time.Now().After(s.FinishTime)`

### Vérification de Completion

**Fichier** : `server/internal/engine/game_loop.go` (lignes 132-143)

```go
for j := range island.Ships {
    s := &island.Ships[j]
    if s.State == "UnderConstruction" && !s.FinishTime.IsZero() {
        if now.After(s.FinishTime) {
            s.State = "Ready"
            s.FinishTime = time.Time{}  // Clear
            db.Save(s)  // Écriture uniquement à la completion
        }
    }
}
```

**Fréquence de vérification** : Toutes les 1 seconde (game loop tick)

### Démarrage de Construction

**Fichier** : `server/internal/api/handlers.go` (lignes 1268-1281)

```go
ship := domain.Ship{
    State:      "UnderConstruction",
    FinishTime: finishTime,  // time.Now().Add(duration)
}
tx.Create(&ship)  // Écriture transactionnelle
```

---

## 4. ISLAND RESOURCES

### Modèle de Calcul

**Fichier** : `server/internal/domain/models.go` (lignes 96-107)

```go
type Island struct {
    ResourcesJSON []byte                   `json:"-" gorm:"column:resources"`  // JSON persisté
    Resources     map[ResourceType]float64 `json:"resources" gorm:"-"`         // Map calculée
    LastUpdated   time.Time                 `json:"last_updated"`              // Timestamp de dernier calcul
}
```

**Type de modèle** : **Calcul on-demand basé sur `LastUpdated` + taux de production**

### Calcul des Ressources

**Fichier** : `server/internal/engine/game_loop.go` (lignes 159-282)

**Fonction** : `CalculateResources(island *domain.Island, delta time.Duration)`

**Logique** :
1. Calcule `delta = time.Now() - island.LastUpdated`
2. Pour chaque bâtiment (non en construction) :
   - Lit `stats.Production` (taux par heure)
   - Applique bonus tech
   - Calcule : `amount = (finalProd / 3600.0) * delta.Seconds()`
   - Ajoute à `island.Resources[resType]`
3. Applique limites de stockage
4. **Met à jour** `island.LastUpdated = now`

**Fichier** : `server/internal/api/handlers.go` (lignes 415-433) - **LAZY UPDATE on Read**

```go
// Dans GetStatus() - calcul lazy lors de la lecture
elapsed := now.Sub(island.LastUpdated)
if elapsed > 0 {
    engine.CalculateResources(island, elapsed)
    island.LastUpdated = now
    db.Omit("Player").Save(island)  // Écriture après calcul
}
```

**Type de modèle** : **Dérivé on-demand, puis persisté**
- Les ressources ne sont **PAS** incrémentées en continu dans une boucle
- Elles sont **calculées** à partir de `LastUpdated` et des taux de production
- Puis **persistées** après calcul

---

## 5. DB WRITE LOCATIONS & FREQUENCY

### Game Loop (Tick toutes les 1 seconde)

**Fichier** : `server/internal/engine/game_loop.go`

**Fonction** : `Tick()` (ligne 46)

**Fréquence** : **1 fois par seconde** (ticker)

**Écritures** :
1. **Research completion** (ligne 119) : `db.Save(&freshPlayer)` - **Seulement si research terminée**
2. **Ship completion** (ligne 138) : `db.Save(s)` - **Seulement si ship terminée**
3. **Building completion** (ligne 199) : `db.Save(b)` - **Seulement si building terminé**
4. **Island resources + LastUpdated** (ligne 153) : `db.Omit("Player").Save(island)` - **TOUJOURS** (chaque tick)

**Impact** : 
- **Island.Save()** est appelé **1 fois/seconde/île** → écriture JSON `resources`, `crew`, `last_updated`
- Si 10 îles → **10 écritures/seconde** de `Island` (potentiellement lourd avec JSON)

### API Handlers (Event-based)

**Fichier** : `server/internal/api/handlers.go`

#### GetStatus() - Lazy Update on Read

**Fonction** : `GetStatus()` (ligne 334)

**Fréquence** : À chaque appel API `/status` (client polling)

**Écritures** :
1. **Research completion** (ligne 401) : `db.Save(&player)` - **Seulement si research terminée**
2. **Island resources** (ligne 433) : `db.Omit("Player").Save(island)` - **Seulement si `elapsed > 0`**

#### Build (nouveau bâtiment)

**Fonction** : `Build()` (ligne 545)

**Écritures** :
1. `tx.Create(&building)` (ligne 625) - Transaction
2. `tx.Save(&island)` (ligne 633) - Transaction (ressources déduites)

**Fréquence** : À chaque construction de bâtiment

#### Upgrade (bâtiment existant)

**Fonction** : `Upgrade()` (ligne 680)

**Écritures** :
1. `tx.Save(&island)` (ligne 757) - Transaction (ressources déduites)
2. `tx.Save(&building)` (ligne 761) - Transaction (Constructing=true, FinishTime)

**Fréquence** : À chaque upgrade de bâtiment

#### StartResearch

**Fonction** : `StartResearch()` (ligne 1002)

**Écritures** :
1. `tx.Save(&island)` (ligne 1090) - Transaction (ressources déduites)
2. `tx.Save(player)` (ligne 1094) - Transaction (ResearchingTechID, ResearchFinishTime)

**Fréquence** : À chaque démarrage de recherche

#### StartShipConstruction

**Fonction** : `StartShipConstruction()` (ligne 1115)

**Écritures** :
1. `tx.Create(&ship)` (ligne 1283) - Transaction
2. `tx.Save(&island)` (ligne 1289) - Transaction (ressources déduites)

**Fréquence** : À chaque construction de navire

---

## 6. SAFETY / CORRECTNESS IMPLICATIONS

### Réduction des Écritures DB : Impact sur la Correctness

#### ✅ **TIMERS (Buildings, Research, Ships) : SÉCURISÉS**

**Modèle actuel** : Timestamp absolu (`FinishTime`)

**Impact si checkpoint 2-5 secondes** : **AUCUN**
- Les timers utilisent `time.Now().After(FinishTime)` → comparaison avec l'heure système
- **Pas de dépendance** à la fréquence d'écriture DB
- La vérification se fait à chaque tick (1s) ou lazy on read
- **Même si l'île n'est pas sauvegardée pendant 5 secondes**, le timer continue de progresser car basé sur l'heure système

**Worst-case crash** : 
- Perte de progression timer : **0 seconde** (les timers sont basés sur l'heure système, pas sur un compteur)
- Si crash 2 secondes avant completion → au redémarrage, `time.Now().After(FinishTime)` sera toujours vrai → completion immédiate

#### ⚠️ **RESSOURCES : IMPACT MODÉRÉ**

**Modèle actuel** : Calcul on-demand basé sur `LastUpdated`

**Impact si checkpoint 2-5 secondes** : **PERTE DE RESSOURCES PASSIVES**

**Scénario actuel** :
- Game Loop écrit `Island` toutes les 1 seconde avec `LastUpdated = now`
- Si crash → ressources calculées jusqu'au dernier `LastUpdated` sauvegardé

**Scénario avec checkpoint 5 secondes** :
- Game Loop écrit `Island` toutes les 5 secondes avec `LastUpdated = now`
- Si crash 4 secondes après le dernier checkpoint → **perte de 4 secondes de production**

**Worst-case crash** :
- Perte maximale : **N secondes** (où N = intervalle de checkpoint)
- Exemple : Checkpoint 5s → perte max **5 secondes** de production passive
- Calcul : Si production = 1000/h → perte max = `(1000/3600) * 5 = 1.39` ressources

**Mitigation possible** :
- Utiliser `LastUpdated` comme "checkpoint" : au redémarrage, calculer depuis le dernier `LastUpdated` sauvegardé
- **Pas de perte** si on recalcule depuis `LastUpdated` (déjà implémenté dans `GetStatus()`)

#### ✅ **ÉVÉNEMENTS (Build, Upgrade, Research Start, Ship Start) : SÉCURISÉS**

**Modèle actuel** : Écriture transactionnelle immédiate

**Impact si checkpoint 2-5 secondes** : **AUCUN**
- Les événements utilisent des transactions (`tx.Commit()`) → écriture immédiate
- **Pas de changement nécessaire** : les événements doivent rester transactionnels

---

## 7. RECOMMANDATIONS

### Stratégie d'Optimisation Proposée

1. **Game Loop** : Réduire la fréquence d'écriture `Island` de 1s → 5s
   - **Sécurité** : ✅ Timers inchangés (basés sur timestamp)
   - **Ressources** : ⚠️ Perte max 5s de production en cas de crash (acceptable)
   - **Gain** : Réduction de 80% des écritures `Island` (10 îles → 2 écritures/seconde au lieu de 10)

2. **Handlers API** : **GARDER** les écritures transactionnelles immédiates
   - Build, Upgrade, Research Start, Ship Start → **Pas de changement**
   - GetStatus lazy update → **Garder** (déjà optimisé)

3. **Checkpoint Strategy** :
   - Game Loop : Écrire `Island` toutes les 5 secondes (au lieu de 1s)
   - Handlers : Écritures transactionnelles immédiates (inchangé)
   - **Recovery** : Au redémarrage, `GetStatus()` recalcule depuis `LastUpdated` → **Pas de perte permanente**

### Fichiers à Modifier (si optimisation)

- `server/internal/engine/game_loop.go` (ligne 153) : Ajouter un compteur/ticker pour écrire toutes les 5 secondes
- **Aucun autre changement nécessaire** : le modèle timestamp est déjà résistant aux écritures différées

---

## Conclusion

Le modèle actuel est **robuste** pour les timers (timestamp-based) mais **coûteux** pour les ressources (écriture 1s/île).

**Réduction des écritures à 5 secondes** :
- ✅ **Aucun impact** sur la correctness des timers
- ⚠️ **Perte max 5s** de production passive en cas de crash (acceptable, récupérable au prochain `GetStatus()`)
- ✅ **Gain significatif** : Réduction de 80% des écritures `Island` dans le game loop

