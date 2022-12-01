package confgen

import (
	"github.com/emirpasic/gods/lists/arraylist"
	"github.com/tableauio/tableau/log"
)

type messagerStatsInfo struct {
	Name         interface{}
	Milliseconds int64
}

func PrintPerfStats(gen *Generator) {
	// print performance stats
	list := arraylist.New()
	gen.PerfStats.Range(func(key, value interface{}) bool {
		list.Add(&messagerStatsInfo{
			Name:         key,
			Milliseconds: value.(int64),
		})
		return true
	})
	list.Sort(func(a, b interface{}) int {
		infoA := a.(*messagerStatsInfo)
		infoB := b.(*messagerStatsInfo)
		return int(infoB.Milliseconds - infoA.Milliseconds)
	})
	list.Each(func(index int, value interface{}) {
		info := value.(*messagerStatsInfo)
		log.Debugf("timespan|%v: %vs", info.Name, float64(info.Milliseconds)/1000)
	})
}
