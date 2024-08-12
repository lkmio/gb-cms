package main

// LocalDB 使用json文件保存设备和通道信息
type LocalDB struct {
}

func (m LocalDB) LoadDevices() []*DBDevice {
	return nil
}

func (m LocalDB) RegisterDevice(device *DBDevice) (error, bool) {
	//持久化...
	device.Status = "ON"

	d := DeviceManager.Find(device.Id)
	if d != nil {
		d.Status = "ON"
		d.RemoteAddr = device.RemoteAddr
		d.Name = device.Name
		d.Transport = device.Transport
	} else if err := DeviceManager.Add(device); err != nil {
		return err, false
	}

	return nil, d == nil || len(d.Channels) == 0
}

func (m LocalDB) UnRegisterDevice(id string) {
	device := DeviceManager.Find(id)
	if device == nil {
		return
	}

	device.Status = "OFF"
}

func (m LocalDB) KeepAliveDevice(device *DBDevice) {

}
