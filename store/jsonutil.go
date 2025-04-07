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

// Format a timestamp to the desired string format
func formatTimestamp(ts string, loc *time.Location) string {
	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return ts // Return the original string if parsing fails
	}
	localTime := t.In(loc)
	return localTime.Format("2006-01-02T15:04:05-07:00")
}

// processWhenUseTimezones emits timestamp in string format with timezones
// (as indicated by an offset).
//
// # Problem
//
// A proto3 JSON serializer should always use UTC (as indicated by "Z") when
// printing the Timestamp type and a proto3 JSON parser should be able to
// accept both UTC and other timezones (as indicated by an offset).
//
// For example, "2017-01-15T01:30:15.01Z" encodes 15.01 seconds past 01:30 UTC
// on January 15, 2017.
//
// # References
//
//   - https://pkg.go.dev/google.golang.org/protobuf/types/known/timestamppb#hdr-JSON_Mapping-Timestamp
//   - https://protobuf.dev/reference/protobuf/google.protobuf/#timestamp
//   - RFC 3339: https://tools.ietf.org/html/rfc3339
func processWhenUseTimezones(jsonStr string, locationName string) (string, error) {
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
