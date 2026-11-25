package engine

import (
	"fmt"
	"testing"
	"time"

	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
	"github.com/TheXmyst/Sea-Dogs/server/internal/economy"
	"github.com/TheXmyst/Sea-Dogs/server/internal/repository"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// newTestDB ouvre une base SQLite en mémoire, propre à chaque test.
func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", uuid.NewString())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { sqlDB.Close() })
	if err := db.AutoMigrate(&domain.Player{}, &domain.Island{}, &domain.Building{}, &domain.Ship{}, &domain.Fleet{}, &domain.Captain{}); err != nil {
		t.Fatal(err)
	}
	repository.DB = db
	if err := economy.LoadConfig("../../configs/buildings.json"); err != nil {
		t.Fatal(err)
	}
	return db
}

type fixture struct {
	player domain.Player
	island domain.Island
	fleet  domain.Fleet
	ship   domain.Ship
}

// seed crée un joueur, une île avec une scierie, une flotte et un navire.
func seed(t *testing.T, db *gorm.DB, gold float64) fixture {
	t.Helper()
	f := fixture{}
	f.player = domain.Player{ID: uuid.New(), Username: "bob-" + uuid.NewString()[:6]}
	f.island = domain.Island{
		ID: uuid.New(), PlayerID: f.player.ID, Name: "Tortuga", X: 100, Y: 100,
		Resources:   map[domain.ResourceType]float64{domain.Gold: gold, domain.Wood: 0},
		LastUpdated: time.Now(),
	}
	f.fleet = domain.Fleet{ID: uuid.New(), IslandID: f.island.ID, Name: "Flotte 1", State: domain.FleetStateIdle}
	f.ship = domain.Ship{ID: uuid.New(), PlayerID: f.player.ID, IslandID: f.island.ID, FleetID: &f.fleet.ID, Name: "Perle", Type: "sloop", State: "Ready", X: 100, Y: 100}
	building := domain.Building{ID: uuid.New(), IslandID: f.island.ID, Type: "Scierie", Level: 5}

	for _, v := range []interface{}{&f.player, &f.island, &f.fleet, &f.ship, &building} {
		if err := db.Omit("Player", "Island").Create(v).Error; err != nil {
			t.Fatal(err)
		}
	}
	return f
}

func dbGold(t *testing.T, db *gorm.DB, islandID uuid.UUID) float64 {
	t.Helper()
	var island domain.Island
	if err := db.First(&island, "id = ?", islandID).Error; err != nil {
		t.Fatal(err)
	}
	return island.Resources[domain.Gold]
}

// Le tick fait avancer la production EN MÉMOIRE sans rien écrire en base ;
// la sauvegarde écrit ensuite le résultat.
func TestTickWorksInMemoryThenSaveWrites(t *testing.T) {
	db := newTestDB(t)
	f := seed(t, db, 1000)
	w, err := LoadWorld(db)
	if err != nil {
		t.Fatal(err)
	}

	w.mu.Lock()
	island := w.islandLocked(f.island.ID)
	island.LastUpdated = time.Now().Add(-1 * time.Hour) // 1 h de production à compter
	writesBefore := w.writes.Load()
	tickIsland(w, island, time.Now())
	wood := island.Resources[domain.Wood]
	writesDuringTick := w.writes.Load() - writesBefore
	w.mu.Unlock()

	if wood <= 0 {
		t.Fatalf("expected wood production in memory, got %v", wood)
	}
	if writesDuringTick != 0 {
		t.Fatalf("tick must not write to the DB, got %d writes", writesDuringTick)
	}

	var before domain.Island
	db.First(&before, "id = ?", f.island.ID)
	if before.Resources[domain.Wood] != 0 {
		t.Fatalf("DB should not change before save, got wood=%v", before.Resources[domain.Wood])
	}

	if err := w.Save(); err != nil {
		t.Fatal(err)
	}
	var after domain.Island
	db.First(&after, "id = ?", f.island.ID)
	if after.Resources[domain.Wood] != wood {
		t.Fatalf("save should write memory value %v, DB has %v", wood, after.Resources[domain.Wood])
	}
}

// Le bug de l'audit : un pillage écrit en base ne doit plus être écrasé par
// une copie périmée de l'île. Avec la passerelle, la mémoire est rechargée
// après l'écriture, donc l'or volé ne réapparaît pas.
func TestPillageIsNotUndoneByTheTick(t *testing.T) {
	db := newTestDB(t)
	f := seed(t, db, 1000)
	w, err := LoadWorld(db)
	if err != nil {
		t.Fatal(err)
	}

	// Un handler "à l'ancienne" retire 500 or au défenseur, en transaction.
	err = w.RunWithDB(func() error {
		return repository.GetDB().Transaction(func(tx *gorm.DB) error {
			var target domain.Island
			if err := tx.First(&target, "id = ?", f.island.ID).Error; err != nil {
				return err
			}
			target.Resources[domain.Gold] -= 500
			return tx.Omit("Player").Save(&target).Error
		})
	})
	if err != nil {
		t.Fatal(err)
	}

	// Le tick tourne et la sauvegarde périodique passe.
	w.mu.Lock()
	tickIsland(w, w.islandLocked(f.island.ID), time.Now())
	memGold := w.islandLocked(f.island.ID).Resources[domain.Gold]
	w.mu.Unlock()
	if err := w.Save(); err != nil {
		t.Fatal(err)
	}

	if memGold > 501 {
		t.Fatalf("memory still has the stolen gold: %v", memGold)
	}
	if got := dbGold(t, db, f.island.ID); got > 501 {
		t.Fatalf("DB got the stolen gold back: %v", got)
	}
}

