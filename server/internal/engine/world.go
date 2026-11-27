package engine

// =============================================================================
//  LE MONDE EN MÉMOIRE  (world.go)
// =============================================================================
//
// Avant : à chaque tick (10 fois par seconde), le moteur relisait TOUTE la base
// SQLite (îles, bâtiments, navires, flottes, joueurs, capitaines), calculait,
// puis réécrivait. Les handlers HTTP lisaient/écrivaient la même base en même
// temps. Résultat : deux copies de la vérité qui se marchent dessus. Exemple :
//
//   t0  le tick charge l'île de Bob (1000 or)
//   t1  un pillage retire 500 or à Bob, directement en base (500 or)
//   t2  le tick sauvegarde SA copie de t0 (1000 or + production)
//       => les 500 or volés réapparaissent chez Bob. Duplication !
//
// Maintenant : il n'y a QU'UNE vérité "vivante", c'est la mémoire (ce fichier).
//
//   1. Au démarrage, on charge toutes les îles une seule fois (LoadWorld).
//   2. Le tick modifie la mémoire, jamais la base directement. Chaque
//      modification "salit" (dirty) l'île concernée.
//   3. Une sauvegarde périodique (toutes les 5 s par défaut) écrit uniquement
//      ce qui est sale, dans UNE transaction (flushLocked).
//   4. À l'arrêt propre du serveur (Ctrl+C / SIGTERM), on fait une dernière
//      sauvegarde (Close).
//   5. Un seul verrou (mu) protège tout : le tick, la sauvegarde et les
//      requêtes HTTP passent chacun leur tour. Plus personne ne peut lire une
//      valeur pendant qu'un autre est en train de la modifier.
//
// Et les handlers HTTP ? Ils ont encore leur code "à l'ancienne" qui lit et
// écrit la base directement (4000+ lignes, on ne réécrit pas tout d'un coup).
// Pour qu'ils restent justes, chaque requête passe par la passerelle
// RunWithDB (voir plus bas) :
//
//   a) on prend le verrou       -> le tick est en pause pendant la requête
//   b) on sauvegarde le sale    -> le handler lit des valeurs fraîches
//   c) le handler s'exécute     -> ses écritures (achat, pillage...) partent
//                                  tout de suite en base, en transaction
//   d) s'il a écrit, on recharge la mémoire depuis la base
//   e) on rend le verrou
//
// C'est une étape intermédiaire honnête : on pourra ensuite migrer les
// handlers un par un pour qu'ils travaillent directement sur la mémoire.
// =============================================================================

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
	"github.com/TheXmyst/Sea-Dogs/server/internal/logger"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// dirtyFlag dit QUELLE partie d'une île doit être réécrite en base.
// On utilise des bits pour pouvoir les combiner : DirtyIsland|DirtyFleets.
type dirtyFlag uint8

const (
	DirtyIsland    dirtyFlag = 1 << iota // ligne "islands" (ressources, last_updated)
	DirtyFleets                          // flottes + leurs navires + navires hors flotte
	DirtyBuildings                       // bâtiments
	DirtyPlayer                          // ligne "players" (recherche terminée...)
	DirtyCaptains                        // capitaines du joueur (XP gagnée en mer)
)

// DefaultSaveInterval : fréquence de la sauvegarde périodique.
// Plus c'est long, moins on écrit, mais plus on perd en cas de crash brutal
// (le courant qui saute). Un arrêt propre, lui, ne perd rien.
const DefaultSaveInterval = 5 * time.Second

// World contient tout l'état "vivant" du jeu.
type World struct {
	mu sync.Mutex // LE verrou. Toute lecture/écriture de islands passe par lui.

	db *gorm.DB

	// islands est la copie mémoire de toutes les îles, avec leurs bâtiments,
	// navires, flottes (+navires) et leur joueur (+capitaines).
	// On garde un slice de valeurs (et pas de pointeurs) : &islands[i] reste
	// stable tant qu'on ne remplace pas le slice entier (rechargement complet).
	islands  []domain.Island
	byID     map[uuid.UUID]int // id d'île   -> index dans islands
	byPlayer map[uuid.UUID]int // id joueur  -> index de sa (première) île

	dirty map[uuid.UUID]dirtyFlag // îles modifiées depuis la dernière sauvegarde

	// Moral et XP en mer : petites quantités par tick, accumulées ici jusqu'à
	// faire un point entier (le moral est un entier en base). Clé = id de flotte.
	moraleFrac map[uuid.UUID]float64
	seaSeconds map[uuid.UUID]float64
	prevState  map[uuid.UUID]domain.FleetState

	// writes compte les écritures SQL (via des callbacks GORM). Ça permet de
	// savoir si un handler a écrit en base, et donc s'il faut recharger.
	writes atomic.Int64
}

