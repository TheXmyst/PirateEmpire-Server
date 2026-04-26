package economy

// TechBonuses is a deprecated wrapper for TechModifiers (STUB - for backward compatibility)
// This provides the old field names for backward compatibility.
type TechBonuses struct {
	TechModifiers
	// Old field names (deprecated, use TechModifiers fields directly)
	BuildTimeReduce    float64
	ResearchTimeReduce float64
}

// CalculateTechBonuses is a deprecated alias for ComputeTechModifiers (STUB - for backward compatibility)
// This is a stub function to fix compilation errors. Code should migrate to ComputeTechModifiers.
func CalculateTechBonuses(techIDs []string) TechBonuses {
	mods := ComputeTechModifiers(techIDs)
	return TechBonuses{
		TechModifiers:      mods,
		BuildTimeReduce:    mods.BuildTimeReduction,
		ResearchTimeReduce: mods.ResearchTimeReduction,
	}
}
