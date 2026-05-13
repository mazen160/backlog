package ids

import (
	"math/rand"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"
)

var (
	mu      sync.Mutex
	entropy *ulid.MonotonicEntropy
)

func init() {
	entropy = ulid.Monotonic(rand.New(rand.NewSource(time.Now().UnixNano())), 0)
}

func New() string {
	mu.Lock()
	defer mu.Unlock()
	return ulid.MustNew(ulid.Timestamp(time.Now()), entropy).String()
}

// IsULID reports whether s is structurally a valid ULID string (26 chars,
// Crockford base32). Callers use it to disambiguate user-supplied aliases
// from canonical IDs.
func IsULID(s string) bool {
	_, err := ulid.ParseStrict(s)
	return err == nil
}
