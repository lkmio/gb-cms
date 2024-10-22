package main

// LocalDB 使用json文件保存设备和通道信息
type LocalDB struct {
}

func (m LocalDB) LoadDevices() []*Device {
	return nil
}

func (m LocalDB) RegisterDevice(device *Device) (error, bool) {
	//持久化...
	device.Status = "ON"

	oldDevice := DeviceManager.Find(device.ID)
	if oldDevice != nil {
		oldDevice.(*Device).Status = "ON"
		oldDevice.(*Device).RemoteAddr = device.RemoteAddr
		oldDevice.(*Device).Name = device.Name
		oldDevice.(*Device).Transport = device.Transport
		device = oldDevice.(*Device)
	} else if err := DeviceManager.Add(device); err != nil {
		return err, false
	}

	return nil, oldDevice == nil || len(device.Channels) == 0
}

func (m LocalDB) UnRegisterDevice(id string) {
	device := DeviceManager.Find(id)
	if device == nil {
		return
	}

	device.(*Device).Status = "OFF"
}

func (m LocalDB) KeepAliveDevice(device *Device) {

}

func (m LocalDB) AddPlatform(record GBPlatformRecord) error {
	//if ExistPlatform(record.SeverID) {
	//	return
	//}

	return nil
}

func (m LocalDB) LoadPlatforms() []GBPlatformRecord {
	//if ExistPlatform(record.SeverID) {
	//	return
	//}

	return nil
}
