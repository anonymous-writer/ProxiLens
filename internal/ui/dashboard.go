package ui

import (
	"context"
	"fmt"
	stdcolor "image/color"
	"sort"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/local/proxilens/internal/bluez"
)

type Dashboard struct {
	app    fyne.App
	win    fyne.Window
	client *bluez.Client
	ctx    context.Context
	cancel context.CancelFunc

	mu       sync.Mutex
	devices  map[string]bluez.Device
	rows     *widget.Table
	status   *widget.Label
	count    *widget.Label
	lastScan *widget.Label
}

func Run() error {
	a := app.NewWithID("de.local.proxilens")
	a.Settings().SetTheme(leaseLensTheme{Theme: theme.DarkTheme()})
	d := &Dashboard{app: a, win: a.NewWindow("ProxiLens · Bluetooth Proximity"), devices: make(map[string]bluez.Device)}
	d.win.Resize(fyne.NewSize(1040, 720))
	d.build()
	d.win.SetCloseIntercept(func() { d.stop(); d.win.Close() })
	d.win.ShowAndRun()
	return nil
}

func (d *Dashboard) build() {
	title := canvas.NewText("ProxiLens", color("#edf3ff"))
	title.TextSize = 30
	title.TextStyle.Bold = true
	subtitle := canvas.NewText("Bluetooth Proximity Dashboard", color("#a8bad2"))
	subtitle.TextSize = 12
	brand := container.NewVBox(title, subtitle)
	statusDot := canvas.NewCircle(color("#f5bd4f"))
	statusDot.Resize(fyne.NewSize(10, 10))
	d.status = widget.NewLabel("Bluetooth nicht gestartet")
	d.status.TextStyle = fyne.TextStyle{Bold: true}
	header := container.NewBorder(nil, nil, nil, container.NewHBox(statusDot, d.status), brand)
	headerBox := panel(container.NewPadded(header))

	d.count = widget.NewLabel("0")
	d.lastScan = widget.NewLabel("Noch kein Scan")
	kpi := container.NewGridWithColumns(3,
		statCard("Gefundene Geräte", d.count),
		statCard("Überwachung", widget.NewLabel("BLE-Scan")),
		statCard("Sitzung", widget.NewLabel("Nicht verändert")),
	)

	d.rows = widget.NewTable(
		func() (int, int) { d.mu.Lock(); defer d.mu.Unlock(); return len(d.sortedDevices()) + 1, 4 },
		func() fyne.CanvasObject { return widget.NewLabel("Gerät") },
		func(id widget.TableCellID, obj fyne.CanvasObject) {
			label := obj.(*widget.Label)
			if id.Row == 0 {
				headers := []string{"GERÄT / NAME", "BLUETOOTH-ADRESSE", "RSSI", "STATUS"}
				label.Text = headers[id.Col]
				label.TextStyle = fyne.TextStyle{Bold: true}
				return
			}
			d.mu.Lock()
			items := d.sortedDevices()
			d.mu.Unlock()
			if id.Row-1 >= len(items) {
				label.SetText("")
				return
			}
			dev := items[id.Row-1]
			switch id.Col {
			case 0:
				label.SetText(displayName(dev))
			case 1:
				label.SetText(dev.Address)
			case 2:
				if dev.RSSI == -127 {
					label.SetText("—")
				} else {
					label.SetText(fmt.Sprintf("%d dBm", dev.RSSI))
				}
			case 3:
				label.SetText("Erkannt")
			}
		},
	)
	d.rows.SetColumnWidth(0, 260)
	d.rows.SetColumnWidth(1, 210)
	d.rows.SetColumnWidth(2, 100)
	d.rows.SetColumnWidth(3, 120)

	start := widget.NewButton("Bluetooth-Scan starten", d.start)
	start.Importance = widget.HighImportance
	stop := widget.NewButton("Scan stoppen", d.stop)
	refresh := widget.NewButton("Liste aktualisieren", func() { d.rows.Refresh() })
	actions := container.NewHBox(start, stop, refresh)
	note := widget.NewLabel("Hinweis: Die Erkennung von iPhone und Apple Watch wird zunächst empirisch geprüft. RSSI ist kein Authentifizierungsnachweis; diese Version sperrt oder entsperrt den Rechner nicht.")
	note.Wrapping = fyne.TextWrapWord
	note.TextStyle = fyne.TextStyle{Italic: true}
	content := container.NewBorder(
		container.NewVBox(headerBox, kpi, actions, widget.NewSeparator(), widget.NewLabel("Sichtbare Bluetooth-LE-Geräte")),
		container.NewVBox(widget.NewSeparator(), d.lastScan, note),
		nil, nil, d.rows,
	)
	d.win.SetContent(container.NewPadded(content))
}

