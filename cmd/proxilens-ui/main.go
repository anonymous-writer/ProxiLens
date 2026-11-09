package main

import (
	"github.com/local/proxilens/internal/ui"
	"log"
)

func main() {
	if err := ui.Run(); err != nil {
		log.Fatal(err)
	}
}
