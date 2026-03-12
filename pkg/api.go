package lightcurvefiller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
)

// Configuration for API connections to the Lightgest server
type LightServeConfiguration struct {
	host       string // Hostname (including port) of lightgest server
	batch_size int    // Size of batches to upload data in
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
}

// Upload source information to the Lightgest API. Currently
// no batch endpoint is available so this may take some time.
func (c LightServeConfiguration) UploadSources(lightcurves []Lightcurve) {
	url := fmt.Sprintf("%s/sources/", c.host)
	client := &http.Client{}

	for _, source := range lightcurves {
		json_content, err := json.Marshal(source)

		if err != nil {
			log.Panic("Could not marshal source to JSON ", source)
		}

		request, err := http.NewRequest(
			http.MethodPut,
			url,
			bytes.NewBuffer(json_content),
		)

		if err != nil {
			log.Panic("Error creating HTTP request")
		}

		request.Header.Add("Content-Type", "application/json")

		res, err := client.Do(request)

		if err != nil || res.StatusCode != 200 {
			log.Panic("Failed to send data to /sources/ endpoint ", res)
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
	client := &http.Client{}

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

		request.Header.Add("Content-Type", "application/json")

		res, err := client.Do(request)

		if err != nil || res.StatusCode != 200 {
			log.Panic("Failed to send data to /instruments/ endpoint ", res)
		}
	}
}

// Upload data to the Lightgest API in batches.
// We always use the batch endpoint, it is much faster.
func (c LightServeConfiguration) UploadData(data []LightcurveDatapoint) {
	number_of_batches := int(math.Ceil(float64(len(data)) / float64(c.batch_size)))
	url := fmt.Sprintf("%s/observations/batch", c.host)
	client := &http.Client{}

	for batch := range number_of_batches {
		start_batch := batch * c.batch_size
		end_batch := min((batch+1)*c.batch_size, len(data))
		json_batch, err := json.Marshal(DataUpload{
			FluxMeasurements: data[start_batch:end_batch],
		})

		if err != nil {
			log.Panic("Could not marshal lightcurve data to JSON")
		}

		request, err := http.NewRequest(
			http.MethodPut,
			url,
			bytes.NewBuffer(json_batch),
		)

		request.Header.Add("Content-Type", "application/json")

		if err != nil {
			log.Panic("Error creating HTTP request")
		}

		res, err := client.Do(request)

		if err != nil || res.StatusCode != 200 {
			log.Panic("Failed to send data to /sources/batch endpoint ", res)
		}
	}
}
