package api

import (
	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
	"github.com/labstack/echo/v4"
)

// NewRequirementsError creates a new RequirementsError
func NewRequirementsError(reqs []domain.Requirement) *domain.RequirementsError {
	return &domain.RequirementsError{
		Code:         "REQUIREMENTS_NOT_MET",
		Message:      "Prérequis non remplis",
		Error:        "Prérequis non remplis", // Pour compatibilité
		Requirements: reqs,
	}
}

// WriteRequirementsError writes a RequirementsError to the HTTP response
func WriteRequirementsError(c echo.Context, status int, reqs []domain.Requirement) error {
	err := NewRequirementsError(reqs)
	return c.JSON(status, err)
}

