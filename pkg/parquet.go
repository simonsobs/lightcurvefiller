package lightcurvefiller

import (
	"fmt"
	"log"
	"path"
	"time"

	"github.com/parquet-go/parquet-go"
)

// Configuration for the parquet output.
type ParquetConfiguration struct {
	enable    bool
	base_path string
	compress  bool
}

// Write the day's data to a single parquet file.
func (p ParquetConfiguration) WriteData(data []LightcurveDatapoint, date time.Time) (string, error) {
	filename := path.Join(p.base_path, fmt.Sprintf("%s.parquet", date.Format(time.DateOnly)))

	log.Printf("Writing parquet file to %s", filename)

	options := []parquet.WriterOption{}

	if p.compress {
		options = append(options, parquet.Compression(&parquet.Gzip))
	}

	err := parquet.WriteFile(filename, data, options...)

	return filename, err
}

// Read a set of lightcurve data points from a parquet file.
func ReadLightcurveFromParquet(filename string) []LightcurveDatapoint {
	log.Printf("Reading parquet file at %s to extract lightcurve datapoints", filename)

	rows, err := parquet.ReadFile[LightcurveDatapoint](filename)

	if err != nil {
		log.Fatalln("Unable to read paruqet file at", filename, err)
	}

	for index := range rows {
		rows[index].Ra = rows[index].Ra - 180.0
	}

	return rows
}

// Read a set of cutouts from a parquet file
func ReadCutoutFromParquet(filename string) []Cutout {
	log.Printf("Reading parquet file at %s to extract cutouts", filename)

	rows, err := parquet.ReadFile[Cutout](filename)

	if err != nil {
		log.Fatalln("Unable to read paruqet file at", filename, err)
	}

	return rows
}

// Read a set of sources from a parquet file
func ReadSourceFromParquet(filename string) []SourceUpload {
	log.Printf("Reading parquet file at %s to extract source information", filename)

	rows, err := parquet.ReadFile[SourceUpload](filename)

	if err != nil {
		log.Fatalln("Unable to read parquet file at", filename, err)
	}

	for index := range rows {
		rows[index].Ra = rows[index].Ra - 180.0
	}

	return rows
}
