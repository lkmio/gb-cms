package main

import (
	"fmt"
	"sync"
)

var DeviceManager *deviceManager

func init() {
	DeviceManager = &deviceManager{}
}

type deviceManager struct {
	m sync.Map
}

func (s *deviceManager) Add(device *DBDevice) error {
	_, ok := s.m.LoadOrStore(device.Id, device)
	if ok {
		return fmt.Errorf("the device %s has been exist", device.Id)
	}

	return nil
}

func (s *deviceManager) Find(id string) *DBDevice {
	value, ok := s.m.Load(id)
	if ok {
		return value.(*DBDevice)
	}

	return nil
}

func (s *deviceManager) Remove(id string) (*DBDevice, error) {
	value, loaded := s.m.LoadAndDelete(id)
	if loaded {
		return value.(*DBDevice), nil
	}

	return nil, fmt.Errorf("device with id %s was not find", id)
}

func (s *deviceManager) AllDevices() []DBDevice {
	var devices []DBDevice
	s.m.Range(func(key, value any) bool {
		devices = append(devices, *value.(*DBDevice))
		return true
	})

	return devices
}
