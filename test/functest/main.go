package main

import (
	"log"
)

func main() {
	CompareGeneratedProto()
	CompareGeneratedJSON()
}

func CompareGeneratedProto() {
	err := genProto("DEBUG", "FULL")
	if err != nil {
		log.Fatalf("%+v", err)
	}
	err = EqualTextFile(".proto", "proto", "_proto", 2)
	if err != nil {
		log.Fatal(err)
	}
}

func CompareGeneratedJSON() {
	err := genConf("DEBUG", "FULL")
	if err != nil {
		log.Fatalf("%+v", err)
	}
	err = EqualTextFile(".json", "conf", "_conf", 1)
	if err != nil {
		log.Fatal(err)
	}
}
