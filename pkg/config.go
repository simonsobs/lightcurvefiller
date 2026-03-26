package lightcurvefiller

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

type LightcurveFillerConfig struct {
	Lightcurve      LightcurveConfiguration
	Cutout          CutoutConfiguration
	Lightserve      LightServeConfiguration
	Campaign        ObservingCampaign
	Parquet         ParquetConfiguration
	NumberOfObjects int
	PrintConfig     bool
	LogToFile       string
}

// Read a boolean environment variable. Values of
// 'yes', 'true', 'on' return true, and 'no', 'false', 'off' return false.
func readBoolEnv(key string, missing bool) bool {
	var missing_string string

	if missing {
		missing_string = "yes"
	} else {
		missing_string = "no"
	}

	value := readStringEnv(key, missing_string)
	value = strings.ToLower(value)

	switch value {
	case "yes":
		return true
	case "no":
		return false
	case "true":
		return true
	case "false":
		return false
	case "on":
		return true
	case "off":
		return false
	default:
		log.Panicf("Error interpreting environment variable %s as a boolean (value %s)", key, value)
		return true
	}
}

func readIntEnv(key string, missing int) int {
	value := os.Getenv(key)

	if len(value) > 0 {
		integer, err := strconv.Atoi(value)

		if err != nil {
			log.Panicf("Error interpreting environment variable %s as an integer (value %s)", key, value)
		}

		return integer
	}

	return missing
}

// Read a 64-bit floating point number from an environment variable
func readFloatEnv(key string, missing float64) float64 {
	value := os.Getenv(key)

	if len(value) > 0 {
		number, err := strconv.ParseFloat(value, 64)

		if err != nil {
			log.Panicf("Error interpreting environment variable %s as a float (value %s)", key, value)
		}

		return number
	}

	return missing
}

func readStringEnv(key string, missing string) string {
	value := os.Getenv(key)

	if len(value) > 0 {
		return value
	}

	return missing
}

func readTimeEnv(key string, missing time.Time) time.Time {
	value := os.Getenv(key)

	if len(value) > 0 {
		parsed, err := time.Parse(time.DateOnly, value)

		if err != nil {
			log.Panicf("Error interpreting environment variable %s as a time (value %s)", key, value)
		}

		return parsed
	}

	return missing
}

func readDurationEnv(key string, missing time.Duration) time.Duration {
	value := os.Getenv(key)

	if len(value) > 0 {
		parsed, err := time.ParseDuration(value)

		if err != nil {
			log.Panicf("Error interpreting environment variable %s as a duration (value %s)", key, value)
		}

		return parsed
	}

	return missing
}

// 'Read' the value of the Telescope environment variable from
// the environment. We only support the LAT at the moment.
func readTelescopeEnv(key string, missing string) Telescope {
	value := readStringEnv(key, missing)

	if value != "LAT" {
		log.Panic("The LAT is the only supported telescope, you provided", value)
	}

	return CreateLAT()
}

// 'Read' the value of the beam widths from the environment variable.
// We only support the LAT (30, 40, 90, 150, 220, 280).
func readBeamData(key string, missing string) map[int]float64 {
	value := readStringEnv(key, missing)

	if value != "LAT" {
		log.Panic("The LAT is the only supported telescope for the beam, you provided", value)
	}

	return CreateLATBeams()
}

