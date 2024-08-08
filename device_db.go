package main

type DeviceDB interface {
	LoadDevices() []*DBDevice

	RegisterDevice(device *DBDevice) (error, bool)

	UnRegisterDevice(id string)

	KeepAliveDevice(device *DBDevice)
}
