package shared

import (
    "database/sql/driver"
    "encoding/json"
    "fmt"

    "github.com/google/uuid"
)

// UUIDString is a thin wrapper around github.com/google/uuid.UUID
// that marshals to/from JSON as a string and implements
// database/sql Scanner/Valuer for compatibility with GORM.
type UUIDString uuid.UUID

// Value implements driver.Valuer (for DB writes).
func (u UUIDString) Value() (driver.Value, error) {
    uu := uuid.UUID(u)
    if uu == uuid.Nil {
        return nil, nil
    }
    return uu.String(), nil
}

// Scan implements sql.Scanner (for DB reads).
func (u *UUIDString) Scan(value interface{}) error {
    if value == nil {
        *u = UUIDString(uuid.Nil)
        return nil
    }
    switch v := value.(type) {
    case string:
        parsed, err := uuid.Parse(v)
        if err != nil {
            return err
        }
        *u = UUIDString(parsed)
        return nil
    case []byte:
        parsed, err := uuid.ParseBytes(v)
        if err != nil {
            return err
        }
        *u = UUIDString(parsed)
        return nil
    default:
        return fmt.Errorf("unsupported scan type %T for UUIDString", value)
    }
}

// MarshalJSON encodes the UUID as a JSON string.
func (u UUIDString) MarshalJSON() ([]byte, error) {
    uu := uuid.UUID(u)
    if uu == uuid.Nil {
        return json.Marshal("")
    }
    return json.Marshal(uu.String())
}

// UnmarshalJSON decodes a JSON string into the UUID.
func (u *UUIDString) UnmarshalJSON(b []byte) error {
    var s string
    if err := json.Unmarshal(b, &s); err != nil {
        return err
    }
    if s == "" {
        *u = UUIDString(uuid.Nil)
        return nil
    }
    parsed, err := uuid.Parse(s)
    if err != nil {
        return err
    }
    *u = UUIDString(parsed)
    return nil
}

// String returns the canonical string form of the underlying UUID.
func (u UUIDString) String() string {
    return uuid.UUID(u).String()
}
