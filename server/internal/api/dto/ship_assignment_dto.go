package dto

// ShipAssignmentDTO represents the response when assigning/unassigning a captain to/from a ship
// Contains both the updated Captain and Ship state after the operation
type ShipAssignmentDTO struct {
	Captain CaptainDTO `json:"captain"`
	Ship    ShipDTO    `json:"ship"`
}
