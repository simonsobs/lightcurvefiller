package lightcurvefiller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"mime/multipart"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
)

// Configuration for API connections to the Lightgest server
type LightServeConfiguration struct {
	host               string // Hostname (including port) of lightgest server
	batch_size         int    // Size of batches to upload data in
	use_bearer         bool   // Whether to use the Bearer token
	bearer             string // Bearer token (only used if use_bearer)
	upload_parquet     bool   // Whether to upload using parquet
	allow_self_signed  bool   // Whether to allow self-signed certificates
	enable             bool   // Whether to actually upload things to lightserve
	upload_instruments bool   // Whether to upload telescope/instrument metadata
	number_of_workers  int    // Number of upload workers for 'actual' data
}

type InstrumentUploadDetails struct {
	Detail string `json:"detail"`
}

// Helper type for uploading sources
type SourceUpload struct {
	SourceID uuid.UUID `json:"source_id" parquet:"source_id"`
	Name     string    `json:"name" parquet:"name"`
	Ra       float64   `json:"ra" parquet:"ra"`
	Dec      float64   `json:"dec" parquet:"dec"`
	Variable bool      `json:"variable" parquet:"variable"`
	Extra    any       `json:"extra" parquet:"extra,variant"`
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

// Upload source information to the Lightgest API.
func (c LightServeConfiguration) UploadSources(sources []SourceUpload) {
	url := fmt.Sprintf("%s/sources/batch", c.host)
	client := c.GetClient()
	number_of_batches := int(math.Ceil(float64(len(sources)) / float64(c.batch_size)))

	for batch := range number_of_batches {
		start_batch := batch * c.batch_size
		end_batch := min((batch+1)*c.batch_size, len(sources))

		batched_data := sources[start_batch:end_batch]
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
			log.Println("Failed to send data to /sources/batch endpoint ", res)
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
			log.Println("Failed to send data to /instruments/ endpoint ", res, err)
		}
	}
}

func uploadDataBatch(
	data *[]LightcurveDatapoint,
	batch_size int,
	url string,
	client *http.Client,
	batch_id <-chan int,
	timing chan<- time.Duration,
) {
	for batch := range batch_id {
		start_batch := batch * batch_size
		end_batch := min((batch+1)*batch_size, len(*data))

		data_upload := DataUpload{
			FluxMeasurements: (*data)[start_batch:end_batch],
		}

		json_batch, err := json.Marshal(data_upload)

		if err != nil {
			log.Panic("Could not marshal lightcurve data to JSON")
		}

		status_code := 999
		failures := 0
		time_to_send := time.Duration(-1000)

		for status_code != 200 {
			start := time.Now()
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
			time_to_send = time.Since(start)

			if status_code != 200 {
				log.Printf("Error uploading data: %d", status_code)
				time.Sleep(time.Duration(failures*5) * time.Second)
				failures += 1
			}

			if failures > 5 {
				log.Panic("Failed over 5 times to send data to API endpoint")
			}
		}

		timing <- time_to_send
	}
}

func uploadCutoutBatch(
	data *[]Cutout,
	batch_size int,
	url string,
	client *http.Client,
	batch_id <-chan int,
	timing chan<- time.Duration,
) {
	for batch := range batch_id {
		start_batch := batch * batch_size
		end_batch := min((batch+1)*batch_size, len(*data))

		data_upload := DataUpload{
			Cutouts: (*data)[start_batch:end_batch],
		}

		json_batch, err := json.Marshal(data_upload)

		if err != nil {
			log.Panic("Could not marshal lightcurve data to JSON")
		}

		status_code := 999
		failures := 0
		time_to_send := time.Duration(-1000)

		for status_code != 200 {
			start := time.Now()
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
				body, readErr := io.ReadAll(res.Body)
				res.Body.Close()
				log.Println("Failed to send data to /observations/batch endpoint", res, body, readErr)
			}

			status_code = res.StatusCode
			time_to_send = time.Since(start)

			if status_code != 200 {
				log.Printf("Error uploading data: %d", status_code)
				time.Sleep(time.Duration(failures*5) * time.Second)
				failures += 1
			}

			if failures > 5 {
				log.Panic("Failed over 5 times to send data to API endpoint")
			}
		}

		timing <- time_to_send
	}
}

// Upload data to the Lightgest API in batches.
// We always use the batch endpoint, it is much faster.
// We upload data using goroutines in parallel.
func (c LightServeConfiguration) UploadData(data []LightcurveDatapoint, cutouts []Cutout) (time.Duration, []time.Duration) {
	number_of_data_batches := int(math.Ceil(float64(len(data)) / float64(c.batch_size)))
	number_of_cutout_batches := int(math.Ceil(float64(len(cutouts)) / float64(c.batch_size)))
	total_batches := number_of_cutout_batches + number_of_data_batches
	log.Printf("Uploading using %d batches\n", total_batches)
	url := fmt.Sprintf("%s/observations/batch", c.host)
	client := c.GetClient()

	data_batch_ids := make(chan int, number_of_data_batches)
	data_timing_channel := make(chan time.Duration, number_of_data_batches)
	var timings []time.Duration

	start := time.Now()

	// Upload all data before any cutouts because we rely on foreign key constraints
	if data != nil {
		for w := 1; w <= c.number_of_workers; w++ {
			go uploadDataBatch(&data, c.batch_size, url, client, data_batch_ids, data_timing_channel)
		}

		for batch := range number_of_data_batches {
			data_batch_ids <- batch
		}

		close(data_batch_ids)

		for range number_of_data_batches {
			timings = append(timings, <-data_timing_channel)
		}
	}

	// Now we can upload the cutouts as we have confirmed we have all the flux measurements.
	if cutouts != nil {
		cutout_batch_ids := make(chan int, number_of_cutout_batches)
		cutout_timing_channel := make(chan time.Duration, number_of_cutout_batches)

		for w := 1; w <= c.number_of_workers; w++ {
			go uploadCutoutBatch(&cutouts, c.batch_size, url, client, cutout_batch_ids, cutout_timing_channel)
		}

		for batch := range number_of_cutout_batches {
			cutout_batch_ids <- batch
		}

		close(cutout_batch_ids)

		for range number_of_cutout_batches {
			timings = append(timings, <-cutout_timing_channel)
		}
	}

	total_time := time.Since(start)

	return total_time, timings
}

// Upload a parquet file that we just made to the API. Does not
// currently support uploading of cutouts.
func (c LightServeConfiguration) UploadParquet(filename string) error {
	url := fmt.Sprintf("%s/observations/parquet", c.host)
	client := c.GetClient()

	log.Printf("Attempting to upload parquet file %s to %s", filename, url)

	contents, err := os.ReadFile(filename)
	if err != nil {
		log.Panic("Unable to open file", filename)
	}

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("file", "upload.parquet")

	if err != nil {
		log.Panic("Failed to create writer form file")
	}

	part.Write(contents)

	if err := writer.Close(); err != nil {
		return err
	}

	request, err := http.NewRequest(
		http.MethodPost,
		url,
		&buf,
	)
	request.Header.Set("Content-Type", writer.FormDataContentType())

	if err != nil {
		log.Panic("Error creating HTTP request")
	}

	log.Printf("Header Content-Type: %s", request.Header.Get("Content-Type"))
	res, err := client.Do(request)

	if err != nil {
		log.Println("Failed to send data to /observations/parquet endpoint ", res)
	}

	status_code := res.StatusCode

	if status_code != 200 {
		body, err := io.ReadAll(res.Body)
		if err != nil {
			return err
		}
		log.Printf("Error uploading data: %d, %s\n", status_code, body)
	}

	return err
}
