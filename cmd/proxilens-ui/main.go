package main

import (
    "log"
    "github.com/local/proxilens/internal/ui"
)

func main() {
    if err := ui.Run(); err != nil { log.Fatal(err) }
}
