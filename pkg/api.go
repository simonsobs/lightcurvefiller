package lightcurvefiller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"time"
)

// Configuration for API connections to the Lightgest server
type LightServeConfiguration struct {
	host              string // Hostname (including port) of lightgest server
	batch_size        int    // Size of batches to upload data in
	use_bearer        bool   // Whether to use the Bearer token
	bearer            string // Bearer token (only used if use_bearer)
	allow_self_signed bool   // Whether to allow self-signed certificates
	enable            bool   // Whether to actually upload things to lightserve
}

type InstrumentUploadDetails struct {
	Detail string `json:"detail"`
}

// Helper type for uploading modules as 'instruments'
type InstrumentUpload struct {
	Frequency  int                     `json:"frequency"`
	Module     string                  `json:"module"`
	Telescope  string                  `json:"telescope"`
	Instrument string                  `json:"instrument"`
	Details    InstrumentUploadDetails `json:"details"`
}

// Helper type for batched uploads of lightcurve data
type DataUpload struct {
	FluxMeasurements []LightcurveDatapoint `json:"flux_measurements"`
	Cutouts          []Cutout              `json:"cutouts"`
}

// Upload source information to the Lightgest API. Currently
// no batch endpoint is available so this may take some time.
func (c LightServeConfiguration) UploadSources(lightcurves []Lightcurve) {
	url := fmt.Sprintf("%s/sources/batch", c.host)
	client := c.GetClient()
	number_of_batches := int(math.Ceil(float64(len(lightcurves)) / float64(c.batch_size)))

	for batch := range number_of_batches {
		start_batch := batch * c.batch_size
		end_batch := min((batch+1)*c.batch_size, len(lightcurves))

		batched_data := lightcurves[start_batch:end_batch]
		json_content, err := json.Marshal(batched_data)

		if err != nil {
			log.Panic("Could not marshal source batch to JSON")
		}

		request, err := http.NewRequest(
			http.MethodPut,
			url,
			bytes.NewBuffer(json_content),
		)

		if err != nil {
			log.Panic("Error creating HTTP request")
		}

		res, err := client.Do(request)

		if err != nil || res.StatusCode != 200 {
			log.Panic("Failed to send data to /sources/batch endpoint ", res)
		}
	}
}

// Upload instrument information to the Lightgest API, stored internally
// here as 'Module' information.
func (c LightServeConfiguration) UploadInstruments(telescope Telescope) {
	instruments := make([]InstrumentUpload, len(telescope.Modules)*2)

	for index, module := range telescope.Modules {
		instruments[index*2] = InstrumentUpload{
			Frequency:  module.Frequencies[0],
			Module:     module.Identifier,
			Telescope:  telescope.Name,
			Instrument: fmt.Sprintf("%s-%s", telescope.Name, module.Identifier),
			Details:    InstrumentUploadDetails{Detail: "test"},
		}
		instruments[index*2+1] = InstrumentUpload{
			Frequency:  module.Frequencies[1],
			Module:     module.Identifier,
			Telescope:  telescope.Name,
			Instrument: fmt.Sprintf("%s-%s", telescope.Name, module.Identifier),
			Details:    InstrumentUploadDetails{Detail: "test"},
		}
	}

	url := fmt.Sprintf("%s/instruments/", c.host)
	client := c.GetClient()

	for _, instrument := range instruments {
		json_content, err := json.Marshal(instrument)

		if err != nil {
			log.Panic("Could not marshal instrument to JSON ", instrument)
		}

		request, err := http.NewRequest(
			http.MethodPut,
			url,
			bytes.NewBuffer(json_content),
		)

		if err != nil {
			log.Panic("Error creating HTTP request")
		}

		res, err := client.Do(request)

		if err != nil || res.StatusCode != 200 {
			log.Panic("Failed to send data to /instruments/ endpoint ", res, err)
		}
	}
}

// Upload data to the Lightgest API in batches.
// We always use the batch endpoint, it is much faster.
func (c LightServeConfiguration) UploadData(data []LightcurveDatapoint, cutouts []Cutout) {
	number_of_batches := int(math.Ceil(float64(len(data)) / float64(c.batch_size)))
	url := fmt.Sprintf("%s/observations/batch", c.host)
	client := c.GetClient()

	for batch := range number_of_batches {
		start_batch := batch * c.batch_size
		end_batch := min((batch+1)*c.batch_size, len(data))

		var batched_cutouts []Cutout

		if cutouts != nil {
			batched_cutouts = cutouts[start_batch:end_batch]
		} else {
			batched_cutouts = nil
		}

		json_batch, err := json.Marshal(DataUpload{
			FluxMeasurements: data[start_batch:end_batch],
			Cutouts:          batched_cutouts,
		})

		if err != nil {
			log.Panic("Could not marshal lightcurve data to JSON")
		}

		status_code := 999
		failures := 0

		for status_code != 200 {
			request, err := http.NewRequest(
				http.MethodPut,
				url,
				bytes.NewBuffer(json_batch),
			)

			if err != nil {
				log.Panic("Error creating HTTP request")
			}

			res, err := client.Do(request)

			if err != nil {
				log.Println("Failed to send data to /observations/batch endpoint ", res)
			}

			status_code = res.StatusCode

			if status_code != 200 {
				log.Printf("Error uploading data: %d", status_code)
				time.Sleep(time.Duration(failures*5) * time.Second)
				failures += 1
			}

			if failures > 5 {
				log.Panic("Failed over 5 times to send data to API endpoint")
			}
		}

	}
}
