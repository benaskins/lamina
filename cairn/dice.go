package cairn

import (
	"fmt"
	"math/rand"
)

// roll rolls a single die with the given number of sides.
func roll(sides int) int {
	return rand.Intn(sides) + 1
}

// rollNdS rolls n dice each with s sides and returns the total.
func rollNdS(n, s int) int {
	total := 0
	for range n {
		total += roll(s)
	}
	return total
}

// parseDie returns the number of sides for a die string like "d6", "d8", etc.
func parseDie(die string) int {
	var sides int
	fmt.Sscanf(die, "d%d", &sides)
	if sides == 0 {
		return 6 // default
	}
	return sides
}
