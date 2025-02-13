package xproto

import (
	"sync"

	"github.com/tableauio/tableau/proto/tableaupb"
	"github.com/tableauio/tableau/xerrors"
	"google.golang.org/protobuf/proto"
	pref "google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

type EnumCache struct {
	sync.Mutex
	enums map[pref.FullName]*enumAliasInfo
}

func (ec *EnumCache) GetValueByAlias(ed pref.EnumDescriptor, valueAlias string) (pref.Value, error) {
	ec.Lock()
	aliasInfo, ok := ec.enums[ed.FullName()]
	if !ok {
		// create and insert if not existed
		aliasInfo = &enumAliasInfo{}
		ec.enums[ed.FullName()] = aliasInfo
	}
	ec.Unlock()

	var onceErr error
	aliasInfo.once.Do(func() {
		aliasInfo.values = make(map[string]pref.Value)
		for i := 0; i < ed.Values().Len(); i++ {
			// get enum value descriptor
			evd := ed.Values().Get(i)
			opts := evd.Options().(*descriptorpb.EnumValueOptions)
			evalueOpts := proto.GetExtension(opts, tableaupb.E_Evalue).(*tableaupb.EnumValueOptions)
			if evalueOpts != nil {
				existedVal, ok := aliasInfo.values[evalueOpts.Name]
				if ok {
					existedEvd := ed.Values().ByNumber(existedVal.Enum())
					onceErr = xerrors.E2021(ed.FullName(), existedEvd.Name(), evd.Name(), valueAlias)
					return
				}
				aliasInfo.values[evalueOpts.Name] = pref.ValueOfEnum(evd.Number())
			}
		}
	})

	if onceErr != nil {
		return DefaultEnumValue, onceErr
	}

	v, ok := aliasInfo.values[valueAlias]
	if !ok {
		return DefaultEnumValue, xerrors.E2006(valueAlias, ed.FullName())
	}
	return v, nil
}

type enumAliasInfo struct {
	once   sync.Once
	values map[string]pref.Value // alias -> pref.Value
}

var enumCache *EnumCache

func init() {
	enumCache = &EnumCache{
		enums: make(map[pref.FullName]*enumAliasInfo),
	}
}
