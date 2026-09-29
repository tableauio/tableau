package store

import (
	"strings"
	"time"

	"github.com/tableauio/tableau/internal/types"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type protoJSONOptions struct {
	protojson.MarshalOptions
	location *time.Location
}

// Marshal delegates protobuf JSON behavior to protojson while presenting each
// Timestamp as a StringValue containing the requested offset. The reflection
// view changes only values seen by this call; it does not mutate msg.
func (o protoJSONOptions) Marshal(msg proto.Message) ([]byte, error) {
	if o.location == nil || msg == nil {
		return o.MarshalOptions.Marshal(msg)
	}
	message := msg.ProtoReflect()
	timestampTypes := make(map[protoreflect.FullName]bool)
	if !message.IsValid() || !collectTimestampTypes(message.Descriptor(), timestampTypes) {
		return o.MarshalOptions.Marshal(msg)
	}

	state := timestampMarshalState{location: o.location, timestampTypes: timestampTypes}
	wrapped := timestampProtoMessage{message: state.wrapMessage(message)}
	options := o.MarshalOptions
	allowPartial := options.AllowPartial
	options.AllowPartial = true
	out, err := options.Marshal(wrapped)
	if state.err != nil {
		return nil, state.err
	}
	if err != nil {
		return nil, err
	}
	if !allowPartial {
		if err := proto.CheckInitialized(msg); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// collectTimestampTypes records message types that can reach a Timestamp or an
// extension range. A reverse graph propagates matches through recursive types
// without treating a descriptor cycle itself as a match.
func collectTimestampTypes(root protoreflect.MessageDescriptor, matches map[protoreflect.FullName]bool) bool {
	parents := make(map[protoreflect.FullName][]protoreflect.FullName)
	visited := make(map[protoreflect.FullName]bool)
	queued := make(map[protoreflect.FullName]bool)
	queue := make([]protoreflect.FullName, 0, 1)
	enqueue := func(name protoreflect.FullName) {
		matches[name] = true
		if !queued[name] {
			queued[name] = true
			queue = append(queue, name)
		}
	}

	var collect func(protoreflect.MessageDescriptor)
	collect = func(message protoreflect.MessageDescriptor) {
		name := message.FullName()
		if visited[name] {
			return
		}
		visited[name] = true
		if matches[name] || name == types.WellKnownMessageTimestamp || message.ExtensionRanges().Len() != 0 {
			enqueue(name)
		}
		fields := message.Fields()
		for i := 0; i < fields.Len(); i++ {
			field := fields.Get(i)
			var child protoreflect.MessageDescriptor
			if field.IsMap() {
				child = field.MapValue().Message()
			} else {
				child = field.Message()
			}
			if child != nil {
				parents[child.FullName()] = append(parents[child.FullName()], name)
				collect(child)
			}
		}
	}
	collect(root)

	for i := 0; i < len(queue); i++ {
		name := queue[i]
		for _, parent := range parents[name] {
			enqueue(parent)
		}
	}
	return matches[root.FullName()]
}

type timestampProtoMessage struct {
	message protoreflect.Message
}

func (m timestampProtoMessage) ProtoReflect() protoreflect.Message {
	return m.message
}

type timestampMarshalState struct {
	location       *time.Location
	timestampTypes map[protoreflect.FullName]bool
	err            error
}

func (s *timestampMarshalState) wrapMessage(message protoreflect.Message) protoreflect.Message {
	if !message.IsValid() {
		return message
	}
	if !s.timestampTypes[message.Descriptor().FullName()] {
		return message
	}
	if message.Descriptor().FullName() != types.WellKnownMessageTimestamp {
		return timestampMessage{Message: message, state: s}
	}
	formatted, err := s.formatTimestamp(message)
	if err != nil && s.err == nil {
		s.err = err
	}
	return wrapperspb.String(formatted).ProtoReflect()
}

func (s *timestampMarshalState) wrapValue(fd protoreflect.FieldDescriptor, value protoreflect.Value) protoreflect.Value {
	if !value.IsValid() {
		return value
	}
	if fd.IsExtension() && fd.Message() != nil {
		collectTimestampTypes(fd.Message(), s.timestampTypes)
	}
	if fd.IsMap() {
		message := fd.MapValue().Message()
		if message == nil || !s.timestampTypes[message.FullName()] {
			return value
		}
		return protoreflect.ValueOfMap(timestampMap{Map: value.Map(), state: s})
	}
	if fd.IsList() {
		message := fd.Message()
		if message == nil || !s.timestampTypes[message.FullName()] {
			return value
		}
		return protoreflect.ValueOfList(timestampList{List: value.List(), state: s})
	}
	if fd.Message() != nil {
		return protoreflect.ValueOfMessage(s.wrapMessage(value.Message()))
	}
	return value
}

func (s *timestampMarshalState) formatTimestamp(message protoreflect.Message) (string, error) {
	fields := message.Descriptor().Fields()
	seconds := message.Get(fields.ByNumber(1)).Int()
	nanos := message.Get(fields.ByNumber(2)).Int()
	if seconds < minTimestampSeconds || seconds > maxTimestampSeconds || nanos < 0 || nanos > 999999999 {
		_, err := protojson.Marshal(message.Interface())
		return "", err
	}
	utc := time.Unix(seconds, nanos).UTC()
	local := utc.In(s.location)
	if local.Year() < 1 || local.Year() > 9999 {
		return formatUTCTimestamp(utc), nil
	}
	return local.Format(time.RFC3339Nano), nil
}

type timestampMessage struct {
	protoreflect.Message
	state *timestampMarshalState
}

func (m timestampMessage) Range(yield func(protoreflect.FieldDescriptor, protoreflect.Value) bool) {
	m.Message.Range(func(fd protoreflect.FieldDescriptor, value protoreflect.Value) bool {
		return yield(fd, m.state.wrapValue(fd, value))
	})
}

func (m timestampMessage) Get(fd protoreflect.FieldDescriptor) protoreflect.Value {
	return m.state.wrapValue(fd, m.Message.Get(fd))
}

type timestampList struct {
	protoreflect.List
	state *timestampMarshalState
}

func (l timestampList) Get(index int) protoreflect.Value {
	return protoreflect.ValueOfMessage(l.state.wrapMessage(l.List.Get(index).Message()))
}

type timestampMap struct {
	protoreflect.Map
	state *timestampMarshalState
}

func (m timestampMap) Range(yield func(protoreflect.MapKey, protoreflect.Value) bool) {
	m.Map.Range(func(key protoreflect.MapKey, value protoreflect.Value) bool {
		return yield(key, protoreflect.ValueOfMessage(m.state.wrapMessage(value.Message())))
	})
}

func (m timestampMap) Get(key protoreflect.MapKey) protoreflect.Value {
	value := m.Map.Get(key)
	if !value.IsValid() {
		return value
	}
	return protoreflect.ValueOfMessage(m.state.wrapMessage(value.Message()))
}

const (
	maxTimestampSeconds = 253402300799
	minTimestampSeconds = -62135596800
)

func formatUTCTimestamp(timestamp time.Time) string {
	formatted := timestamp.Format("2006-01-02T15:04:05.000000000")
	formatted = strings.TrimSuffix(formatted, "000")
	formatted = strings.TrimSuffix(formatted, "000")
	formatted = strings.TrimSuffix(formatted, ".000")
	return formatted + "Z"
}
