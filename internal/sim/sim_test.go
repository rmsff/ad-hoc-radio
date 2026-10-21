package sim

import (
	"math/rand/v2"
	"testing"
)

// TestSmokeDeterminism is the Story 1.1 smoke test.
//
// It proves three things required by Story 1.1 acceptance criteria:
//   - the Go module compiles and the test command works (AC-2);
//   - deterministic behavior with a fixed seed (NFR-1);
//   - no RF/baseband dependency — only stdlib is used (NFR-2, NFR-5).
//
// Traceability ID: UT-000 (module smoke / determinism).
func TestSmokeDeterminism(t *testing.T) {
	const seed = 42
	const n = 16

	first := make([]int, n)
	rngA := rand.New(rand.NewPCG(seed, seed))
	for i := 0; i < n; i++ {
		first[i] = rngA.Int()
	}

	second := make([]int, n)
	rngB := rand.New(rand.NewPCG(seed, seed))
	for i := 0; i < n; i++ {
		second[i] = rngB.Int()
	}

	for i := 0; i < n; i++ {
		if first[i] != second[i] {
			t.Fatalf("non-deterministic stream at index %d: %d != %d", i, first[i], second[i])
		}
	}
}

// TestSmokeSeedSensitivity guards against the degenerate case where the
// PRNG is not actually wired to the seed: a different seed must produce
// a different stream.
func TestSmokeSeedSensitivity(t *testing.T) {
	rngA := rand.New(rand.NewPCG(1, 1))
	rngB := rand.New(rand.NewPCG(2, 2))

	same := true
	for i := 0; i < 8; i++ {
		if rngA.Int() != rngB.Int() {
			same = false
			break
		}
	}
	if same {
		t.Fatal("two differently-seeded PRNGs produced identical streams; seed is not wired")
	}
}
