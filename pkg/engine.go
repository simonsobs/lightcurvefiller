package lightcurvefiller

import (
	"math/rand"
	"time"
)

// Observing module, like an optics tube
type Module struct {
	Identifier  string // Identifier of this observing module (e.g. i1)
	Frequencies [2]int // Frequencies that this module observes at
}

// A telescope is just a collection of optics tubes
type Telescope struct {
	Name    string   // Name of telescope
	Modules []Module // Observing modules of the telescope (e.g. optics tubes)
}

// Configuration of the entire observing campaign; lightcurves
// will be observed over this period. Lightcruves will be observed
// for 24 hours a day, taking the entirety of `Interval` to observe
// all registered lightcurves
type ObservingCampaign struct {
	Start     time.Time     // Start time of the osberving campaign
	End       time.Time     // End time of the observing campaign
	Interval  time.Duration // Interval between individaul obserations; we assume 24 observation
	Jitter    time.Duration // The interval within which we randomly sample observation times for individual modules
	Telescope Telescope     // Telescope definition
}

// Create the 13-OT LAT.
func CreateLAT() Telescope {
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

// Observe the objects! Note that we always do this in module order, so we first
// generate the order that we'll observe the objects in. This returns the result
// of an entire `Interval`-length observing run.
func (o ObservingCampaign) ObserveLightcurvesAt(lightcurves []Lightcurve, t time.Time) []LightcurveDatapoint {
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
