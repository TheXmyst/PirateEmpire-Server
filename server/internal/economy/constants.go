package economy

const (
	// PlayerTravelSpeedMultiplier multiplies the final travel speed of player fleets (x4)
	PlayerTravelSpeedMultiplier = 4.0

	// NpcTravelSpeedMultiplier multiplies the final travel speed of NPC fleets (x1)
	NpcTravelSpeedMultiplier = 1.0

	// PlayerRumConsumptionMultiplier multiplies the rum consumption rate (only when moving)
	PlayerRumConsumptionMultiplier = 1.0 // Rééquilibrage : 4 -> 1 (0,02 rhum/s par navire, soit 72/h)
)
