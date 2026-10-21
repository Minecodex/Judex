package collab

import (
	"github.com/google/uuid"
	"testing"
)

func TestCausalLoops(t *testing.T) {
	a, b, c := uuid.New(), uuid.New(), uuid.New()
	for _, tc := range []struct {
		name  string
		edges []uuid.UUID
		loops int
		tail  int
	}{{"two peers", []uuid.UUID{a, b, a}, 1, 1}, {"three peers", []uuid.UUID{a, b, c, a}, 1, 1}, {"two closures", []uuid.UUID{a, b, a, b, a}, 2, 1}, {"interior closure", []uuid.UUID{a, b, c, b}, 1, 2}, {"no closure", []uuid.UUID{a, b, c}, 0, 3}} {
		t.Run(tc.name, func(t *testing.T) {
			path := []uuid.UUID{}
			loops := 0
			for _, id := range tc.edges {
				next, closed := AdvancePath(path, id)
				path = next
				if closed {
					loops++
				}
			}
			if loops != tc.loops || len(path) != tc.tail {
				t.Fatalf("loops=%d path=%v", loops, path)
			}
		})
	}
}
