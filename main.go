package main

import (
	"flag"
	"log"

	_ "modernc.org/sqlite"
)

func main() {
	flag.Parse()

	if len(flag.Args()) < 1 {
		log.Fatalf("No input database provided")
	}

	inputDB := flag.Arg(0)

	model := newModel()
	view := newMainView()
	ctrl := newController(model, view)
	view.setController(ctrl)
	model.setController(ctrl)

	if err := ctrl.openDatabase("sqlite", inputDB); err != nil {
		log.Fatalf("Couldn't open database %s: %v", inputDB, err)
	}

	if err := view.run(); err != nil {
		log.Fatalf("koios failed: %v", err)
	}
}
