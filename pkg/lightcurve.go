package lightcurvefiller

import (
	"log"
	"math"
	"time"

	"github.com/google/uuid"
)

type LightcurveConfiguration struct {
	earliest_peak_time time.Time
	latest_peak_time   time.Time

	shortest_width time.Duration
	longest_width  time.Duration

	lowest_base  float64
	highest_base float64

	lowest_flare  float64
	highest_flare float64

	lowest_scatter  float64
	highest_scatter float64

	lowest_spectral_index  float64
	highest_spectral_index float64

	lowest_ra   float64
	highest_ra  float64
	lowest_dec  float64
	highest_dec float64
	pointing    float64
}

func SampleLightcurveConfiguration() LightcurveConfiguration {
	return LightcurveConfiguration{
		earliest_peak_time:     time.Now().Add(-time.Duration(time.Hour * 8760)), // 1 year
		latest_peak_time:       time.Now(),
		shortest_width:         time.Hour * 24,
		longest_width:          time.Hour * 512,
		lowest_base:            50.0,
		highest_base:           500.0,
		lowest_flare:           200.0,
		highest_flare:          3000.0,
		lowest_scatter:         25.0,
		highest_scatter:        200.0,
		lowest_spectral_index:  -1.0,
		highest_spectral_index: 2.0,
		lowest_ra:              -180.0,
		highest_ra:             180.0,
		lowest_dec:             -70.0,
		highest_dec:            25.0,
		pointing:               1.0 / 60.0,
	}
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
			Flux:           flux + flare_flux_base*freq_offset_a,
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
			Flux:           flux + flare_flux_base*freq_offset_b,
			FluxErr:        RandomFloatBetween(0.5*l.scatter, l.scatter),
		},
	}
}
