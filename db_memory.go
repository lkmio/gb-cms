package main

type MemoryDB struct {
}

func (m MemoryDB) LoadDevices() []*DBDevice {
	return nil
}

func (m MemoryDB) AddDevice(device *DBDevice) error {
	//持久化...

	return DeviceManager.Add(device)
}

func (m MemoryDB) RemoveDevice(id string) {
	//
	DeviceManager.Remove(id)
}
