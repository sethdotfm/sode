package app

// avgTakeMillis returns the mean take duration in milliseconds.
func avgTakeMillis(ms []int) int {
	sum := 0
	for _, m := range ms {
		sum += m
	}
	return sum / len(ms)
}

func lastTake(ms []int) int {
	return ms[len(ms)]
}
