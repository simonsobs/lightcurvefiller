package lightcurvefiller

import (
	"fmt"
	"log"
	"path"
	"time"

	"github.com/segmentio/parquet-go"
)

// Configuration for the parquet output.
type ParquetConfiguration struct {
	enable    bool
	base_path string
	compress  bool
}

// Write the day's data to a single parquet file.
func (p ParquetConfiguration) WriteData(data []LightcurveDatapoint, date time.Time) error {
	filename := path.Join(p.base_path, fmt.Sprintf("%s.parquet", date.Format(time.DateOnly)))

	log.Printf("Writing parquet file to %s", filename)

	options := []parquet.WriterOption{}

	if p.compress {
		options = append(options, parquet.Compression(&parquet.Gzip))
	}

	err := parquet.WriteFile(filename, data, options...)

	return err
}
