package testutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tableauio/tableau/store"
	"google.golang.org/protobuf/proto"
)

// AssertProtoJSONEq asserts that two proto messages are equivalent in protojson format.
func AssertProtoJSONEq(t *testing.T, expected, actual proto.Message, msgAndArgs ...any) {
	expectedJSON, err := store.MarshalToJSON(expected, &store.MarshalOptions{})
	assert.NoError(t, err)
	actualJSON, err := store.MarshalToJSON(actual, &store.MarshalOptions{})
	assert.NoError(t, err)
	assert.JSONEq(t, string(expectedJSON), string(actualJSON), msgAndArgs...)
}

// AssertProtoJSONEqf asserts that two proto messages are equivalent in protojson format.
func AssertProtoJSONEqf(t *testing.T, expected, actual proto.Message, msg string, args ...any) {
	AssertProtoJSONEq(t, expected, actual, append([]any{msg}, args...)...)
}
