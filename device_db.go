package main

type DeviceDB interface {
	LoadDevices() []*DBDevice

	AddDevice(device *DBDevice) error

	RemoveDevice(id string)
}