func (d *Dashboard) start() {
	if d.cancel != nil {
		return
	}
	client, err := bluez.New()
	if err != nil {
		d.status.SetText("BlueZ-Fehler: " + err.Error())
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	d.client, d.ctx, d.cancel = client, ctx, cancel
	if err := client.StartDiscovery(ctx); err != nil {
		d.status.SetText("Scan fehlgeschlagen: " + err.Error())
		cancel()
		client.Close()
		d.cancel = nil
		return
	}
	d.status.SetText("BLE-Scan läuft")
	d.lastScan.SetText("Scan gestartet · " + time.Now().Format("15:04:05"))
	go func() {
		err := client.Watch(ctx, func(dev bluez.Device) {
			fyne.Do(func() {
				d.mu.Lock()
				d.devices[dev.Address] = dev
				n := len(d.devices)
				d.mu.Unlock()
				d.count.SetText(fmt.Sprintf("%d", n))
				d.rows.Refresh()
				d.lastScan.SetText("Letztes Signal · " + time.Now().Format("15:04:05"))
			})
		})
		if err != nil && ctx.Err() == nil {
			fyne.Do(func() { d.status.SetText("D-Bus-Fehler: " + err.Error()) })
		}
	}()
}

func (d *Dashboard) stop() {
	if d.cancel == nil {
		return
	}
	d.cancel()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = d.client.StopDiscovery(ctx)
	_ = d.client.Close()
	d.cancel = nil
	d.status.SetText("Scan gestoppt")
}

func (d *Dashboard) sortedDevices() []bluez.Device {
	items := make([]bluez.Device, 0, len(d.devices))
	for _, dev := range d.devices {
		items = append(items, dev)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items
}

func displayName(d bluez.Device) string {
	if d.Name != "" {
		return d.Name
	}
	return "Unbekanntes BLE-Gerät"
}

func statCard(caption string, value fyne.CanvasObject) fyne.CanvasObject {
	label := widget.NewLabel(caption)
	label.TextStyle = fyne.TextStyle{Italic: true}
	return panel(container.NewPadded(container.NewVBox(label, value)))
}

func panel(obj fyne.CanvasObject) fyne.CanvasObject {
	bg := canvas.NewRectangle(color("#16263b"))
	bg.StrokeColor = color("#29405c")
	bg.StrokeWidth = 1
	return container.NewMax(bg, obj)
}

func color(hex string) stdcolor.Color {
	// Do not call theme.ForegroundColor() here: it consults the active
	// application theme, which calls this helper again and causes recursion.
	if len(hex) == 7 && hex[0] == '#' {
		var r, g, b uint8
		if _, err := fmt.Sscanf(hex, "#%02x%02x%02x", &r, &g, &b); err == nil {
			return stdcolor.NRGBA{R: r, G: g, B: b, A: 255}
		}
	}
	return stdcolor.NRGBA{R: 237, G: 243, B: 255, A: 255}
}

type leaseLensTheme struct{ fyne.Theme }

func (t leaseLensTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) stdcolor.Color {
	switch name {
	case theme.ColorNameBackground:
		return color("#0d1929")
	case theme.ColorNameForeground:
		return color("#edf3ff")
	case theme.ColorNamePrimary:
		return color("#58a9ff")
	case theme.ColorNameButton:
		return color("#16263b")
	case theme.ColorNameInputBackground:
		return color("#16263b")
	case theme.ColorNameHover:
		return color("#29405c")
	}
	return t.Theme.Color(name, variant)
}
