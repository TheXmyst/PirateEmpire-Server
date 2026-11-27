package economy

// =============================================================================
//  MORAL DE CROISIÈRE ET XP DES CAPITAINES  (fleet_morale.go)
// =============================================================================
//
// Le moral d'une flotte (0-100) vit maintenant :
//   - au port (Idle)   : il remonte de 2 par minute jusqu'à 100 ;
//   - partout ailleurs : il baisse de 1 toutes les 2 minutes,
//     1 par minute si la cale n'a plus de rhum ;
//   - après un combat  : victoire +10, défaite −15.
// Les capitaines peuvent ralentir la baisse, accélérer la remontée, donner du
// moral au départ ou après une victoire.
//
// Les capitaines gagnent de l'XP en combattant et en naviguant. Passer du
// niveau N au niveau N+1 demande 40 × N XP (niveau max 80).
// =============================================================================

import "github.com/TheXmyst/Sea-Dogs/server/internal/domain"

const (
	DefaultFleetMorale        = 50
	MoralePortRecoveryPerMin  = 2.0
	MoraleSeaDecayPerMin      = 0.5
	MoraleSeaDecayNoRumPerMin = 1.0
	MoraleLowThreshold        = 40
	MoraleVictoryGain         = 10
	MoraleDefeatLoss          = 15
	CaptainMaxLevel           = 80
	CaptainXPPerLevelFactor   = 40
	CaptainXPPerMinuteAtSea   = 1.0
	CaptainXPDefeat           = 10
	CaptainXPPvPVictory       = 100
)

// CaptainXPPvEVictory : XP d'une victoire PvE selon le palier de la cible.
var CaptainXPPvEVictory = map[int]int{1: 20, 2: 40, 3: 70}

// MoraleRatePerSecond renvoie de combien le moral d'une flotte change par
// seconde dans son état actuel (positif au port, négatif en mer).
func MoraleRatePerSecond(f *domain.Fleet, e CaptainEffects) float64 {
	morale := FleetMorale(f)
	if f.State == domain.FleetStateIdle {
		if morale >= 100 {
			return 0
		}
		return MoralePortRecoveryPerMin * (1 + e.PortRecoveryPct) / 60
	}
	if e.NoSeaDecay || morale <= 0 {
		return 0
	}
	rate := MoraleSeaDecayPerMin
	if f.Cargo == nil || f.Cargo[domain.Rum] <= 0 {
		rate = MoraleSeaDecayNoRumPerMin
	}
	rate *= 1 - e.SeaDecayReduction
	if morale < MoraleLowThreshold {
		rate *= 1 - e.LowMoraleDecayReduction
	}
	if rate < 0 {
		rate = 0
	}
	return -rate / 60
}

// SetFleetMorale fixe le moral (borné 0-100).
func SetFleetMorale(f *domain.Fleet, value int) {
	v := clampMorale(value)
	f.MoraleCruise = &v
}

// ApplyCombatMorale : moral après un combat (winner = "fleet_a", "fleet_b", "draw").
func ApplyCombatMorale(f *domain.Fleet, won, lost bool, e CaptainEffects) {
	m := FleetMorale(f)
	if won {
		m += MoraleVictoryGain + e.VictoryMorale
	} else if lost {
		m -= MoraleDefeatLoss
	}
	SetFleetMorale(f, m)
}

// XPToNextLevel : XP à accumuler pour passer du niveau `level` au suivant.
func XPToNextLevel(level int) int {
	if level >= CaptainMaxLevel {
		return 0
	}
	if level < 1 {
		level = 1
	}
	return CaptainXPPerLevelFactor * level
}

// AddCaptainXP ajoute de l'XP et fait monter de niveau. Renvoie le nombre de
// niveaux gagnés. Captain.XP = XP accumulée dans le niveau actuel.
func AddCaptainXP(c *domain.Captain, xp int) int {
	if c == nil || xp <= 0 {
		return 0
	}
	if c.Level < 1 {
		c.Level = 1
	}
	if c.Level >= CaptainMaxLevel {
		c.Level = CaptainMaxLevel
		c.XP = 0
		return 0
	}
	c.XP += xp
	gained := 0
	for c.Level < CaptainMaxLevel {
		need := XPToNextLevel(c.Level)
		if c.XP < need {
			break
		}
		c.XP -= need
		c.Level++
		gained++
	}
	if c.Level >= CaptainMaxLevel {
		c.XP = 0
	}
	return gained
}