// LoadWorld charge l'état complet du jeu en mémoire. On ne l'appelle qu'une
// fois au démarrage : c'est LA lecture massive, et elle n'arrive plus 10x/s.
func LoadWorld(db *gorm.DB) (*World, error) {
	w := &World{db: db, dirty: make(map[uuid.UUID]dirtyFlag),
		moraleFrac: make(map[uuid.UUID]float64),
		seaSeconds: make(map[uuid.UUID]float64),
		prevState:  make(map[uuid.UUID]domain.FleetState)}
	w.registerWriteCounter()

	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.reloadLocked(); err != nil {
		return nil, err
	}
	logger.Info("[WORLD] state loaded in memory", "islands", len(w.islands))
	return w, nil
}

// registerWriteCounter branche un petit compteur sur GORM : à chaque INSERT,
// UPDATE, DELETE ou requête brute (Exec), writes augmente de 1.
func (w *World) registerWriteCounter() {
	mark := func(*gorm.DB) { w.writes.Add(1) }
	cb := w.db.Callback()
	_ = cb.Create().After("gorm:create").Register("world:count_write", mark)
	_ = cb.Update().After("gorm:update").Register("world:count_write", mark)
	_ = cb.Delete().After("gorm:delete").Register("world:count_write", mark)
	_ = cb.Raw().After("gorm:raw").Register("world:count_write", mark)
}

// -----------------------------------------------------------------------------
// Chargement depuis la base
// -----------------------------------------------------------------------------

func loadIslandsQuery(db *gorm.DB) *gorm.DB {
	return db.Preload("Buildings").
		Preload("Player").
		Preload("Player.Captains").
		Preload("Ships").
		Preload("Fleets.Ships")
}

// reloadLocked remplace TOUTE la mémoire par le contenu de la base.
// Attention : ce qui était "sale" et pas encore sauvegardé serait perdu, donc
// on ne l'appelle que quand la mémoire vient d'être sauvegardée (ou au boot).
func (w *World) reloadLocked() error {
	var islands []domain.Island
	if err := loadIslandsQuery(w.db).Find(&islands).Error; err != nil {
		return fmt.Errorf("world reload: %w", err)
	}
	// Les champs calculés (production/h, limites de stockage) ne sont pas en
	// base : on les recopie depuis l'ancienne mémoire pour que /status ne les
	// affiche pas vides jusqu'au prochain tick.
	for i := range islands {
		if old := w.islandLocked(islands[i].ID); old != nil {
			copyTransientFields(&islands[i], old)
		}
	}
	w.islands = islands
	w.reindexLocked()
	w.dirty = make(map[uuid.UUID]dirtyFlag)
	return nil
}

func (w *World) reindexLocked() {
	w.byID = make(map[uuid.UUID]int, len(w.islands))
	w.byPlayer = make(map[uuid.UUID]int, len(w.islands))
	for i := range w.islands {
		w.byID[w.islands[i].ID] = i
		if _, ok := w.byPlayer[w.islands[i].PlayerID]; !ok {
			w.byPlayer[w.islands[i].PlayerID] = i
		}
	}
}

func copyTransientFields(dst, src *domain.Island) {
	dst.StorageLimits = src.StorageLimits
	dst.ResourceGeneration = src.ResourceGeneration
	dst.ResourceGenerationBase = src.ResourceGenerationBase
	dst.ResourceGenerationBonus = src.ResourceGenerationBonus
}

func (w *World) islandLocked(id uuid.UUID) *domain.Island {
	if idx, ok := w.byID[id]; ok {
		return &w.islands[idx]
	}
	return nil
}

// -----------------------------------------------------------------------------
// Marquer "sale" et sauvegarder
// -----------------------------------------------------------------------------

