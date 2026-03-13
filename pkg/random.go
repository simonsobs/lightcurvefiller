package lightcurvefiller

import (
	"math/rand"
	"time"
)

// Generates a random time (uniformly distributed) between
// the start and end time.
func GenerateRandomTimeBetween(start, end time.Time) time.Time {
	start_timestamp := start.Unix()
	end_timestamp := end.Unix()
	length := end_timestamp - start_timestamp

	random_timestamp := start_timestamp + int64(rand.Float64()*float64(length))
	random_time := time.Unix(random_timestamp, 0)

	return random_time
}

// Generates a random duration (uniformly distributed) between two
// durations.
func GenerateRandomDuration(lower, upper time.Duration) time.Duration {
	lower_seconds := lower.Seconds()
	upper_seconds := upper.Seconds()
	length := upper_seconds - lower_seconds

	random_seconds := lower_seconds + (rand.Float64() * length)
	random_duration := time.Duration(int64(random_seconds)) * time.Second

	return random_duration
}

// Uniformly distributed random float.
func RandomFloatBetween(lower, upper float64) float64 {
	return lower + (upper-lower)*rand.Float64()
}

// Random sign, 50/50 +1 or -1 float.
func RandomSign() float64 {
	if rand.Float64() < 0.5 {
		return -1.0
	} else {
		return 1.0
	}
}
