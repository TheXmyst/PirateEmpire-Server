package game

import (
	"fmt"
	"strings"

	"github.com/TheXmyst/Sea-Dogs/client/internal/client"
	"github.com/TheXmyst/Sea-Dogs/client/internal/domain"
)

// handleAPIError handles API errors and opens the prerequisites modal if needed
// Returns true if the error was handled (prerequisites modal opened)
func (g *Game) handleAPIError(err error, action string) bool {
	if err == nil {
		return false
	}

	// Check if it's a requirements error
	reqErr, ok := err.(*client.ErrRequirementsNotMet)
	if ok {
		fmt.Printf("[PREREQ] blocked action=%s missing=%d\n", action, len(reqErr.Requirements))
		g.openPrereqModal(reqErr.Title, reqErr.Subtitle, reqErr.Requirements)
		return true
	}

	// Check if error message contains prerequisite keywords (fallback for legacy endpoints)
	errMsg := strings.ToLower(err.Error())
	prereqKeywords := []string{
		"requires", "requis", "prereq", "prérequis", "townhall", "academy", "shipyard",
		"technologie", "technology", "non recherchée", "non construit", "niveau",
		"level", "manque", "bloqué", "verrouillé", "locked",
	}
	for _, keyword := range prereqKeywords {
		if strings.Contains(errMsg, keyword) {
			// Convert to a minimal requirements error
			reqs := []domain.Requirement{
				{
					Kind:    "other",
					ID:      action,
					Name:    action,
					Message: err.Error(),
				},
			}
			fmt.Printf("[PREREQ] fallback action=%s msg=%s\n", action, err.Error())
			g.openPrereqModal("PRÉREQUIS MANQUANTS", "", reqs)
			return true
		}
	}

	// Not a prerequisites error - return false to allow standard error handling
	return false
}
