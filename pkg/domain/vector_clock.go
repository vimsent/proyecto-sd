package domain

import "github.com/tu-usuario/proyecto-sd/api/reviews"

// IncrementClock aumenta el contador del nodo actual
func IncrementClock(clock *reviews.VectorClock, nodeID string) {
	if clock.Versions == nil {
		clock.Versions = make(map[string]int64)
	}
	clock.Versions[nodeID]++
}

// MergeClocks combina relojes (max de cada entrada) para convergencia
func MergeClocks(c1, c2 *reviews.VectorClock) *reviews.VectorClock {
	merged := &reviews.VectorClock{Versions: make(map[string]int64)}
	for k, v := range c1.Versions {
		merged.Versions[k] = v
	}
	for k, v := range c2.Versions {
		if v > merged.Versions[k] {
			merged.Versions[k] = v
		}
	}
	return merged
}

// IsAfter verifica si c1 es "posterior" o igual a c2 (para Monotonic Reads)
func IsAfterOrEqual(c1, c2 *reviews.VectorClock) bool {
	for k, v2 := range c2.Versions {
		if c1.Versions[k] < v2 {
			return false
		}
	}
	return true
}