package lightcurvefiller

import (
	"log"
	"math"
	"time"

	"github.com/google/uuid"
)

const CUTOUT_SIZE int = 32

type CutoutConfiguration struct {
	enabled    bool
	size       int
	pixel_size float64
	beam_size  map[int]float64
	units      string
}

type Cutout struct {
	Data      [CUTOUT_SIZE][CUTOUT_SIZE]float64 `json:"data"`
	Time      time.Time                         `json:"time"`
	Units     string                            `json:"units"`
	Frequency int                               `json:"frequency"`
	Module    string                            `json:"module"`
	SourceID  uuid.UUID                         `json:"source_id"`
}

// Generate sample beams for the LAT
func CreateLATBeams() map[int]float64 {
	return map[int]float64{
		30:  5.0 / 60.0,
		40:  4.0 / 60.0,
		90:  2.2 / 60.0,
		150: 1.5 / 60.0,
		220: 1.0 / 60.0,
		280: 0.8 / 60.0,
	}
}

// Generate a matching cut-out for an indiivdual measurement
func (c CutoutConfiguration) GenerateCutout(measurement LightcurveDatapoint) Cutout {
	if !c.enabled {
		log.Fatalln("Attempting to generate a cut-out, but the config says this feature is disabled.")
	}

	beam := c.beam_size[measurement.Frequency]
	beam_square := beam * beam
	output := [CUTOUT_SIZE][CUTOUT_SIZE]float64{}

	for x := range CUTOUT_SIZE {
		x_pixel := float64(x-CUTOUT_SIZE/2)*c.pixel_size + RandomSign()*measurement.RaUncertainty
		for y := range CUTOUT_SIZE {
			y_pixel := float64(y-CUTOUT_SIZE/2)*c.pixel_size + RandomSign()*measurement.DecUncertainty

			base := RandomFloatBetween(0.5*measurement.FluxErr, measurement.FluxErr)
			exponent := (x_pixel*x_pixel + y_pixel*y_pixel) / beam_square

			output[y][x] = measurement.Flux*math.Exp(-exponent) + base
		}
	}

	return Cutout{
		Data:      output,
		Time:      measurement.Time,
		Units:     c.units,
		Frequency: measurement.Frequency,
		Module:    measurement.Module,
		SourceID:  measurement.SourceID,
	}
}
