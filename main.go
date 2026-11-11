package main

import (
	"log"

	"github.com/local/proxilens/internal/app"
)

func main() {
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