// markDirty note qu'une partie d'une île a changé en mémoire.
// Ça ne coûte rien : on n'écrit rien en base ici.
func (w *World) markDirty(islandID uuid.UUID, flags dirtyFlag) {
	w.dirty[islandID] |= flags
}

// flushLocked écrit en base tout ce qui est sale, dans UNE transaction :
// soit tout passe, soit rien (et on réessaiera à la prochaine sauvegarde).
//
// On utilise Updates + Select("*") plutôt que Save : Save fait un INSERT si la
// ligne n'existe plus, ce qui pourrait "ressusciter" un navire détruit.
// Updates, lui, ne fait que mettre à jour une ligne existante.
func (w *World) flushLocked() error {
	if len(w.dirty) == 0 {
		return nil
	}
	start := time.Now()
	err := w.db.Transaction(func(tx *gorm.DB) error {
		for id, flags := range w.dirty {
			island := w.islandLocked(id)
			if island == nil {
				continue
			}
			if err := persistIsland(tx, island, flags); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		logger.Error("[WORLD] save failed, will retry", "error", err)
		return err
	}
	if n := len(w.dirty); n > 0 && time.Since(start) > 200*time.Millisecond {
		logger.Warn("[WORLD] slow save", "islands", n, "duration", time.Since(start).String())
	}
	w.dirty = make(map[uuid.UUID]dirtyFlag)
	return nil
}

func updateRow(tx *gorm.DB, model interface{}) error {
	return tx.Model(model).Select("*").Omit(clause.Associations).Updates(model).Error
}

// persistIsland écrit les parties sales d'une île.
func persistIsland(tx *gorm.DB, island *domain.Island, flags dirtyFlag) error {
	if flags&DirtyIsland != 0 {
		_ = island.BeforeSave(tx) // map Resources -> colonne JSON
		if err := tx.Model(&domain.Island{}).Where("id = ?", island.ID).Updates(map[string]interface{}{
			"resources":    island.ResourcesJSON,
			"last_updated": island.LastUpdated,
		}).Error; err != nil {
			return fmt.Errorf("save island %s: %w", island.ID, err)
		}
	}

	if flags&DirtyFleets != 0 {
		// Les navires d'une flotte existent en double en mémoire : dans
		// island.Fleets[].Ships (ceux qui bougent, la référence) et dans
		// island.Ships (liste à plat). On sauve d'abord ceux des flottes, puis
		// seulement les navires qui ne sont dans aucune flotte.
		saved := make(map[uuid.UUID]bool)
		for i := range island.Fleets {
			f := &island.Fleets[i]
			if err := updateRow(tx, f); err != nil {
				return fmt.Errorf("save fleet %s: %w", f.ID, err)
			}
			for j := range f.Ships {
				if err := updateRow(tx, &f.Ships[j]); err != nil {
					return fmt.Errorf("save ship %s: %w", f.Ships[j].ID, err)
				}
				saved[f.Ships[j].ID] = true
			}
		}
		for i := range island.Ships {
			if saved[island.Ships[i].ID] {
				continue
			}
			if err := updateRow(tx, &island.Ships[i]); err != nil {
				return fmt.Errorf("save ship %s: %w", island.Ships[i].ID, err)
			}
		}
	}

	if flags&DirtyBuildings != 0 {
		for i := range island.Buildings {
			if err := updateRow(tx, &island.Buildings[i]); err != nil {
				return fmt.Errorf("save building %s: %w", island.Buildings[i].ID, err)
			}
		}
	}

	if flags&DirtyCaptains != 0 {
		for i := range island.Player.Captains {
			c := &island.Player.Captains[i]
			if err := tx.Model(&domain.Captain{}).Where("id = ?", c.ID).Updates(map[string]interface{}{"level": c.Level, "xp": c.XP}).Error; err != nil {
				return fmt.Errorf("save captain %s: %w", c.ID, err)
			}
		}
	}

	if flags&DirtyPlayer != 0 && island.Player.ID != uuid.Nil {
		if err := updateRow(tx, &island.Player); err != nil {
			return fmt.Errorf("save player %s: %w", island.Player.ID, err)
		}
	}
	return nil
}

// Save sauvegarde maintenant ce qui est sale (utilisé par la boucle de
// sauvegarde périodique).
func (w *World) Save() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.flushLocked()
}

// RunAutosave lance la sauvegarde périodique jusqu'à ce que stop soit fermé.
func (w *World) RunAutosave(interval time.Duration, stop <-chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			_ = w.Save()
		case <-stop:
			return
		}
	}
}

