package economy

import (
	"math"
	"testing"
	"time"

	"github.com/TheXmyst/Sea-Dogs/server/internal/api/dto"
	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
)

func TestFillHarvestInfoGoldRichness(t *testing.T) {
	now := time.Now()
	f := &domain.Fleet{State: domain.FleetStateStationed, StationedAt: &now, StoredResource: string(domain.Gold), NodeRichness: 1.5}
	d := &dto.FleetDTO{}
	FillHarvestInfo(f, d)
	if math.Abs(d.HarvestRatePerHour-540) > 1 {
		t.Fatalf("débit or x1.5 attendu ~540/h, obtenu %.1f", d.HarvestRatePerHour)
	}
	if d.NodeRichness != 1.5 || math.Abs(d.HarvestDurationMult-1) > 0.01 {
		t.Fatalf("richesse/durée inattendues: %v %v", d.NodeRichness, d.HarvestDurationMult)
	}

	later := now.Add(-200 * time.Minute)
	f.StationedAt = &later
	FillHarvestInfo(f, d)
	if math.Abs(d.HarvestRatePerHour-1080) > 1 || d.HarvestDurationMult != 2 {
		t.Fatalf("après 200 min attendu 1080/h x2, obtenu %.1f x%.2f", d.HarvestRatePerHour, d.HarvestDurationMult)
	}
}

func TestFillHarvestInfoNotStationed(t *testing.T) {
	f := &domain.Fleet{State: domain.FleetStateIdle, NodeRichness: 1.5}
	d := &dto.FleetDTO{}
	FillHarvestInfo(f, d)
	if d.HarvestRatePerHour != 0 {
		t.Fatalf("flotte non stationnée : pas de débit attendu")
	}
}
