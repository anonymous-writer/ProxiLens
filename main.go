package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/local/proxilens/internal/app"
	"github.com/local/proxilens/internal/grantbroker"
	"github.com/local/proxilens/internal/policyservice"
)

func main() {
	brokerMode := flag.Bool("grant-broker", false, "run the per-user development grant broker")
	policyServiceMode := flag.Bool("policy-probe-service", false, "run the root observation-only policy probe service")
	policyProbe := flag.Bool("policy-probe", false, "query the local observation-only policy probe service")
	flag.Parse()
	if *brokerMode {
		if err := grantbroker.Serve(); err != nil {
			log.Fatal(err)
		}
		return
	}
	if *policyServiceMode {
		if err := policyservice.Serve(); err != nil {
			log.Fatal(err)
		}
		return
	}
	if *policyProbe {
		out, err := policyservice.Probe()
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(out)
		return
	}
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
