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
		return float64((t^(t>>14))>>0) / 4294967296.0
	}
}

func Jitter(rng func() float64, base, spread int) int {
	return base + int(rng()*float64(spread*2+1)) - spread
}
