package simulator

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"sort"
	"time"

	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/contracts"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/evidence"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/idgen"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/modeltime"
)

// ScenarioStep is one presentation within a "fixed" scenario: person_id
// presents at model_start + offset_seconds.
type ScenarioStep struct {
	PersonID      string  `json:"person_id"`
	OffsetSeconds float64 `json:"offset_seconds"`
}

// ScenarioDef is a named, data-driven scenario (design-specification.md
// section 3.5): participating simulated persons, initial model time, and
// (via Kind) which of three small, deliberately non-general execution
// strategies to use. This is not a general scenario DSL — it is exactly
// the three shapes the eight required scenarios need:
//
//   - "fixed": an explicit, ordered list of (person, offset) presentations.
//   - "population_sample": a seeded random subset/order/spacing of the
//     configured Population, for "normal-day"-style variety.
//   - "burst": one person presenting Count times at IntervalMillis spacing,
//     for latency/stress evaluation.
type ScenarioDef struct {
	Description string `json:"description"`
	ModelStart  string `json:"model_start"`
	ReaderID    string `json:"reader_id"`
	DoorID      string `json:"door_id"`
	Direction   string `json:"direction"`
	Kind        string `json:"kind"`

	// kind == "fixed"
	Steps []ScenarioStep `json:"steps,omitempty"`

	// kind == "population_sample"
	SampleSize     int     `json:"sample_size,omitempty"`
	SpacingSeconds float64 `json:"spacing_seconds,omitempty"`
	JitterSeconds  float64 `json:"jitter_seconds,omitempty"`

	// kind == "burst"
	PersonID              string `json:"person_id,omitempty"`
	DefaultCount          int    `json:"default_count,omitempty"`
	DefaultIntervalMillis int    `json:"default_interval_ms,omitempty"`
}

// ScenarioSet is the full named-scenario configuration file.
type ScenarioSet struct {
	Scenarios map[string]ScenarioDef `json:"scenarios"`
}

// LoadScenarios reads a scenario configuration file.
func LoadScenarios(path string) (*ScenarioSet, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read scenarios file: %w", err)
	}
	var set ScenarioSet
	if err := json.Unmarshal(data, &set); err != nil {
		return nil, fmt.Errorf("parse scenarios file: %w", err)
	}
	return &set, nil
}

// SetPopulation attaches the configured simulated-person population.
// Scenario kinds that reference a person_id resolve it against this set.
func (s *Server) SetPopulation(pop *Population) { s.population = pop }

// SetScenarios attaches the named scenario definitions /scenario/run can
// invoke. Both this and SetPopulation are optional: a Server with neither
// set behaves exactly as before this feature existed (/scenario/present
// and everything else is unaffected).
func (s *Server) SetScenarios(set *ScenarioSet) { s.scenarios = set }

// SetScenarioRunLog attaches the file scenario-run reproducibility
// metadata (scenario name, seed, model time, participating cards) is
// appended to. This is deliberately NOT the Historical Evidence Store
// (ADR-005: Event/Data Service is its sole writer) — it is a separate,
// Simulator-owned experiment-reproducibility log, matching the "retained
// records... through experiment metadata/files instead" latitude.
func (s *Server) SetScenarioRunLog(store *evidence.Store) { s.scenarioRunLog = store }