// La passerelle sauvegarde la mémoire AVANT le handler : un achat voit donc
// les ressources produites depuis la dernière sauvegarde.
func TestHandlerSeesFreshMemoryState(t *testing.T) {
	db := newTestDB(t)
	f := seed(t, db, 1000)
	w, _ := LoadWorld(db)

	w.mu.Lock()
	island := w.islandLocked(f.island.ID)
	island.Resources[domain.Gold] = 4242
	w.markDirty(island.ID, DirtyIsland)
	w.mu.Unlock()

	var seen float64
	_ = w.RunWithDB(func() error {
		seen = dbGold(t, repository.GetDB(), f.island.ID)
		return nil
	})
	if seen != 4242 {
		t.Fatalf("handler should read the fresh value 4242, read %v", seen)
	}
}

// Un handler qui ne fait que lire ne déclenche pas de rechargement.
func TestReadOnlyHandlerDoesNotReload(t *testing.T) {
	db := newTestDB(t)
	f := seed(t, db, 1000)
	w, _ := LoadWorld(db)

	w.mu.Lock()
	w.islandLocked(f.island.ID).ResourceGeneration = map[domain.ResourceType]float64{domain.Wood: 1}
	ptr := w.islandLocked(f.island.ID)
	w.mu.Unlock()

	_ = w.RunWithDB(func() error {
		var p domain.Player
		return repository.GetDB().First(&p, "id = ?", f.player.ID).Error
	})

	w.mu.Lock()
	defer w.mu.Unlock()
	if &w.islands[w.byID[f.island.ID]] != ptr {
		t.Fatal("world was reloaded after a read-only request")
	}
}

// Close (arrêt propre) écrit tout ce qui reste en mémoire.
func TestCloseSavesPendingChanges(t *testing.T) {
	db := newTestDB(t)
	f := seed(t, db, 1000)
	w, _ := LoadWorld(db)

	w.mu.Lock()
	island := w.islandLocked(f.island.ID)
	island.Resources[domain.Gold] = 777
	island.Fleets[0].Ships[0].X = 321
	w.markDirty(island.ID, DirtyIsland|DirtyFleets)
	w.mu.Unlock()

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if got := dbGold(t, db, f.island.ID); got != 777 {
		t.Fatalf("gold not saved on close: %v", got)
	}
	var ship domain.Ship
	db.First(&ship, "id = ?", f.ship.ID)
	if ship.X != 321 {
		t.Fatalf("ship position not saved on close: %v", ship.X)
	}
}

// La sauvegarde ne doit jamais recréer une ligne supprimée entre-temps
// (par exemple un navire détruit).
func TestSaveNeverResurrectsDeletedRows(t *testing.T) {
	db := newTestDB(t)
	f := seed(t, db, 1000)
	w, _ := LoadWorld(db)

	w.mu.Lock()
	w.markDirty(f.island.ID, DirtyFleets)
	db.Delete(&domain.Ship{}, "id = ?", f.ship.ID)
	err := w.flushLocked()
	w.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}

	var count int64
	db.Model(&domain.Ship{}).Where("id = ?", f.ship.ID).Count(&count)
	if count != 0 {
		t.Fatal("deleted ship was re-created by the save")
	}
}

// Une flotte en route vers une attaque avance en mémoire, sans écriture.
func TestTravelingFleetMovesInMemory(t *testing.T) {
	db := newTestDB(t)
	f := seed(t, db, 1000)
	targetIsland := uuid.New()
	tx, ty := 1000, 100
	db.Model(&domain.Fleet{}).Where("id = ?", f.fleet.ID).Updates(map[string]interface{}{
		"state": domain.FleetStateTravelingToAttack, "target_x": tx, "target_y": ty, "target_island_id": targetIsland,
	})
	w, _ := LoadWorld(db)

	w.mu.Lock()
	before := w.writes.Load()
	w.processTravelFleets()
	ship := w.islandLocked(f.island.ID).Fleets[0].Ships[0]
	writes := w.writes.Load() - before
	dirty := w.dirty[f.island.ID]
	w.mu.Unlock()

	if ship.X <= 100 {
		t.Fatalf("fleet should have moved toward x=1000, x=%v", ship.X)
	}
	if writes != 0 {
		t.Fatalf("movement must not write to the DB, got %d writes", writes)
	}
	if dirty&DirtyFleets == 0 {
		t.Fatal("moved fleet should be marked dirty")
	}
}
