package fieldprop

import (
	"cmp"
	"time"

	"github.com/tableauio/tableau/internal/types"
	"github.com/tableauio/tableau/proto/tableaupb"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
)

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
		// TODO: also support check WellKnownMessageDuration
		case types.WellKnownMessageTimestamp:
			oldMsg := oldVal.Message().Interface().(*dynamicpb.Message)
			newMsg := newVal.Message().Interface().(*dynamicpb.Message)
			oldTime := time.Unix(oldMsg.Get(md.Fields().ByName("seconds")).Int(), oldMsg.Get(md.Fields().ByName("nanos")).Int())
			newTime := time.Unix(newMsg.Get(md.Fields().ByName("seconds")).Int(), newMsg.Get(md.Fields().ByName("nanos")).Int())
			return oldTime, newTime, isTimeOrdered(oldTime, newTime, order)
		}
	}
	return nil, nil, true
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
