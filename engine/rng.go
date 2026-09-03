package engine

// Mulberry32 seeded PRNG. Same seed → same stream.
func Mulberry32(seed uint32) func() float64 {
	a := seed
	if a == 0 {
		a = 0x9e3779b9
	}
	return func() float64 {
		a += 0x6d2b79f5
		t := a
		t = (t ^ (t >> 15)) * (1 | t)
		t = (t + ((t ^ (t >> 7)) * (61 | t))) ^ t
		return float64((t ^ (t >> 14))) / 4294967296.0
	}
}

// randInt returns an integer in [0, n). n must be > 0.
func randInt(rng func() float64, n int) int {
	if n <= 0 {
		return 0
	}
	i := int(rng() * float64(n))
	if i < 0 {
		return 0
	}
	if i >= n {
		return n - 1
	}
	return i
}

func shuffle(rng func() float64, n int) []int {
	idx := make([]int, n)
	for i := range idx {
		idx[i] = i
	}
	for i := n - 1; i > 0; i-- {
		j := randInt(rng, i+1)
		idx[i], idx[j] = idx[j], idx[i]
	}
	return idx
}
