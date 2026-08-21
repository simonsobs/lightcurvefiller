// Benchmarking tools to calculate means, standard deviations, etc. from a list
// of time durations, and write out time duration files.
package lightcurvefiller

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"time"
)

type BenchmarkConfiguration struct {
	save      bool
	directory string
}

type BenchmarkResult struct {
	Minimum time.Duration
	Maximum time.Duration
	Mean    time.Duration
	Std     time.Duration
}

// Return a string describing information about the benchmarks
func (r BenchmarkResult) String() string {
	return fmt.Sprintf(
		"Minimum: %d ms, Maximum: %d ms, Mean: %d ms, Standard Deviation: %d ms",
		r.Minimum.Milliseconds(),
		r.Maximum.Milliseconds(),
		r.Mean.Milliseconds(),
		r.Std.Milliseconds(),
	)
}

// Calculate the benchmark result from a list of duratioins
func calculateBenchmarkResult(data []time.Duration) BenchmarkResult {
	total := 0.0
	total_squared := 0.0
	min := time.Duration(math.MaxInt64)
	max := time.Duration(0)
	n := len(data)

	for _, value := range data {
		if value < min {
			min = value
		}
		if value > max {
			max = value
		}
		value_seconds := value.Abs().Seconds()
		total += value_seconds
		total_squared += value_seconds * value_seconds
	}

	mean := total / float64(n)
	variance := total_squared/float64(n) - mean*mean
	standard_deviation := math.Sqrt(variance)

	return BenchmarkResult{
		Minimum: min,
		Maximum: max,
		Mean:    time.Duration(mean * float64(time.Second)),
		Std:     time.Duration(standard_deviation * float64(time.Second)),
	}
}

func (c BenchmarkConfiguration) SaveWriteBenchmarkResult(
	config LightServeConfiguration,
	timings []time.Duration,
	filename string,
	total_time time.Duration,
) (BenchmarkResult, error) {
	benchmark_result := calculateBenchmarkResult(timings)

	err := error(nil)

	if c.save {
		var result map[string]any

		result["units"] = "ns"
		result["metadata"] = config
		result["summary"] = benchmark_result
		result["timings"] = timings
		result["total_wallclock"] = total_time

		serialized, err := json.Marshal(result)

		if err != nil {
			log.Fatalln("Unable to marshal data for the write benchmark save")
		}

		file_path := fmt.Sprintf("%s/%s", c.directory, filename)

		err = os.WriteFile(file_path, serialized, os.ModePerm)
	}

	return benchmark_result, err
}

func (c BenchmarkConfiguration) SaveReadBenchmarkResult(
	config ReaderConfiguration,
	timings []time.Duration,
	filename string,
	total_time time.Duration,
) (BenchmarkResult, error) {
	benchmark_result := calculateBenchmarkResult(timings)

	err := error(nil)

	if c.save {
		var result map[string]any

		result["units"] = "ns"
		result["metadata"] = config
		result["summary"] = benchmark_result
		result["timings"] = timings
		result["total_wallclock"] = total_time

		serialized, err := json.Marshal(result)

		if err != nil {
			log.Fatalln("Unabel to marshal data for the read benchmark save")
		}

		file_path := fmt.Sprintf(filename, serialized, os.ModePerm)

		err = os.WriteFile(file_path, serialized, os.ModePerm)
	}

	return benchmark_result, err
}
