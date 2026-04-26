# TODO LIST PRIORISÉE - Audit Sea-Dogs

## 🔴 P0 - BLOQUANT (À corriger avant production)

### [P0-1] Race Condition Pity Counter (x10 Summon)
**Fichier**: `server/internal/api/handlers.go`  
**Lignes**: 2474-2592  
**Action**: Ajouter `SELECT FOR UPDATE` ou incrément atomique SQL  
**Estimation**: 2h  
**Tests**: Test de concurrence avec 2 requêtes x10 simultanées

---

### [P0-2] Implémenter Checkpoint Island (5s throttling)
**Fichier**: `server/internal/api/handlers.go`  
**Lignes**: 418-446  
**Action**: 
- Ajouter `LastSavedCheckpoint time.Time` au modèle Island
- Sauvegarder seulement si `time.Since(island.LastSavedCheckpoint) >= 5*time.Second`
**Estimation**: 1h  
**Tests**: Vérifier que sauvegarde se fait max 1 fois/5s même avec polling 1s

---

### [P0-3] Validation Maximum Crew Counts
**Fichier**: `server/internal/api/handlers.go`  
**Lignes**: 3304-3313  
**Action**: Ajouter validation `req.Warriors <= MaxCrewPerType` (const = 10000)  
**Estimation**: 15min  
**Tests**: Tester avec valeur > 10000 doit retourner erreur

---

### [P0-4] Vérifier Atomicité Transaction ExchangeShards
**Fichier**: `server/internal/api/handlers.go`  
**Lignes**: 2861-2925  
**Action**: Ajouter test unitaire pour valider atomicité (rollback si un Save échoue)  
**Estimation**: 1h  
**Tests**: Test avec mock DB qui échoue au milieu de la transaction

---

## 🟠 P1 - IMPORTANT (À corriger prochainement)

### [P1-1] RNG Déterministe pour Tests Gacha
**Fichier**: `server/internal/economy/gacha.go`  
**Lignes**: 98-108  
**Action**: Ajouter paramètre `seed int64` à `PickCaptainTemplateByRarity`  
**Estimation**: 30min  
**Tests**: Test avec seed fixe doit retourner même résultat

---

### [P1-2] Guards Nil Systématiques Client
**Fichiers**: 
- `client/cmd/main.go:895`
- `client/cmd/construction_ui.go:106`
- Autres endroits similaires
**Action**: Ajouter guards `if g.player == nil || len(g.player.Islands) == 0` partout  
**Estimation**: 2h  
**Tests**: Tester avec `player = nil` ne doit pas causer de panic

---

### [P1-3] Daily Cap Reset avec SELECT FOR UPDATE
**Fichier**: `server/internal/api/handlers.go`  
**Lignes**: 2801-2818  
**Action**: Utiliser `tx.Set("gorm:query_option", "FOR UPDATE").First(...)`  
**Estimation**: 30min  
**Tests**: Test de concurrence avec 2 requêtes à minuit

---

### [P1-4] Clamp DR Défensif dans executeAttack
**Fichier**: `server/internal/economy/naval_combat.go`  
**Lignes**: 440-450  
**Action**: Ajouter clamp `if effectiveDR > 0.90 { effectiveDR = 0.90 }`  
**Estimation**: 15min  
**Tests**: Test avec DR = 0.95 doit être clampé à 0.90

---

### [P1-5] Implémenter LockFleetForEngagement
**Fichier**: `server/internal/economy/fleet_lock.go`  
**Lignes**: 19-26  
**Action**: Implémenter la fonction (actuellement placeholder)  
**Estimation**: 1h  
**Tests**: Test que fleet est lockée après appel, et unlock après durée

---

### [P1-6] Optimiser Preload GetStatus (Éviter N+1)
**Fichier**: `server/internal/api/handlers.go`  
**Lignes**: 348-361  
**Action**: Vérifier que preload initial charge tout, sinon optimiser  
**Estimation**: 2h  
**Tests**: Profiler avec beaucoup de flottes/navires

---

## 🟡 P2 - NICE-TO-HAVE (Améliorations futures)

### [P2-1] Throttling Logs Debug
**Fichier**: `server/internal/api/handlers.go` (GetStatus)  
**Action**: Ajouter debounce 1min par joueur pour logs debug  
**Estimation**: 1h

---

### [P2-2] Validation Format UUID Avant Parsing
**Fichiers**: Plusieurs endpoints  
**Action**: Améliorer messages d'erreur ou valider format  
**Estimation**: 2h

---

### [P2-3] Ajouter omitempty sur Champs Optionnels
**Fichier**: `server/internal/domain/models.go`  
**Action**: Ajouter `omitempty` sur PityLegendaryCount, DailyShardExchangeCount, etc.  
**Estimation**: 30min

---

### [P2-4] Standardiser Messages d'Erreur (FR/EN)
**Fichiers**: Tous les handlers  
**Action**: Standardiser sur FR ou système de traduction  
**Estimation**: 4h

---

### [P2-5] Tests Manuels Click-Through Modals
**Fichier**: `client/cmd/state_router.go`  
**Action**: Tester tous les modals (Tavern, Building, Dev, Tech, Construction, Fleet)  
**Estimation**: 1h (tests manuels)

---

## 📋 ORDRE DE PRIORITÉ RECOMMANDÉ

1. **P0-1** (Race Condition Pity) - **CRITIQUE**
2. **P0-2** (Checkpoint Island) - **Performance**
3. **P0-3** (Validation Crew Max) - **Sécurité**
4. **P1-2** (Guards Nil Client) - **Stabilité**
5. **P1-5** (Lock Fleet) - **Anti-exploit**
6. **P1-1** (RNG Déterministe) - **Tests**
7. **P1-3** (Daily Cap Race) - **Anti-exploit**
8. **P1-4** (Clamp DR) - **Sécurité**
9. **P1-6** (Preload Optim) - **Performance**
10. **P0-4** (Atomicité Test) - **Fiabilité**

P2 peuvent être faits plus tard selon les besoins.

---

**Total Estimation P0**: ~4h15  
**Total Estimation P1**: ~6h15  
**Total Estimation P2**: ~8h30  
**Total Global**: ~19h

