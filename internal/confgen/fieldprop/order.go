package fieldprop

import (
	"cmp"
	"time"

	"github.com/tableauio/tableau/internal/types"
	"github.com/tableauio/tableau/log"
	"github.com/tableauio/tableau/proto/tableaupb"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
)

// CheckOrder checks whether the given field values are ordered. If not
// ordered, it will return the parsed old and new values for debugging.
func CheckOrder(fd protoreflect.FieldDescriptor, oldVal, newVal protoreflect.Value, order tableaupb.Order) (any, any, bool) {
	if !oldVal.IsValid() {
		return nil, nil, true
	}
	switch fd.Kind() {
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind,
		protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		parsedOldVal, parsedNewVal := oldVal.Int(), newVal.Int()
		return parsedOldVal, parsedNewVal, isOrdered(parsedOldVal, parsedNewVal, order)
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind,
		protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		parsedOldVal, parsedNewVal := oldVal.Uint(), newVal.Uint()
		return parsedOldVal, parsedNewVal, isOrdered(parsedOldVal, parsedNewVal, order)
	case protoreflect.FloatKind, protoreflect.DoubleKind:
		parsedOldVal, parsedNewVal := oldVal.Float(), newVal.Float()
		return parsedOldVal, parsedNewVal, isOrdered(parsedOldVal, parsedNewVal, order)
	case protoreflect.StringKind:
		parsedOldVal, parsedNewVal := oldVal.String(), newVal.String()
		return parsedOldVal, parsedNewVal, isOrdered(parsedOldVal, parsedNewVal, order)
	case protoreflect.EnumKind:
		parsedOldVal, parsedNewVal := oldVal.Enum(), newVal.Enum()
		return parsedOldVal, parsedNewVal, isOrdered(parsedOldVal, parsedNewVal, order)
	case protoreflect.MessageKind:
		md := fd.Message()
		msgName := md.FullName()
		switch msgName {
		case types.WellKnownMessageTimestamp:
			parseTime := func(val protoreflect.Value) time.Time {
				msg := val.Message().Interface().(*dynamicpb.Message)
				return time.Unix(msg.Get(md.Fields().ByName("seconds")).Int(), msg.Get(md.Fields().ByName("nanos")).Int())
			}
			oldTime, newTime := parseTime(oldVal), parseTime(newVal)
			return oldTime, newTime, isTimeOrdered(oldTime, newTime, order)

		case types.WellKnownMessageDuration:
			parseDuration := func(val protoreflect.Value) time.Duration {
				msg := val.Message().Interface().(*dynamicpb.Message)
				return time.Second*time.Duration(msg.Get(md.Fields().ByName("seconds")).Int()) + time.Nanosecond*time.Duration(msg.Get(md.Fields().ByName("nanos")).Int())
			}
			oldDuration, newDuration := parseDuration(oldVal), parseDuration(newVal)
			return oldDuration, newDuration, isOrdered(oldDuration, newDuration, order)

		default:
			log.Warnf("not supported to check field prop order of message type: %s", msgName)
			return nil, nil, true
		}
	default:
		log.Warnf("not supported to check field prop order of kind: %s at %s", fd.Kind(), fd.FullName())
		return nil, nil, true
	}
}

func isOrdered[T cmp.Ordered](oldVal, newVal T, order tableaupb.Order) bool {
	switch order {
	case tableaupb.Order_ORDER_ASC:
		return oldVal <= newVal
	case tableaupb.Order_ORDER_DESC:
		return oldVal >= newVal
	case tableaupb.Order_ORDER_STRICTLY_ASC:
		return oldVal < newVal
	case tableaupb.Order_ORDER_STRICTLY_DESC:
		return oldVal > newVal
	default:
		return true
	}
}

func isTimeOrdered(oldVal, newVal time.Time, order tableaupb.Order) bool {
	switch order {
	case tableaupb.Order_ORDER_ASC:
		return !oldVal.After(newVal)
	case tableaupb.Order_ORDER_DESC:
		return !oldVal.Before(newVal)
	case tableaupb.Order_ORDER_STRICTLY_ASC:
		return oldVal.Before(newVal)
	case tableaupb.Order_ORDER_STRICTLY_DESC:
		return oldVal.After(newVal)
	default:
		return true
	}
}
