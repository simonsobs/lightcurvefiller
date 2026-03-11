package main

import (
	"log"

	lc "joshborrow.com/lightcurvefiller/pkg"
)

const NUMBER_OF_OBJECTS = 1000

func main() {
	// Create and send off our data!
	lightcurve_configuration := lc.SampleLightcurveConfiguration()
	lightserve_configuration := lc.CreateSampleLightServeConfiguration()
	campaign := lc.CreateSampleObservingCampaign()

	lightcurves := make([]lc.Lightcurve, NUMBER_OF_OBJECTS)

	for index := range lightcurves {
		lightcurves[index] = lc.NewLightcurve(lightcurve_configuration)
	}

	// Upload necessary metadata to lightserve
	lightserve_configuration.UploadInstruments(campaign.Telescope)
	log.Println("Successfully uploaded telescope information")
	lightserve_configuration.UploadSources(lightcurves)
	log.Println("Successfully uploaded source metadata")

	number_of_observing_periods := campaign.End.Sub(campaign.Start) / campaign.Interval

	for observing_period := range number_of_observing_periods {
		time := campaign.Start.Add(campaign.Interval * observing_period)
		observations := campaign.ObserveLightcurvesAt(lightcurves, time)
		log.Printf("Generated %d observations for time %s\n", len(observations), time)
		lightserve_configuration.UploadData(observations)
	}
}
