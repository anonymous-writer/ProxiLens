package main

import (
	"flag"
	"log"

	"github.com/local/proxilens/internal/app"
	"github.com/local/proxilens/internal/grantbroker"
)

func main() {
	brokerMode := flag.Bool("grant-broker", false, "run the per-user development grant broker")
	flag.Parse()
	if *brokerMode {
		if err := grantbroker.Serve(); err != nil {
			log.Fatal(err)
		}
		return
	}
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
