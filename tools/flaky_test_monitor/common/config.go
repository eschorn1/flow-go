package common

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Properties used in property file
const FailureThresholdPercent = "failures_threshold_percent"
const FailuresSliceMax = "failures_slice_max"

const DurationThresholdSeconds = "duration_threshold_seconds"
const DurationSliceMax = "duration_slice_max"

type Config struct {
	FailureThresholdPercent float32
	FailuresSliceMax        int

	DurationThresholdSeconds float32
	DurationSliceMax         int
}

func ReadProperties(directory string) Config {
	// looks for properties file with specific name
	propertyFile := filepath.Join(directory, "flaky-test-monitor.properties")

	file, err := os.Open(propertyFile)

	AssertNoError(err, "error opening property file")
	defer file.Close()

	properties := map[string]string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		// read in line by line
		line := scanner.Text()

		// if line starts with # or //, treat that as a comment and skip it
		if strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}

		// if line is empty or doesn't follow the `property=value` format, skip it
		property := strings.Split(line, "=")
		if len(property) != 2 {
			continue
		}

		// add property and trim any whitespace from property name and value
		properties[strings.TrimSpace(property[0])] = strings.TrimSpace(property[1])
	}

	// create struct with all properties set for easy retrieval
	config := Config{}

	failureThresholdPercent64, err := strconv.ParseFloat(properties[FailureThresholdPercent], 32)
	AssertNoError(err, "error converting string to float (FailureThresholdPercent)")
	config.FailureThresholdPercent = float32(failureThresholdPercent64)

	failuresSliceMax, err := strconv.Atoi(properties[FailuresSliceMax])
	AssertNoError(err, "error converting string to int (FailuresSliceMax)")
	config.FailuresSliceMax = failuresSliceMax

	durationThresholdSeconds64, err := strconv.ParseFloat(properties[DurationThresholdSeconds], 32)
	AssertNoError(err, "error converting string to float (DurationThresholdSeconds)")
	config.DurationThresholdSeconds = float32(durationThresholdSeconds64)

	durationSliceMax, err := strconv.Atoi(properties[DurationSliceMax])
	AssertNoError(err, "error converting string to int (DurationSliceMax)")
	config.DurationSliceMax = durationSliceMax

	return config
}
