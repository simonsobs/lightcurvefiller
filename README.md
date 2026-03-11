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

Configuration ocurrs directly in the source code at this time. See
`main.go`.