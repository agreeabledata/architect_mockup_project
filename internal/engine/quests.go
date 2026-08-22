package engine

var blurbs = map[string]string{
	"pier4":       "Coins on the harbour roofs. Grab them, stamp the far door.",
	"clocktower":  "Clock-route coins. One-way planks. Watch the drop.",
	"nightmarket": "Lantern-row coins on the high ledge. Stamp the lower door.",
	"roofduel":    "Stack coins until the thug shows — only when you can beat him.",
}

func SeedBoard(seed uint32) []Quest {
	rng := Mulberry32(seed)
	out := make([]Quest, 0, len(LevelOrder))
	for i, id := range LevelOrder {
		lvl := GetLevel(id)
		reward := Jitter(rng, lvl.RewardKrone, 4)
		if reward < 20 {
			reward = 20
		}
		foe := ""
		hasFoe := len(lvl.Encounters) > 0
		if hasFoe {
			foe = lvl.Encounters[0].Name
		}
		out = append(out, Quest{
			ID:          "q-" + id + "-" + itoa(int(seed)) + "-" + itoa(i),
			LevelID:     id,
			Title:       lvl.Name,
			District:    lvl.District,
			Blurb:       blurbs[id],
			RewardKrone: reward,
			TravelDays:  lvl.TravelDays,
			HasFoe:      hasFoe,
			FoeName:     foe,
			Status:      QuestOpen,
		})
	}
	return out
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
