// The lightcurvefiller package allows users to generate synthetic lightcurve data
// for 'flaring' objects. This is used to test the scaliability and performance of
// the lightserve system and lightcurvedb backends.
//
// [pkg/lightcurve] provides the core lightcurve generation functionality
// [pkg/random] provides some utility functions for random number generation
// [pkg/config] handles environment variables
// [pkg/engine] provides the tools for generating new data points and configuring
//
//	the platform variables
//
// [pkg/api] handles interactions with the lightserve API.
package lightcurvefiller

import (
	"log"
	"math"
	"time"

	"github.com/google/uuid"
)

// Configuration for the lightcurve generation system, sets up
// basic parameters.
type LightcurveConfiguration struct {
	earliest_peak_time time.Time // Earliest possible flare time
	latest_peak_time   time.Time // Latest possible flare time

	shortest_width time.Duration // Minimum width for a single flare
	longest_width  time.Duration // Maximum width for a single flare

	lowest_base  float64 // Minimum value for 'base' flux (mJy)
	highest_base float64 // Maximum value for 'base' flux (mJy)

	lowest_flare  float64 // Minimum value for 'flare' flux at 90 GHz (mJy)
	highest_flare float64 // Maximum value for 'flare' flux at 90 GHz (mJy)

	lowest_scatter  float64 // Minimum value for random variation in `base` (mJy)
	highest_scatter float64 // Maximum value for random variation in `base` (mJy)

	lowest_spectral_index  float64 // Lowest spectral index to use when scaling flux to 90 GHz
	highest_spectral_index float64 // Highest spectral index to use when scaling flux to 90 GHz

	lowest_ra   float64 // Lowest value of RA to use as sky position (-180 < RA < 180 is convention; deg)
	highest_ra  float64 // Highest value of RA to use as sky position (deg)
	lowest_dec  float64 // Lowest value of Dec to use as sky position (-90 < Dec < 90 is convention; deg)
	highest_dec float64 // Highest value of Dec to use as sky position (deg)
	pointing    float64 // Pointing variance to use (deg)
}

type Lightcurve struct {
	SourceID       uuid.UUID `json:"source_id"`
	peak           time.Time
	width          time.Duration
	base           float64
	flare          float64
	scatter        float64
	spectral_index float64
	Ra             float64 `json:"ra"`
	Dec            float64 `json:"dec"`
	pointing       float64
	Name           string `json:"name"`
}

type LightcurveDatapoint struct {
	Frequency      int       `json:"frequency"`
	Module         string    `json:"module"`
	SourceID       uuid.UUID `json:"source_id"`
	Time           time.Time `json:"time"`
	Ra             float64   `json:"ra"`
	Dec            float64   `json:"dec"`
	RaUncertainty  float64   `json:"ra_uncertainty"`
	DecUncertainty float64   `json:"dec_uncertainty"`
	Flux           float64   `json:"flux"`
	FluxErr        float64   `json:"flux_err"`
}

func NewLightcurve(configuration LightcurveConfiguration) Lightcurve {
	uuid, err := uuid.NewV7()

	if err != nil {
		log.Panic("Error in UUID generation")
	}

	return Lightcurve{
		SourceID:       uuid,
		peak:           GenerateRandomTimeBetween(configuration.earliest_peak_time, configuration.latest_peak_time),
		width:          GenerateRandomDuration(configuration.shortest_width, configuration.longest_width),
		base:           RandomFloatBetween(configuration.lowest_base, configuration.highest_base),
		flare:          RandomFloatBetween(configuration.lowest_flare, configuration.highest_flare),
		scatter:        RandomFloatBetween(configuration.lowest_scatter, configuration.highest_scatter),
		spectral_index: RandomFloatBetween(configuration.lowest_spectral_index, configuration.highest_spectral_index),
		Ra:             RandomFloatBetween(configuration.lowest_ra, configuration.highest_ra),
		Dec:            RandomFloatBetween(configuration.lowest_dec, configuration.highest_dec),
		pointing:       configuration.pointing,
		Name:           uuid.String(),
	}
}

func (l Lightcurve) GenerateDataPoint(t time.Time, m Module) [2]LightcurveDatapoint {
	// Generates a new datapoint from an individua lightcurve. Includes
	// random variability and the possibiltiy for the flare.
	flux := l.base + RandomFloatBetween(0.5*l.scatter, l.scatter)

	time_to_flare := t.Sub(l.peak)
	ratio_to_flare := time_to_flare.Seconds() / l.width.Seconds()

	flare_flux_base := l.flare * math.Exp(-(ratio_to_flare * ratio_to_flare))

	freq_offset_a := math.Pow(float64(m.Frequencies[0])/90.0, l.spectral_index)
	freq_offset_b := math.Pow(float64(m.Frequencies[1])/90.0, l.spectral_index)

	flare_flux_a := flare_flux_base * freq_offset_a
	flare_flux_b := flare_flux_base * freq_offset_b

	return [2]LightcurveDatapoint{
		{
			SourceID:       l.SourceID,
			Time:           t,
			Frequency:      m.Frequencies[0],
			Module:         m.Identifier,
			Ra:             l.Ra,
			Dec:            l.Dec,
			RaUncertainty:  RandomFloatBetween(0.5*l.pointing, l.pointing),
			DecUncertainty: RandomFloatBetween(0.5*l.pointing, l.pointing),
			Flux:           flux + flare_flux_a,
			FluxErr:        RandomFloatBetween(0.5*l.scatter, l.scatter),
		},
		{
			SourceID:       l.SourceID,
			Time:           t,
			Frequency:      m.Frequencies[1],
			Module:         m.Identifier,
			Ra:             l.Ra,
			Dec:            l.Dec,
			RaUncertainty:  RandomFloatBetween(0.5*l.pointing, l.pointing),
			DecUncertainty: RandomFloatBetween(0.5*l.pointing, l.pointing),
			Flux:           flux + flare_flux_b,
			FluxErr:        RandomFloatBetween(0.5*l.scatter, l.scatter),
		},
	}
}
