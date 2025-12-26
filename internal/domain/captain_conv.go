package domain

import (
    "github.com/TheXmyst/Sea-Dogs/server/internal/api/dto"
    "github.com/google/uuid"
)

// ToDTO converts a domain Captain to its API DTO representation.
func (c *Captain) ToDTO() dto.CaptainDTO {
    var assigned *string
    if c.AssignedShipID != nil {
        s := c.AssignedShipID.String()
        assigned = &s
    }

    return dto.CaptainDTO{
        ID:             c.ID.String(),
        PlayerID:       c.PlayerID.String(),
        TemplateID:     c.TemplateID,
        Name:           c.Name,
        Rarity:         string(c.Rarity),
        Level:          c.Level,
        XP:             c.XP,
        Stars:          c.Stars,
        SkillID:        c.SkillID,
        AssignedShipID: assigned,
        InjuredUntil:   c.InjuredUntil,
        CreatedAt:      c.CreatedAt,
        UpdatedAt:      c.UpdatedAt,
    }
}

// FromDTO fills a domain Captain from a DTO. Returns error on invalid UUIDs.
func (c *Captain) FromDTO(d dto.CaptainDTO) error {
    var err error
    if c.ID, err = uuid.Parse(d.ID); err != nil {
        return err
    }
    if c.PlayerID, err = uuid.Parse(d.PlayerID); err != nil {
        return err
    }
    c.TemplateID = d.TemplateID
    c.Name = d.Name
    c.Rarity = CaptainRarity(d.Rarity)
    c.Level = d.Level
    c.XP = d.XP
    c.Stars = d.Stars
    c.SkillID = d.SkillID
    if d.AssignedShipID != nil && *d.AssignedShipID != "" {
        parsed, err := uuid.Parse(*d.AssignedShipID)
        if err != nil {
            return err
        }
        c.AssignedShipID = &parsed
    } else {
        c.AssignedShipID = nil
    }
    c.InjuredUntil = d.InjuredUntil
    c.CreatedAt = d.CreatedAt
    c.UpdatedAt = d.UpdatedAt
    return nil
}
