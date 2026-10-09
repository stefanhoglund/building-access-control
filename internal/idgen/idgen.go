// Package idgen generates random identifiers (event_id, decision_id,
// command_id) using only the standard library, avoiding a UUID dependency
// for this slice (CLAUDE.md dependency policy: prefer the standard
// library where practical).
package idgen

import (
	"crypto/rand"
	"fmt"
)

// New returns a random 128-bit identifier formatted as a UUID-v4-shaped
// string. It is not RFC 4122 compliant (no version/variant bits are set)
// because this slice only needs a stable, collision-resistant identifier,
// not interoperability with external UUID consumers.
func New() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(fmt.Sprintf("idgen: read random bytes: %v", err))
	}
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
