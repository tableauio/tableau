package main

import (
	"log"

	"github.com/tableauio/tableau/xerrors"
)

func main() {
	CompareGeneratedProto()
	CompareGeneratedJSON()
}

func CompareGeneratedProto() {
	err := genProto("DEBUG")
	if err != nil {
		log.Fatalf("%+v", err)
		log.Fatalf("%s", xerrors.NewDesc(err))
	}
	err = EqualTextFile(".proto", "proto", "_proto", 2)
	if err != nil {
		log.Fatal(err)
	}
}

func CompareGeneratedJSON() {
	err := genConf("DEBUG")
	if err != nil {
		log.Fatalf("%+v", err)
		log.Fatalf("%s", xerrors.NewDesc(err))
	}
	err = EqualTextFile(".json", "conf", "_conf", 1)
	if err != nil {
		log.Fatal(err)
	}
}