func ReadLightcurveConfigFromEnvironment() LightcurveConfiguration {
	return LightcurveConfiguration{
		earliest_peak_time:     readTimeEnv("LIGHTCURVE_EARLIEST_PEAK_TIME", time.Now().Add(-time.Duration(time.Hour*8760))),
		latest_peak_time:       readTimeEnv("LIGHTCURVE_LATEST_PEAK_TIME", time.Now()),
		shortest_width:         readDurationEnv("LIGHTCURVE_SHORTEST_WIDTH", time.Hour*24),
		longest_width:          readDurationEnv("LIGHTCURVE_LONGEST_WIDTH", time.Hour*512),
		lowest_base:            readFloatEnv("LIGHTCURVE_LOWEST_BASE", 50.0),
		highest_base:           readFloatEnv("LIGHTCURVE_HIGHEST_BASE", 500.0),
		lowest_flare:           readFloatEnv("LIGHTCURVE_LOWEST_FLARE", 200.0),
		highest_flare:          readFloatEnv("LIGHTCURVE_HIGHEST_FLARE", 3000.0),
		lowest_scatter:         readFloatEnv("LIGHTCURVE_LOWEST_SCATTER", 25.0),
		highest_scatter:        readFloatEnv("LIGHTCURVE_HIGHEST_SCATTER", 200.0),
		lowest_spectral_index:  readFloatEnv("LIGHTCURVE_LOWEST_SPECTRAL_INDEX", -1.0),
		highest_spectral_index: readFloatEnv("LIGHTCURVE_HIGHEST_SPECTRAL_INDEX", 2.0),
		lowest_ra:              readFloatEnv("LIGHTCURVE_LOWEST_RA", -180.0),
		highest_ra:             readFloatEnv("LIGHTCURVE_HIGHEST_RA", 180.0),
		lowest_dec:             readFloatEnv("LIGHTCURVE_LOWEST_DEC", -70.0),
		highest_dec:            readFloatEnv("LIGHTCURVE_HIGHEST_DEC", 25.0),
		pointing:               readFloatEnv("LIGHTCURVE_POINTING", 1.0/60.0),
	}
}

func (l LightcurveConfiguration) Print() {
	fmt.Printf("LIGHTCURVE_EARLIEST_PEAK_TIME=%s\n", l.earliest_peak_time.Format(time.DateOnly))
	fmt.Printf("LIGHTCURVE_LATEST_PEAK_TIME=%s\n", l.latest_peak_time.Format(time.DateOnly))
	fmt.Printf("LIGHTCURVE_SHORTEST_WIDTH=%s\n", l.shortest_width)
	fmt.Printf("LIGHTCURVE_LONGEST_WIDTH=%s\n", l.longest_width)
	fmt.Printf("LIGHTCURVE_LOWEST_BASE=%f\n", l.lowest_base)
	fmt.Printf("LIGHTCURVE_HIGHEST_BASE=%f\n", l.highest_base)
	fmt.Printf("LIGHTCURVE_LOWEST_FLARE=%f\n", l.lowest_flare)
	fmt.Printf("LIGHTCURVE_HIGHEST_FLARE=%f\n", l.highest_flare)
	fmt.Printf("LIGHTCURVE_LOWEST_SCATTER=%f\n", l.lowest_scatter)
	fmt.Printf("LIGHTCURVE_HIGHEST_SCATTER=%f\n", l.highest_scatter)
	fmt.Printf("LIGHTCURVE_LOWEST_SPECTRAL_INDEX=%f\n", l.lowest_spectral_index)
	fmt.Printf("LIGHTCURVE_HIGHEST_SPECTRAL_INDEX=%f\n", l.highest_spectral_index)
	fmt.Printf("LIGHTCURVE_LOWEST_RA=%f\n", l.lowest_ra)
	fmt.Printf("LIGHTCURVE_HIGHEST_RA=%f\n", l.highest_ra)
	fmt.Printf("LIGHTCURVE_LOWEST_DEC=%f\n", l.lowest_dec)
	fmt.Printf("LIGHTCURVE_HIGHEST_DEC=%f\n", l.highest_dec)
	fmt.Printf("LIGHTCURVE_POINTING=%f\n", l.pointing)
}

func ReadCutoutConfigFromEnvironment() CutoutConfiguration {
	return CutoutConfiguration{
		enabled:    readBoolEnv("CUTOUT_ENABLE", true),
		size:       CUTOUT_SIZE,
		pixel_size: readFloatEnv("CUTOUT_PIXEL_SIZE", 0.5/60.0),
		beam_size:  readBeamData("TELESCOPE", "LAT"),
		units:      readStringEnv("CUTOUT_UNITS", "mJy"),
	}
}

