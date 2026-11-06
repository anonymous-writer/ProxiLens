package bluez

import (
	"context"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/godbus/dbus/v5"
)

const (
	service = "org.bluez"
	root    = dbus.ObjectPath("/")
)

type Device struct {
	Path             dbus.ObjectPath
	Name             string
	Address          string
	RSSI             int16
	TxPower          int16
	UUIDs            []string
	ManufacturerData map[uint16]string
	ServiceData      map[string]string
}

type Client struct {
	conn *dbus.Conn
	mu   sync.Mutex
	seen map[dbus.ObjectPath]Device
}

func New() (*Client, error) {
	conn, err := dbus.SystemBus()
	if err != nil {
		return nil, err
	}
	return &Client{conn: conn, seen: make(map[dbus.ObjectPath]Device)}, nil
}

func (c *Client) Close() error { return c.conn.Close() }

func (c *Client) StartDiscovery(ctx context.Context) error {
	objects, err := c.getManagedObjects(ctx)
	if err != nil {
		return err
	}
	adapter, err := findAdapter(objects)
	if err != nil {
		return err
	}
	obj := c.conn.Object(service, adapter)
	call := obj.CallWithContext(ctx, "org.bluez.Adapter1.StartDiscovery", 0)
	if call.Err != nil && !strings.Contains(call.Err.Error(), "InProgress") {
		return fmt.Errorf("StartDiscovery: %w", call.Err)
	}
	return nil
}

func (c *Client) StopDiscovery(ctx context.Context) error {
	objects, err := c.getManagedObjects(ctx)
	if err != nil {
		return err
	}
	adapter, err := findAdapter(objects)
	if err != nil {
		return err
	}
	call := c.conn.Object(service, adapter).CallWithContext(ctx, "org.bluez.Adapter1.StopDiscovery", 0)
	if call.Err != nil && !strings.Contains(call.Err.Error(), "NotReady") {
		return call.Err
	}
	return nil
}

func (c *Client) Watch(ctx context.Context, onDevice func(Device)) error {
	if err := c.conn.AddMatchSignal(
		dbus.WithMatchObjectPath(root),
		dbus.WithMatchInterface("org.freedesktop.DBus.ObjectManager"),
	); err != nil {
		return err
	}
	if err := c.conn.AddMatchSignal(
		dbus.WithMatchInterface("org.freedesktop.DBus.Properties"),
		dbus.WithMatchMember("PropertiesChanged"),
	); err != nil {
		return err
	}
	ch := make(chan *dbus.Signal, 128)
	c.conn.Signal(ch)
	defer c.conn.RemoveSignal(ch)

	// Initial snapshot: devices already known to BlueZ.
	objects, err := c.getManagedObjects(ctx)
	if err != nil {
		return err
	}
	for path, ifaces := range objects {
		if d, ok := decodeDevice(path, ifaces["org.bluez.Device1"]); ok {
			c.emit(d, onDevice)
		}
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case sig := <-ch:
			if sig == nil {
				continue
			}
			switch sig.Name {
			case "org.freedesktop.DBus.ObjectManager.InterfacesAdded":
				if len(sig.Body) < 2 {
					continue
				}
				path, ok := sig.Body[0].(dbus.ObjectPath)
				if !ok {
					continue
				}
				ifaces, ok := sig.Body[1].(map[string]map[string]dbus.Variant)
				if !ok {
					continue
				}
				if d, ok := decodeDevice(path, ifaces["org.bluez.Device1"]); ok {
					c.emit(d, onDevice)
				}
			case "org.freedesktop.DBus.Properties.PropertiesChanged":
				if len(sig.Body) < 3 {
					continue
				}
				iface, _ := sig.Body[0].(string)
				if iface != "org.bluez.Device1" {
					continue
				}
				changed, ok := sig.Body[1].(map[string]dbus.Variant)
				if !ok {
					continue
				}
				d := c.current(sig.Path)
				apply(&d, changed)
				if d.Address != "" {
					c.emit(d, onDevice)
				}
			}
		}
	}
}

func (c *Client) emit(d Device, fn func(Device)) {
	c.mu.Lock()
	old, exists := c.seen[d.Path]
	c.seen[d.Path] = d
	c.mu.Unlock()
	if !exists || old.RSSI != d.RSSI || old.Name != d.Name || old.Address != d.Address {
		fn(d)
	}
}

func (c *Client) current(path dbus.ObjectPath) Device {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.seen[path]
}

func (c *Client) getManagedObjects(ctx context.Context) (map[dbus.ObjectPath]map[string]map[string]dbus.Variant, error) {
	var objects map[dbus.ObjectPath]map[string]map[string]dbus.Variant
	err := c.conn.Object(service, root).CallWithContext(ctx,
		"org.freedesktop.DBus.ObjectManager.GetManagedObjects", 0).Store(&objects)
	return objects, err
}

func findAdapter(objects map[dbus.ObjectPath]map[string]map[string]dbus.Variant) (dbus.ObjectPath, error) {
	for path, ifaces := range objects {
		if _, ok := ifaces["org.bluez.Adapter1"]; ok {
			return path, nil
		}
	}
	return "", fmt.Errorf("kein BlueZ-Adapter gefunden; Bluetooth einschalten und Adapter prüfen")
}

func decodeDevice(path dbus.ObjectPath, props map[string]dbus.Variant) (Device, bool) {
	if props == nil {
		return Device{}, false
	}
	d := Device{Path: path, RSSI: -127}
	apply(&d, props)
	return d, d.Address != ""
}

func apply(d *Device, props map[string]dbus.Variant) {
	for key, value := range props {
		switch key {
		case "Name", "Alias":
			if s, ok := value.Value().(string); ok && s != "" {
				d.Name = s
			}
		case "Address":
			if s, ok := value.Value().(string); ok {
				d.Address = s
			}
		case "RSSI", "TxPower":
			var n int16
			switch v := value.Value().(type) {
			case int16:
				n = v
			case int32:
				n = int16(v)
			default:
				continue
			}
			if key == "RSSI" {
				d.RSSI = n
			} else {
				d.TxPower = n
			}
		case "UUIDs":
			if values, ok := value.Value().([]string); ok {
				d.UUIDs = append([]string(nil), values...)
				sort.Strings(d.UUIDs)
			}
		case "ManufacturerData":
			if values, ok := value.Value().(map[uint16]dbus.Variant); ok {
				d.ManufacturerData = make(map[uint16]string, len(values))
				for id, raw := range values {
					if b, ok := raw.Value().([]byte); ok {
						d.ManufacturerData[id] = hex.EncodeToString(b)
					}
				}
			}
		case "ServiceData":
			if values, ok := value.Value().(map[string]dbus.Variant); ok {
				d.ServiceData = make(map[string]string, len(values))
				for uuid, raw := range values {
					if b, ok := raw.Value().([]byte); ok {
						d.ServiceData[uuid] = hex.EncodeToString(b)
					}
				}
			}
		}
	}
}
