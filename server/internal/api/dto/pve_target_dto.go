package dto

import "time"

// PveTargetDTO represents a PVE target for API responses with string UUIDs
type PveTargetDTO struct {
	ID            string      `json:"id"`
	LegacyID      string      `json:"legacy_id,omitempty"`
	X             int         `json:"x"`
	Y             int         `json:"y"`
	Tier          int         `json:"tier"`
	Name          string      `json:"name"`
	RealX         float64     `json:"real_x"`
	RealY         float64     `json:"real_y"`
	TargetX       float64     `json:"target_x"`
	TargetY       float64     `json:"target_y"`
	Speed         float64     `json:"speed"`
	NextChangeAt  time.Time   `json:"next_change_at,omitempty"`
	Captain       *CaptainDTO `json:"captain,omitempty"`
}
