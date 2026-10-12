package simulator

import (
	"encoding/json"
	"fmt"
	"os"
)

// Person is a minimal SimulatedPerson (design-specification.md section
// 3.1/3.2): a stable simulation identifier, the card it presents, and the
// normal hour-of-day window scenario generation treats as "typical" for
// it. This is simulation-domain data, separate from System Configuration
// (ADR-004) — NormalHourStart/End describes expected behavior, it does
// not grant or restrict permission (section 3.2: "Schedules describe
// typical behavior. They do not grant access permission.").
type Person struct {
	PersonID        string `json:"person_id"`
	CardID          string `json:"card_id"`
	RoleLabel       string `json:"role_label"`
	NormalHourStart int    `json:"normal_hour_start"`
	NormalHourEnd   int    `json:"normal_hour_end"`
}

// Population is the configured set of simulated people (REQ-SIM-001/002):
// a small, deterministic, behaviorally-diverse set, not a large crowd.
type Population struct {
	People []Person `json:"people"`
}

// LoadPopulation reads a population configuration file.
func LoadPopulation(path string) (*Population, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read population file: %w", err)
	}
	var pop Population
	if err := json.Unmarshal(data, &pop); err != nil {
		return nil, fmt.Errorf("parse population file: %w", err)
	}
	return &pop, nil
}

// Find returns the person with the given personID, if present.
func (p *Population) Find(personID string) (Person, bool) {
	for _, person := range p.People {
		if person.PersonID == personID {
			return person, true
		}
	}
	return Person{}, false
}
