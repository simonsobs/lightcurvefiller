// Handles reading (and also benchmarking) results from the database once
// it has been filled. Reads entire source lightcurves and binned versions
// so we can understand both internal and external latency.
package lightcurvefiller

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type ReaderConfiguration struct {
	enable            bool      // Whether to attempt reading at all
	host              string    // Hostname (including port) of lightserve server
	use_bearer        bool      // Whether to use the Bearer token
	bearer            string    // Bearer token (only used if use_bearer)
	allow_self_signed bool      // Whether to allow self-signed certificates
	sources_to_read   int       // Total number of sources to read back
	read_frequency    bool      // Whether to read frequency-binned lightcurves
	read_all          bool      // Whether to read the un-binned lightcurve
	read_summary      bool      // Whether to read the source summary
	start_time        time.Time // Start date for querying
	end_time          time.Time // End date for querying
}

// JSON Response from /sources; only partial information is required
type sourceResponse struct {
	source_id uuid.UUID // SourceID
}

type ReadBenchmarkResult struct {
	Minimum time.Duration
	Maximum time.Duration
	Mean    time.Duration
	Std     time.Duration
}

// Return a string describing information about the benchmarks
func (r ReadBenchmarkResult) String() string {
	return fmt.Sprintf(
		"Minimum: %d ms, Maximum: %d ms, Mean: %d ms, Standard Deviation: %d ms",
		r.Minimum.Milliseconds(),
		r.Maximum.Milliseconds(),
		r.Mean.Milliseconds(),
		r.Std.Milliseconds(),
	)
}

// Calculate the benchmark result from a list of duratioins
func calculateBenchmarkResult(data []time.Duration) ReadBenchmarkResult {
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

	return ReadBenchmarkResult{
		Minimum: min,
		Maximum: max,
		Mean:    time.Duration(mean * float64(time.Second)),
		Std:     time.Duration(standard_deviation * float64(time.Second)),
	}
}

// Read all the SourceIDs from the lightcurve egress server.
func (c ReaderConfiguration) ReadSourceIDs() []sourceResponse {
	url := fmt.Sprintf("%s/sources", c.host)
	client := c.GetClient()

	response, err := client.Get(url)

	if err != nil {
		log.Panic("Could not get", url)
	}

	var sources = make([]sourceResponse, 1)

	err = json.NewDecoder(response.Body).Decode(&sources)

	if err != nil {
		log.Panic("Cound not unmarshal the list of sources")
	}

	return sources
}

// Return the time taken (round-trip) to get a response.
func timedHTTPRequest(endpoint string, host string, client *http.Client) time.Duration {
	url := fmt.Sprintf("%s/%s", endpoint, host)

	start := time.Now()
	_, err := client.Get(url)
	end := time.Now()

	if err != nil {
		log.Panic("Could not recieve HTTP GET request at", url)
	}

	return end.Sub(start)
}

// Read the frequency-binned lightcurves for the number of lightcurves requested.
func (c ReaderConfiguration) readFrequencyBinnedLightcurves(sources []sourceResponse) []time.Duration {
	number_read := 0
	timings := make([]time.Duration, c.sources_to_read)

	// In theory we could allow this to be changed from the default weekly binned case
	parameters := fmt.Sprintf(
		"start_time=%s&end_time=%s&selection_strategy=frequency",
		c.start_time.Format(time.RFC3339),
		c.end_time.Format(time.RFC3339),
	)

	client := c.GetClient()

	for number_read < c.sources_to_read {
		source := sources[RandomIntegerBetween(0, c.sources_to_read)]
		endpoint := fmt.Sprintf("lightcurves/%s/binned?%s", source.source_id.String(), parameters)

		timings[number_read] = timedHTTPRequest(endpoint, c.host, client)

		number_read++
	}

	return timings
}

// Read the unbinned data for the number of lightcurves requested.
func (c ReaderConfiguration) readUnBinnedLightcurves(sources []sourceResponse) []time.Duration {
	number_read := 0
	timings := make([]time.Duration, c.sources_to_read)

	client := c.GetClient()

	for number_read < c.sources_to_read {
		source := sources[RandomIntegerBetween(0, c.sources_to_read)]
		endpoint := fmt.Sprintf("lightcurves/%s/unbinned", source.source_id.String())

		timings[number_read] = timedHTTPRequest(endpoint, c.host, client)

		number_read++
	}

	return timings
}

// Read the source 'summary' information for the number of lightcurves requestsed
func (c ReaderConfiguration) readSourceSummaries(sources []sourceResponse) []time.Duration {
	number_read := 0
	timings := make([]time.Duration, c.sources_to_read)

	client := c.GetClient()

	for number_read < c.sources_to_read {
		source := sources[RandomIntegerBetween(0, c.sources_to_read)]
		endpoint := fmt.Sprintf("sources/%s/summary", source.source_id.String())

		timings[number_read] = timedHTTPRequest(endpoint, c.host, client)

		number_read++
	}

	return timings
}

// Perform the 'read' trials and return data
func (c ReaderConfiguration) ReadData() {
	sources := c.ReadSourceIDs()

	if c.read_frequency {
		data := c.readFrequencyBinnedLightcurves(sources)
		benchmark := calculateBenchmarkResult(data)
		log.Printf("Read all result: %s\n", benchmark.String())
	}

	if c.read_all {
		data := c.readUnBinnedLightcurves(sources)
		benchmark := calculateBenchmarkResult(data)
		log.Printf("Read all result: %s\n", benchmark.String())
	}

	if c.read_summary {
		data := c.readSourceSummaries(sources)
		benchmark := calculateBenchmarkResult(data)
		log.Printf("Read all result: %s\n", benchmark.String())
	}

	return
}
