package economy

// DisplayNameBuilding returns the French display name for a building type
func DisplayNameBuilding(buildingType string) string {
	names := map[string]string{
		"Hôtel de Ville":        "Hôtel de Ville",
		"Scierie":               "Scierie",
		"Carrière":              "Carrière",
		"Mine d'Or":             "Mine d'Or",
		"Distillerie":           "Distillerie",
		"Académie":              "Académie",
		"Chantier Naval":        "Chantier Naval",
		"Entrepôt":              "Entrepôt",
		"Le Nid du Flibustier":  "Le Nid du Flibustier",
		"La Chambre des Plans":  "La Chambre des Plans",
		"Tavern":                "Taverne",
		"Milice":                "Milice",
	}
	if name, ok := names[buildingType]; ok {
		return name
	}
	return buildingType // Fallback to type if not found
}

// DisplayNameTech returns the French display name for a technology ID
func DisplayNameTech(techID string) string {
	// Try to get from loaded techs first
	tech, err := GetTech(techID)
	if err == nil && tech.Name != "" {
		return tech.Name
	}

	// Fallback mapping for common techs
	names := map[string]string{
		"log_build_1":     "Construction I",
		"log_build_2":     "Construction II",
		"log_build_3":     "Construction III",
		"log_build_4":     "Construction IV",
		"log_research_2":  "Recherche II",
		"log_research_4":  "Recherche IV",
		"log_research_5":  "Recherche V",
		"log_queue_2":     "File d'attente II",
		"tech_naval_1":    "Navigation I",
		"tech_naval_2":    "Navigation II",
		"tech_naval_3":    "Navigation III",
		"tech_storage_1":  "Stockage I",
		"tech_storage_2":  "Stockage II",
		"tech_storage_3":  "Stockage III",
		"tech_storage_4":  "Stockage IV",
		"tech_storage_5":  "Stockage V",
		"eco_wood_1":      "Exploitation du bois I",
		"eco_wood_2":      "Exploitation du bois II",
		"eco_wood_3":      "Exploitation du bois III",
		"eco_stone_1":     "Exploitation de la pierre I",
		"eco_stone_2":     "Exploitation de la pierre II",
		"eco_stone_3":     "Exploitation de la pierre III",
		"eco_rum_1":       "Distillation I",
		"eco_rum_2":       "Distillation II",
		"eco_rum_3":       "Distillation III",
		"combat_guerrier_1": "Combat Guerrier I",
		"combat_guerrier_2": "Combat Guerrier II",
		"combat_dmg_3":      "Dommages III",
	}
	if name, ok := names[techID]; ok {
		return name
	}
	return techID // Fallback to ID if not found
}

// DisplayNameShipType returns the French display name for a ship type
func DisplayNameShipType(shipType string) string {
	names := map[string]string{
		"sloop":     "Sloop",
		"brigantine": "Brigantine",
		"frigate":   "Frégate",
		"galleon":   "Galion",
		"manowar":   "Man-o'-War",
	}
	if name, ok := names[shipType]; ok {
		return name
	}
	return shipType // Fallback to type if not found
}

// DisplayNameResource returns the French display name for a resource type
func DisplayNameResource(resType string) string {
	names := map[string]string{
		"wood":  "Bois",
		"stone": "Pierre",
		"gold":  "Or",
		"rum":   "Rhum",
	}
	if name, ok := names[resType]; ok {
		return name
	}
	return resType // Fallback to type if not found
}

