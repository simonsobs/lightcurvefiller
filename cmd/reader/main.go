package main

import (
	"flag"
	"log"
	"slices"

	lc "joshborrow.com/lightcurvefiller/pkg"
)

var EMPTY_VARIABLE = "__undefined"

// Parse out a potentially unique list of instruemnts from the various
// lightcurves for upload to the system.
func parseInstruments(data []lc.LightcurveDatapoint, telescope string) lc.Telescope {
	// Map of frequencies to list of modules at that frequency
	combinations := make(map[string][]int)

	for _, lc := range data {
		frequencies, ok := combinations[lc.Module]

		if ok {
			if slices.Contains(frequencies, lc.Frequency) {
				continue
			} else {
				combinations[lc.Module] = append(frequencies, lc.Frequency)
			}
		} else {
			combinations[lc.Module] = []int{lc.Frequency}
		}
	}

	modules := make([]lc.Module, 0)
	for module, frequencies := range combinations {
		log.Println("Found frequency combination", frequencies, "for module", module)

		normalized_frequencies := [2]int{0, 0}

		if len(frequencies) > 0 {
			normalized_frequencies[0] = frequencies[0]
		}

		if len(frequencies) > 1 {
			normalized_frequencies[1] = frequencies[1]
		}

		modules = append(
			modules,
			lc.Module{
				Frequencies: normalized_frequencies,
				Identifier:  module,
			},
		)
	}

	return lc.Telescope{
		Name:    telescope,
		Modules: modules,
	}
}

func main() {
	lightserve_config := lc.ReadLightserveConfigFromEnvironment()
	lightserve_config.Print()

	var lightcurve_file, cutout_file, source_file, telescope string
	var create_instruments bool
	flag.StringVar(&lightcurve_file, "lightcurve", EMPTY_VARIABLE, "The lightcurve parquet file to upload, optional")
	flag.StringVar(&cutout_file, "cutout", EMPTY_VARIABLE, "The cutout parquet file to upload, optional")
	flag.StringVar(&source_file, "source", EMPTY_VARIABLE, "The source parquet file to upload, optional")
	flag.BoolVar(&create_instruments, "instruments", false, "Automatically try to create instruments corresponding to the data")
	flag.StringVar(&telescope, "telescope", "ACT", "Name of telescope to use in instrument upload")
	flag.Parse()

	var upload_sources []lc.SourceUpload
	var upload_telescope lc.Telescope
	upload_data := lc.DataUpload{}

	if source_file != EMPTY_VARIABLE {
		upload_sources = lc.ReadSourceFromParquet(source_file)
	}

	if lightcurve_file != EMPTY_VARIABLE {
		upload_data.FluxMeasurements = lc.ReadLightcurveFromParquet(lightcurve_file)
	}

	if create_instruments {
		upload_telescope = parseInstruments(upload_data.FluxMeasurements, telescope)
	}

	if cutout_file != EMPTY_VARIABLE {
		upload_data.Cutouts = lc.ReadCutoutFromParquet(cutout_file)
	}

	lightserve_config.UploadInstruments(upload_telescope)
	lightserve_config.UploadSources(upload_sources)
	lightserve_config.UploadData(upload_data.FluxMeasurements, upload_data.Cutouts)
}