func (c CutoutConfiguration) Print() {
	enable_string := "no"
	if c.enabled {
		enable_string = "yes"
	}
	fmt.Printf("CUTOUT_ENABLE=%s\n", enable_string)
	fmt.Printf("CUTOUT_PIXEL_SIZE=%f\n", c.pixel_size)
	fmt.Printf("CUTOUT_UNITS=%s\n", c.units)
}

func ReadLightserveConfigFromEnvironment() LightServeConfiguration {
	return LightServeConfiguration{
		host:              readStringEnv("LIGHTSERVE_HOST", "http://localhost:8001"),
		batch_size:        readIntEnv("LIGHTSERVE_BATCH_SIZE", 2048),
		use_bearer:        readBoolEnv("LIGHTSERVE_USE_BEARER", false),
		bearer:            readStringEnv("LIGHTSERVE_BEARER_TOKEN", ""),
		allow_self_signed: readBoolEnv("LIGHTSERVE_ALLOW_SELF_SIGNED", false),
		enable:            readBoolEnv("LIGHTSERVE_ENABLE", true),
	}
}

func (s LightServeConfiguration) Print() {
	bearer_string := "no"
	if s.use_bearer {
		bearer_string = "yes"
	}

	self_signed_string := "no"
	if s.allow_self_signed {
		self_signed_string = "yes"
	}

	enable_string := "no"
	if s.enable {
		enable_string = "yes"
	}

	fmt.Printf("LIGHTSERVE_HOST=%s\n", s.host)
	fmt.Printf("LIGHTSERVE_BATCH_SIZE=%d\n", s.batch_size)
	fmt.Printf("LIGHTSERVE_USE_BEARER=%s\n", bearer_string)
	fmt.Printf("LIGHTSERVE_BEARER_TOKEN=%s\n", s.bearer)
	fmt.Printf("LIGHTSERVE_ALLOW_SELF_SIGNED=%s\n", self_signed_string)
	fmt.Printf("LIGHTSERVE_ENABLE=%s\n", enable_string)
}

func ReadParquetConfiguration() ParquetConfiguration {
	return ParquetConfiguration{
		enable:    readBoolEnv("PARQUET_ENABLE", false),
		base_path: readStringEnv("PARQUET_BASE_PATH", "."),
		compress:  readBoolEnv("PARQUET_COMPRESS", true),
	}
}

func (p ParquetConfiguration) Print() {
	enable_string := "no"
	if p.enable {
		enable_string = "yes"
	}
	compress_string := "no"
	if p.compress {
		compress_string = "yes"
	}

	fmt.Printf("PARQUET_ENABLE=%s\n", enable_string)
	fmt.Printf("PARQUET_BASE_PATH=%s\n", p.base_path)
	fmt.Printf("PARQUET_COMPRESS=%s\n", compress_string)
}

func ReadObservingCampaignConfigFromEnvironment() ObservingCampaign {
	return ObservingCampaign{
		Start:     readTimeEnv("OBSERVATION_START", time.Now().Add(-time.Duration(time.Hour*8760))),
		End:       readTimeEnv("OBSERVATION_END", time.Now()),
		Interval:  readDurationEnv("OBSERVATION_INTERVAL", time.Hour*24),
		Jitter:    readDurationEnv("OBSERVATION_JITTER", time.Minute*15),
		Telescope: readTelescopeEnv("TELESCOPE", "LAT"),
	}

}

func (c ObservingCampaign) Print() {
	fmt.Printf("OBSERVATION_START=%s\n", c.Start.Format(time.DateOnly))
	fmt.Printf("OBSERVATION_END=%s\n", c.End.Format(time.DateOnly))
	fmt.Printf("OBSERVATION_INTERVAL=%s\n", c.Interval)
	fmt.Printf("OBSERVATION_JITTER=%s\n", c.Jitter)
	fmt.Printf("TELESCOPE=%s\n", c.Telescope.Name)
}

