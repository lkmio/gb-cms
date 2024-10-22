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

func (s *deviceManager) Add(device GBDevice) error {
	_, ok := s.m.LoadOrStore(device.GetID(), device)
	if ok {
		return fmt.Errorf("the device %s has been exist", device.GetID())
	}

	return nil
}

func (s *deviceManager) Find(id string) GBDevice {
	value, ok := s.m.Load(id)
	if ok {
		return value.(GBDevice)
	}

	return nil
}

func (s *deviceManager) Remove(id string) (GBDevice, error) {
	value, loaded := s.m.LoadAndDelete(id)
	if loaded {
		return value.(GBDevice), nil
	}

	return nil, fmt.Errorf("device with id %s was not find", id)
}

func (s *deviceManager) AllDevices() []GBDevice {
	var devices []GBDevice
	s.m.Range(func(key, value any) bool {
		devices = append(devices, value.(GBDevice))
		return true
	})

	return devices
}
