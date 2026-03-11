package lightcurvefiller

import (
	"math/rand"
	"time"
)

type Module struct {
	Identifier  string
	Frequencies [2]int
}

type Telescope struct {
	Name    string
	Modules []Module
}

type ObservingCampaign struct {
	Start     time.Time
	End       time.Time
	Interval  time.Duration
	Jitter    time.Duration // The interval within which we randomly sample observation times for individual modules
	Telescope Telescope
}

func CreateSampleTelescope() Telescope {
	return Telescope{
		Name: "LAT",
		Modules: []Module{
			{
				Identifier:  "c1",
				Frequencies: [2]int{220, 280},
			},
			{
				Identifier:  "i1",
				Frequencies: [2]int{90, 150},
			},
			{
				Identifier:  "i2",
				Frequencies: [2]int{220, 280},
			},
			{
				Identifier:  "i3",
				Frequencies: [2]int{90, 150},
			},
			{
				Identifier:  "i4",
				Frequencies: [2]int{90, 150},
			},
			{
				Identifier:  "i5",
				Frequencies: [2]int{220, 280},
			},
			{
				Identifier:  "i6",
				Frequencies: [2]int{90, 150},
			},
			{
				Identifier:  "o1",
				Frequencies: [2]int{220, 280},
			},
			{
				Identifier:  "o2",
				Frequencies: [2]int{90, 150},
			},
			{
				Identifier:  "o3",
				Frequencies: [2]int{90, 150},
			},
			{
				Identifier:  "o4",
				Frequencies: [2]int{90, 150},
			},
			{
				Identifier:  "o5",
				Frequencies: [2]int{90, 150},
			},
			{
				Identifier:  "o6",
				Frequencies: [2]int{30, 40},
			},
		},
	}
}

func CreateSampleObservingCampaign() ObservingCampaign {
	return ObservingCampaign{
		Start:     time.Now().Add(-time.Hour * 8760), // 1 year
		End:       time.Now(),
		Interval:  time.Hour * 24,
		Jitter:    time.Minute * 15,
		Telescope: CreateSampleTelescope(),
	}
}

func (o ObservingCampaign) ObserveLightcurvesAt(lightcurves []Lightcurve, t time.Time) []LightcurveDatapoint {
	// Observe the objects! Note that we always do this in module order, so we first
	// generate the order that we'll observe the objects in.
	object_order := rand.Perm(len(lightcurves))

	current_observation := 0
	results := make([]LightcurveDatapoint, len(lightcurves)*len(o.Telescope.Modules)*2)

	for _, module := range o.Telescope.Modules {
		for observed_order, object_index := range object_order {
			l := lightcurves[object_index]

			this_jitter := GenerateRandomDuration(0, o.Jitter)
			observation_fraction := float64(observed_order) / float64(len(object_order))
			observation_time_elapsed := time.Duration(observation_fraction * float64(o.Interval))
			this_time := t.Add(observation_time_elapsed).Add(this_jitter)

			obs := l.GenerateDataPoint(this_time, module)
			results[current_observation] = obs[0]
			results[current_observation+1] = obs[1]

			current_observation += 2
		}
	}

	return results
}
