package types

import "google.golang.org/protobuf/reflect/protoreflect"

const (
	WellKnownMessageTimestamp  = "google.protobuf.Timestamp"
	WellKnownMessageDuration   = "google.protobuf.Duration"
	WellKnownMessageFraction   = "tableau.Fraction"
	WellKnownMessageComparator = "tableau.Comparator"
)

var wellKnownMessages map[string]string

func init() {
	wellKnownMessages = map[string]string{
		WellKnownMessageTimestamp:  "google/protobuf/timestamp.proto",
		WellKnownMessageDuration:   "google/protobuf/duration.proto",
		WellKnownMessageFraction:   "tableau/protobuf/wellknown.proto",
		WellKnownMessageComparator: "tableau/protobuf/wellknown.proto",
	}
}

// IsWellKnownMessage checks if the given message full name is a well-known
// message.
//
// # Well-known messages
//
//   - google.protobuf.Timestamp
//   - google.protobuf.Duration
//   - tableau.Fraction
//   - tableau.Comparator
func IsWellKnownMessage[T protoreflect.FullName | string](fullTypeName T) bool {
	return wellKnownMessages[string(fullTypeName)] != ""
}

func GetWellKnownMessageImport(fullTypeName string) string {
	return wellKnownMessages[fullTypeName]
}
