package main

type DeviceDB interface {
	LoadDevices() []*Device

	RegisterDevice(device *Device) (error, bool)

	UnRegisterDevice(id string)

	KeepAliveDevice(device *Device)

	LoadPlatforms() []GBPlatformRecord

	AddPlatform(record GBPlatformRecord) error

	//RemovePlatform(record GBPlatformRecord) (GBPlatformRecord, bool)
	//
	//PlatformList() []GBPlatformRecord
	//
	//BindPlatformChannel() bool
	//
	//UnbindPlatformChannel() bool
}