// RunScenario executes the named scenario synchronously: every
// presentation it specifies is enqueued (and, for crossing-enabled doors,
// its crossing watched) before RunScenario returns. seed makes the
// "population_sample" and "burst" kinds reproducible — a "fixed" scenario
// has no randomness and ignores it. count/intervalMillis override a
// "burst" scenario's configured defaults when nonzero.
func (s *Server) RunScenario(name string, seed int64, count, intervalMillis int) (contracts.ScenarioRunResult, error) {
	if s.scenarios == nil {
		return contracts.ScenarioRunResult{}, fmt.Errorf("no scenarios configured")
	}
	def, ok := s.scenarios.Scenarios[name]
	if !ok {
		return contracts.ScenarioRunResult{}, fmt.Errorf("unknown scenario %q", name)
	}
	modelStart, err := modeltime.Parse(def.ModelStart)
	if err != nil {
		return contracts.ScenarioRunResult{}, fmt.Errorf("scenario %q: invalid model_start: %w", name, err)
	}
	s.clock.Set(modelStart)
	rng := rand.New(rand.NewSource(seed))

	var cardIDs []string
	switch def.Kind {
	case "fixed":
		for _, step := range def.Steps {
			person, ok := s.resolvePerson(step.PersonID)
			if !ok {
				return contracts.ScenarioRunResult{}, fmt.Errorf("scenario %q: unknown person_id %q", name, step.PersonID)
			}
			s.clock.Set(modelStart.Add(time.Duration(step.OffsetSeconds * float64(time.Second))))
			s.enqueuePresentation(def.ReaderID, person.CardID, def.DoorID, def.Direction, person.PersonID)
			cardIDs = append(cardIDs, person.CardID)
		}

	case "population_sample":
		if s.population == nil {
			return contracts.ScenarioRunResult{}, fmt.Errorf("scenario %q: no population configured", name)
		}
		order := rng.Perm(len(s.population.People))
		sampleSize := def.SampleSize
		if sampleSize <= 0 || sampleSize > len(order) {
			sampleSize = len(order)
		}
		type scheduled struct {
			person Person
			offset float64
		}
		var picks []scheduled
		for i, idx := range order[:sampleSize] {
			jitter := 0.0
			if def.JitterSeconds > 0 {
				jitter = (rng.Float64()*2 - 1) * def.JitterSeconds
			}
			offset := float64(i)*def.SpacingSeconds + jitter
			if offset < 0 {
				offset = 0
			}
			picks = append(picks, scheduled{person: s.population.People[idx], offset: offset})
		}
		sort.Slice(picks, func(i, j int) bool { return picks[i].offset < picks[j].offset })
		for _, p := range picks {
			s.clock.Set(modelStart.Add(time.Duration(p.offset * float64(time.Second))))
			s.enqueuePresentation(def.ReaderID, p.person.CardID, def.DoorID, def.Direction, p.person.PersonID)
			cardIDs = append(cardIDs, p.person.CardID)
		}

	case "burst":
		person, ok := s.resolvePerson(def.PersonID)
		if !ok {
			return contracts.ScenarioRunResult{}, fmt.Errorf("scenario %q: unknown person_id %q", name, def.PersonID)
		}
		n := count
		if n <= 0 {
			n = def.DefaultCount
		}
		baseInterval := intervalMillis
		if baseInterval <= 0 {
			baseInterval = def.DefaultIntervalMillis
		}
		for i := 0; i < n; i++ {
			s.clock.Set(modelStart.Add(time.Duration(i) * time.Millisecond))
			s.enqueuePresentation(def.ReaderID, person.CardID, def.DoorID, def.Direction, person.PersonID)
			cardIDs = append(cardIDs, person.CardID)
			if i == n-1 {
				break
			}
			jitterFrac := 1 + (rng.Float64()*0.4 - 0.2) // +/-20%, seeded
			wait := time.Duration(float64(baseInterval)*jitterFrac) * time.Millisecond
			time.Sleep(wait)
		}

	default:
		return contracts.ScenarioRunResult{}, fmt.Errorf("scenario %q: unknown kind %q", name, def.Kind)
	}

	result := contracts.ScenarioRunResult{
		RunID:         idgen.New(),
		Scenario:      name,
		Seed:          seed,
		ModelStart:    def.ModelStart,
		StepsExecuted: len(cardIDs),
		CardIDs:       cardIDs,
	}
	s.logScenarioRun(result)
	return result, nil
}

// resolvePerson looks personID up in the configured Population.
func (s *Server) resolvePerson(personID string) (Person, bool) {
	if s.population == nil {
		return Person{}, false
	}
	return s.population.Find(personID)
}

func (s *Server) logScenarioRun(result contracts.ScenarioRunResult) {
	if s.scenarioRunLog == nil {
		return
	}
	if err := s.scenarioRunLog.Append("scenario_run", result); err != nil {
		fmt.Printf("simulator: append scenario run log: %v\n", err)
	}
}
