package xproto

var WellKnownMessages map[string]int

func init() {
	WellKnownMessages = map[string]int{
		"google.protobuf.Timestamp": 1,
		"google.protobuf.Duration":  1,
	}
}