// Close fait la sauvegarde finale. À appeler à l'arrêt propre du serveur,
// APRÈS avoir arrêté le tick et les requêtes HTTP.
func (w *World) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	n := len(w.dirty)
	if err := w.flushLocked(); err != nil {
		return err
	}
	logger.Info("[WORLD] final save done", "islands_saved", n)
	return nil
}

// -----------------------------------------------------------------------------
// La passerelle pour le code qui travaille encore directement sur la base
// -----------------------------------------------------------------------------

// runWithDBLocked exécute du code "à l'ancienne" (qui lit/écrit la base) sans
// casser la mémoire. Le verrou doit déjà être pris.
//
//  1. flush   : la base reçoit l'état le plus récent de la mémoire
//  2. fn()    : le code lit des données fraîches et écrit ce qu'il veut,
//     tout de suite (c'est là que les achats/pillages sont
//     enregistrés en transaction)
//  3. reload  : si fn a écrit quelque chose, la mémoire se recale sur la base
//
// Pendant tout ça le tick est bloqué par le verrou : aucune copie périmée ne
// peut écraser ce que fn vient d'écrire.
func (w *World) runWithDBLocked(fn func() error) error {
	if err := w.flushLocked(); err != nil {
		// La base n'a pas reçu l'état récent. Laisser fn lire des données
		// périmées puis recharger ferait perdre de la progression : on refuse.
		return fmt.Errorf("world not saved, action refused: %w", err)
	}
	before := w.writes.Load()
	fnErr := fn()
	if w.writes.Load() != before {
		if err := w.reloadLocked(); err != nil {
			logger.Error("[WORLD] reload after write failed", "error", err)
		}
	}
	return fnErr
}

// RunWithDB : version qui prend le verrou elle-même.
func (w *World) RunWithDB(fn func() error) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.runWithDBLocked(fn)
}

// Middleware Echo : fait passer chaque requête HTTP par RunWithDB.
// Les handlers n'ont pas besoin d'être modifiés pour être protégés.
func (w *World) Middleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		var handlerErr error
		err := w.RunWithDB(func() error {
			handlerErr = next(c)
			return nil
		})
		if err != nil {
			return c.JSON(503, map[string]string{"error": "server busy, retry"})
		}
		return handlerErr
	}
}

// -----------------------------------------------------------------------------
// Lecture depuis l'extérieur du package (ex : /status)
// -----------------------------------------------------------------------------

// WithPlayerIsland donne accès à l'île (en mémoire) d'un joueur, verrou pris.
// fn reçoit nil si le joueur n'a pas d'île chargée. fn ne doit PAS garder le
// pointeur après son retour : une fois le verrou rendu, le tick peut modifier
// l'île à tout moment.
func (w *World) WithPlayerIsland(playerID uuid.UUID, fn func(island *domain.Island, tools *IslandTools)) {
	w.mu.Lock()
	defer w.mu.Unlock()
	idx, ok := w.byPlayer[playerID]
	if !ok {
		fn(nil, nil)
		return
	}
	fn(&w.islands[idx], &IslandTools{w: w, islandID: w.islands[idx].ID})
}

// IslandTools : ce qu'un handler a le droit de faire sur une île en mémoire.
type IslandTools struct {
	w        *World
	islandID uuid.UUID
}

// MarkDirty signale une modification faite directement en mémoire.
func (t *IslandTools) MarkDirty(flags dirtyFlag) { t.w.markDirty(t.islandID, flags) }

// CompleteResearchIfDue termine la recherche si son timer est écoulé.
func (t *IslandTools) CompleteResearchIfDue(now time.Time) bool {
	island := t.w.islandLocked(t.islandID)
	if island != nil && completeResearchIfDue(island, now) {
		t.w.markDirty(t.islandID, DirtyPlayer)
		return true
	}
	return false
}

// RunWithDB exécute du code base de données (verrou déjà pris), puis rend
// l'île rafraîchie (le pointeur peut changer si la mémoire a été rechargée).
func (t *IslandTools) RunWithDB(fn func() error) (*domain.Island, error) {
	err := t.w.runWithDBLocked(fn)
	return t.w.islandLocked(t.islandID), err
}
