package main

func (d *DBDevice) OnCatalog(response *QueryCatalogResponse) {
	if d.Channels == nil {
		d.Channels = make(map[string]Channel, 5)
	}

	for index := range response.DeviceList.Devices {
		device := response.DeviceList.Devices[index]
		d.Channels[device.DeviceID] = device
	}
}
