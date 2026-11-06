package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/local/proxilens/internal/bluez"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	client, err := bluez.New()
	if err != nil {
		log.Fatalf("BlueZ D-Bus nicht erreichbar: %v", err)
	}
	defer client.Close()

	log.Println("ProxiLens – BlueZ-Scan startet. Abbruch mit Ctrl+C.")
	if err := client.StartDiscovery(ctx); err != nil {
		log.Fatalf("Discovery konnte nicht gestartet werden: %v", err)
	}
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := client.StopDiscovery(cleanupCtx); err != nil {
			log.Printf("Discovery konnte nicht sauber beendet werden: %v", err)
		}
	}()

	if err := client.Watch(ctx, func(d bluez.Device) {
		log.Printf("BLE-Gerät: %-24s RSSI: %4d dBm TxPower: %4d dBm Adresse: %s UUIDs: %v Herstellerdaten: %v Servicedaten: %v",
			d.Name, d.RSSI, d.TxPower, d.Address, d.UUIDs, d.ManufacturerData, d.ServiceData)
	}); err != nil && ctx.Err() == nil {
		log.Fatalf("D-Bus-Überwachung fehlgeschlagen: %v", err)
	}
}
