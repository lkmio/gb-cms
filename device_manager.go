package main

import (
	"fmt"
	"sync"
)

// DeviceManager 位于内存中的所有设备和通道
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

func (s *deviceManager) Remove(id string) GBDevice {
	value, loaded := s.m.LoadAndDelete(id)
	if loaded {
		return value.(GBDevice)
	}

	return nil
}

func (s *deviceManager) All() []GBDevice {
	var devices []GBDevice
	s.m.Range(func(key, value any) bool {
		devices = append(devices, value.(GBDevice))
		return true
	})

	return devices
}
