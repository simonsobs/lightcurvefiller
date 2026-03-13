LightCurveFiller
================

This is a data simulation tool designed to produce fake lightcurves with
a similar sampling strategy to the real SO pipeline. The lightcurve data is
then uploaded to the `lightgest` server.

Requirements
------------

- `go > 1.25.0`
- `github.com/google/uuid v1.6.0`

Building
--------

The application can be built using the `golang` toolchain:
```shell
go build cmd/main.go
```
This produces a binary, `main`. You can invoke this with `./main`

Configuration
-------------

All relevant parameters can be configured using environment
variables. By default, `main` will print the list of used parameters
to the console. They are (note that dates default to the current date
as 'end' and now minus one year for start):
```shell
LIGHTCURVE_EARLIEST_PEAK_TIME=2025-03-13
LIGHTCURVE_LATEST_PEAK_TIME=2026-03-13
LIGHTCURVE_SHORTEST_WIDTH=24h0m0s
LIGHTCURVE_LONGEST_WIDTH=512h0m0s
LIGHTCURVE_LOWEST_BASE=50.000000
LIGHTCURVE_HIGHEST_BASE=500.000000
LIGHTCURVE_LOWEST_FLARE=200.000000
LIGHTCURVE_HIGHEST_FLARE=3000.000000
LIGHTCURVE_LOWEST_SCATTER=25.000000
LIGHTCURVE_HIGHEST_SCATTER=200.000000
LIGHTCURVE_LOWEST_SPECTRAL_INDEX=-1.000000
LIGHTCURVE_HIGHEST_SPECTRAL_INDEX=2.000000
LIGHTCURVE_LOWEST_RA=-180.000000
LIGHTCURVE_HIGHEST_RA=180.000000
LIGHTCURVE_LOWEST_DEC=-70.000000
LIGHTCURVE_HIGHEST_DEC=25.000000
LIGHTCURVE_POINTING=0.016667
CUTOUT_ENABLE=yes
CUTOUT_PIXEL_SIZE=0.008333
CUTOUT_UNITS=mJy
LIGHTSERVE_HOST=http://localhost:8001
LIGHTSERVE_BATCH_SIZE=2048
OBSERVATION_START=2025-03-13
OBSERVATION_END=2026-03-13
OBSERVATION_INTERVAL=24h0m0s
OBSERVATION_JITTER=15m0s
TELESCOPE=LAT
NUMBER_OF_OBJECTS=100
PRINT_CONFIG=yes
```