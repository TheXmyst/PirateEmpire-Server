package game

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode"

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
					Message: friendlyError(err),
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

var (
	reHTTPStatus  = regexp.MustCompile(`\(?\b[1-5]\d\d [A-Z][A-Za-z' ]*\)?( - )?`)
	reTechPrefix  = regexp.MustCompile(`^[a-zA-Z ]+ failed: *`)
	reJSONMessage = regexp.MustCompile(`\{.*\}`)
)

// friendlyError : le message d'une erreur, propre pour un joueur. Il enlève
// les codes HTTP (« 400 Bad Request »), les préfixes techniques en anglais
// (« upgrade failed: ») et garde seulement la phrase du serveur.
func friendlyError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	if m := reJSONMessage.FindString(msg); m != "" {
		var e struct {
			Error   string `json:"error"`
			Message string `json:"message"`
		}
		if json.Unmarshal([]byte(m), &e) == nil {
			if e.Error != "" {
				msg = e.Error
			} else if e.Message != "" {
				msg = e.Message
			}
		}
	}
	msg = reTechPrefix.ReplaceAllString(msg, "")
	msg = reHTTPStatus.ReplaceAllString(msg, "")
	msg = strings.Trim(msg, " \t:-")
	if msg == "" {
		return "Action impossible pour le moment"
	}
	r := []rune(msg)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}
