package types

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

func IsWellKnownMessage(fullTypeName string) bool {
	return wellKnownMessages[fullTypeName] != ""
}

func GetWellKnownMessageImport(fullTypeName string) string {
	return wellKnownMessages[fullTypeName]
}
