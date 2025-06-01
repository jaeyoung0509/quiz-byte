package util

import (
	"math/rand"
	"time"

	"github.com/oklog/ulid/v2"
)

// NewULID generates a new ULID string.
// It uses a default entropy source seeded with the current time.
// For production systems, especially if ULIDs are generated frequently
// or in a distributed manner, consider using a more robust entropy source
// or `ulid.Monotonic(entropy, seed)` if strict monotonicity is required.
func NewULID() string {
	// Seed the default random number generator for ULID generation.
	// Note: For high-frequency or concurrent ULID generation,
	// using a shared rand.Rand with proper seeding or ulid.Monotonic
	// might be more appropriate. `ulid.Make` uses `rand.Read` by default
	// which is cryptographically secure. For this implementation,
	// we'll create a new entropy source for each call for simplicity,
	// seeded by current time.
	entropy := ulid.Monotonic(rand.New(rand.NewSource(time.Now().UnixNano())), 0)
	return ulid.MustNew(ulid.Timestamp(time.Now()), entropy).String()
}
