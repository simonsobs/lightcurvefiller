package lightcurvefiller

import (
	"fmt"
	"log"
	"os"
	"path"
	"reflect"
	"time"

	"github.com/parquet-go/parquet-go"
)

// Configuration for the parquet output.
type ParquetConfiguration struct {
	enable    bool
	base_path string
	compress  bool
}

// Read a parquet file. This wrapper is needed to support `any` values in the
// struct that are read from parquet. This is useful for e.g. structs containing
// the `extra` field that are intended to become JSONB columns.
func ReadParquetFile[T any](filename string) ([]T, error) {
	if reflect.TypeFor[T]().Kind() != reflect.Struct {
		return nil, fmt.Errorf("ReadParquetFile: provided a type that is not a struct")
	}

	f, err := os.Open(filename)

	if err != nil {
		return nil, err
	}

	defer f.Close()

	info, err := os.Stat(filename)

	if err != nil {
		return nil, err
	}

	// Use the lower-level API than in other cases
	parquet_file, err := parquet.OpenFile(f, info.Size())

	if err != nil {
		return nil, err
	}

	target_schema := parquet.SchemaOf(new(T))
	available_in_file := make(map[string]parquet.Node)

	for _, field := range parquet_file.Schema().Fields() {
		available_in_file[field.Name()] = field
	}

	fields := parquet.Group{}
	for _, field := range target_schema.Fields() {
		if node, ok := available_in_file[field.Name()]; ok {
			fields[field.Name()] = node
		}
	}

	if len(fields) == 0 {
		return nil, fmt.Errorf("ReadParquetFile: no matching columns found in file")
	}

	schema := parquet.NewSchema(target_schema.Name(), fields)

	return parquet.Read[T](f, info.Size(), schema)
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

	rows, err := ReadParquetFile[LightcurveDatapoint](filename)

	if err != nil {
		log.Fatalln("Unable to read paruqet file at", filename, err)
	}

	for index, row := range rows {
		rows[index].Ra = row.Ra - 180.0
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
