package store

import (
	"fmt"
	"regexp"
	"time"

	"github.com/tableauio/tableau/xerrors"
)

// Define a regex pattern to match RFC3339 timestamps
var timestampPattern = `"\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?Z"`
var tsRegexp *regexp.Regexp

func init() {
	// Compile the regex
	tsRegexp = regexp.MustCompile(timestampPattern)
}

// deprecatedProcessWhenEmitTimezones provides a regex implementation for processing timestamps
// This is only for comparison in benchmarks, because it fails when the content of a string field matches the timestamp pattern
func deprecatedProcessWhenEmitTimezones(jsonStr string, locationName string) (string, error) {
	loc, err := time.LoadLocation(locationName)
	if err != nil {
		return "", xerrors.Wrap(err)
	}
	// Replace timestamps in the JSON string with the desired format
	result := tsRegexp.ReplaceAllStringFunc(jsonStr, func(ts string) string {
		// Remove quotes from the timestamp string
		ts = ts[1 : len(ts)-1]
		// Format the timestamp
		formattedTs := formatTimestamp(ts, loc)
		// Add quotes back to the formatted timestamp
		return fmt.Sprintf(`"%s"`, formattedTs)
	})
	return result, nil
}
