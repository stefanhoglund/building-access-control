// Package config loads the versioned System Configuration
// (architecture.md section 5.8, ADR-004). It is loaded only by the
// Autonomous Decision Service in this slice.
package config

import (
	"encoding/json"
	"fmt"
	"os"
)

type Zone struct {
	ZoneID string `json:"zone_id"`
	Name   string `json:"name"`
}

type ControlledDoor struct {
	DoorID     string `json:"door_id"`
	FromZone   string `json:"from_zone"`
	ToZone     string `json:"to_zone"`
	Direction  string `json:"direction"`
	ReaderID   string `json:"reader_id"`
	ActuatorID string `json:"actuator_id"`
}

type Role struct {
	RoleID string `json:"role_id"`
}

type User struct {
	UserID string   `json:"user_id"`
	Roles  []string `json:"roles"`
}

// Card status values (design-specification.md section 2.2). Only ACTIVE
// is exercised by this slice.
const (
	CardStatusActive = "ACTIVE"
)

type AccessCard struct {
	CardID string `json:"card_id"`
	UserID string `json:"user_id"`
	Status string `json:"status"`
}

// Permission subject types and effects (design-specification.md section 2.4).
const (
	SubjectUser = "USER"
	SubjectRole = "ROLE"

	EffectAllow = "ALLOW"
	EffectDeny  = "DENY"
)

type Permission struct {
	SubjectType string `json:"subject_type"`
	SubjectID   string `json:"subject_id"`
	DoorID      string `json:"door_id"`
	Direction   string `json:"direction"`
	Effect      string `json:"effect"`
	Enabled     bool   `json:"enabled"`
}

// Config is the full System Configuration document
// (docs/data-contracts.md section 7).
type Config struct {
	Zones           []Zone           `json:"zones"`
	ControlledDoors []ControlledDoor `json:"controlled_doors"`
	Roles           []Role           `json:"roles"`
	Users           []User           `json:"users"`
	AccessCards     []AccessCard     `json:"access_cards"`
	Permissions     []Permission     `json:"permissions"`
}

// Load reads and parses the System Configuration file at path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: read %s: %w", path, err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("config: parse %s: %w", path, err)
	}
	return &cfg, nil
}

// FindCard returns the AccessCard with the given ID, if any.
func (c *Config) FindCard(cardID string) (AccessCard, bool) {
	for _, card := range c.AccessCards {
		if card.CardID == cardID {
			return card, true
		}
	}
	return AccessCard{}, false
}

// FindUser returns the User with the given ID, if any.
func (c *Config) FindUser(userID string) (User, bool) {
	for _, u := range c.Users {
		if u.UserID == userID {
			return u, true
		}
	}
	return User{}, false
}

// PermissionsFor returns every enabled Permission whose controlled
// boundary and direction match doorID/direction, regardless of subject.
// Precedence among them is evaluated by the authorization package
// (ADR-001), not here.
func (c *Config) PermissionsFor(doorID, direction string) []Permission {
	var matched []Permission
	for _, p := range c.Permissions {
		if !p.Enabled {
			continue
		}
		if p.DoorID != doorID || p.Direction != direction {
			continue
		}
		matched = append(matched, p)
	}
	return matched
}
