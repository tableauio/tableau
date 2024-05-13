package types

const (
	WellKnownMessageTimestamp = "google.protobuf.Timestamp"
	WellKnownMessageDuration  = "google.protobuf.Duration"
)

var wellKnownMessages map[string]bool

func init() {
	wellKnownMessages = map[string]bool{
		WellKnownMessageTimestamp: true,
		WellKnownMessageDuration:  true,
	}
}

func IsWellKnownMessage(fullTypeName string) bool {
	return wellKnownMessages[fullTypeName]
}