// Read the entire configuration from the environment. There
// are no required parameters. By default we print the entire
// configuration after reading it.
func ReadConfigFromEnvironment() LightcurveFillerConfig {
	config := LightcurveFillerConfig{
		Lightcurve:      ReadLightcurveConfigFromEnvironment(),
		Cutout:          ReadCutoutConfigFromEnvironment(),
		Lightserve:      ReadLightserveConfigFromEnvironment(),
		Campaign:        ReadObservingCampaignConfigFromEnvironment(),
		Parquet:         ReadParquetConfiguration(),
		NumberOfObjects: readIntEnv("NUMBER_OF_OBJECTS", 100),
		PrintConfig:     readBoolEnv("PRINT_CONFIG", true),
		LogToFile:       readStringEnv("LOG_FILE", ""),
	}

	if config.PrintConfig {
		config.Lightcurve.Print()
		config.Cutout.Print()
		config.Lightserve.Print()
		config.Campaign.Print()
		config.Parquet.Print()
		fmt.Printf("NUMBER_OF_OBJECTS=%d\n", config.NumberOfObjects)
		fmt.Printf("PRINT_CONFIG=%s\n", "yes")
		fmt.Printf("LOG_FILE=%s\n", config.LogToFile)
	}

	return config
}

// Run the entire observing campaign, outputting to the API.
func (c LightcurveFillerConfig) Run() {
	if c.LogToFile != "" {
		file, err := os.OpenFile(c.LogToFile, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0666)
		if err != nil {
			fmt.Printf("Error opening file: %v\n", err)
		}
		fmt.Println("Logging set up, directed to file ", c.LogToFile)
		log.SetOutput(file)
		defer file.Close()
	}

	lightcurves := make([]Lightcurve, c.NumberOfObjects)

	for index := range lightcurves {
		lightcurves[index] = NewLightcurve(c.Lightcurve)
	}

	// Upload necessary metadata to lightserve
	if c.Lightserve.enable {
		before_upload_instruments := time.Now()
		c.Lightserve.UploadInstruments(c.Campaign.Telescope)
		log.Printf(
			"Successfully uploaded telescope information, took %d ms\n",
			time.Since(before_upload_instruments).Milliseconds(),
		)
		before_upload_sources := time.Now()
		c.Lightserve.UploadSources(lightcurves)
		log.Printf(
			"Successfully uploaded source metadata, took %d ms\n",
			time.Since(before_upload_sources).Milliseconds(),
		)
	}

	number_of_observing_periods := c.Campaign.End.Sub(c.Campaign.Start) / c.Campaign.Interval

	for observing_period := range number_of_observing_periods {
		internal_time := c.Campaign.Start.Add(c.Campaign.Interval * observing_period)

		before_generation := time.Now()
		observations := c.Campaign.ObserveLightcurvesAt(lightcurves, internal_time)
		time_to_generate := time.Since(before_generation)

		log.Printf(
			"Generated %d observations for time %s (took %d ms)\n",
			len(observations),
			internal_time.Format(time.DateOnly),
			time_to_generate.Milliseconds(),
		)

		var cutouts []Cutout = nil
		if c.Cutout.enabled {
			before_cutout := time.Now()
			cutouts = make([]Cutout, len(observations))
			for index := range cutouts {
				cutouts[index] = c.Cutout.GenerateCutout(observations[index])
			}
			time_to_cutout := time.Since(before_cutout)
			log.Printf(
				"Generated %d cutouts for time %s (took %d ms)\n",
				len(cutouts),
				internal_time.Format(time.DateOnly),
				time_to_cutout.Milliseconds(),
			)
		}

		if c.Parquet.enable {
			before_parquet := time.Now()
			err := c.Parquet.WriteData(observations, internal_time)
			time_to_parquet := time.Since(before_parquet)
			log.Printf(
				"Wrote parquet data to disk for time %s (took %d ms)\n",
				internal_time.Format(time.DateOnly),
				time_to_parquet.Milliseconds(),
			)

			if err != nil {
				log.Printf("Unable to write parquet file\n")
			}
		}

		if c.Lightserve.enable {
			before_upload := time.Now()
			c.Lightserve.UploadData(observations, cutouts)
			time_to_upload := time.Since(before_upload)
			log.Printf(
				"Uploaded %d observations for time %s (took %d ms)\n",
				len(observations),
				internal_time.Format(time.DateOnly),
				time_to_upload.Milliseconds(),
			)
		}
	}
}
