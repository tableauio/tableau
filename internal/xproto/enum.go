package xproto

import (
	"sync"

	"github.com/tableauio/tableau/proto/tableaupb"
	"google.golang.org/protobuf/proto"
	pref "google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

type EnumCache struct {
	sync.Mutex
	enums map[pref.FullName]*enumAlias
}

func (ec *EnumCache) GetValueByAlias(ed pref.EnumDescriptor, valueAlias string) (pref.Value, bool) {
	enumCache.Lock()
	alias, ok := enumCache.enums[ed.FullName()]
	if !ok {
		// put in cache
		alias = &enumAlias{
			values: make(map[string]pref.Value),
		}
		for i := 0; i < ed.Values().Len(); i++ {
			// get enum value descriptor
			evd := ed.Values().Get(i)
			opts := evd.Options().(*descriptorpb.EnumValueOptions)
			evalueOpts := proto.GetExtension(opts, tableaupb.E_Evalue).(*tableaupb.EnumValueOptions)
			if evalueOpts != nil {
				alias.values[evalueOpts.Name] = pref.ValueOfEnum(evd.Number())
			}
		}
		enumCache.enums[ed.FullName()] = alias
	}
	enumCache.Unlock()

	v, ok := alias.values[valueAlias]
	if ok {
		return v, true
	}
	return DefaultEnumValue, false
}

type enumAlias struct {
	values map[string]pref.Value // alias -> pref.Value
}

var enumCache *EnumCache

func init() {
	enumCache = &EnumCache{
		enums: make(map[pref.FullName]*enumAlias),
	}
}
