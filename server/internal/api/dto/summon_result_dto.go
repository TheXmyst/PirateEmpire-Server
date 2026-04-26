package dto

// SummonResultDTO represents a single summon result with DTO conversion
type SummonResultDTO struct {
	Rarity        string      `json:"rarity"`
	TemplateID    string      `json:"template_id"`
	Name          string      `json:"name"`
	IsDuplicate   bool        `json:"is_duplicate"`
	ShardsGranted int         `json:"shards_granted,omitempty"`
	Captain       *CaptainDTO `json:"captain,omitempty"` // nil if duplicate
}
