//go:build !race

package app

// Race-detector runs are several times slower; scales test deadlines.
const raceSlowdown = 1
