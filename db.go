package main

type GB28181DB interface {
	LoadOnlineDevices() (map[string]*Device, error)

	LoadDevices() (map[string]*Device, error)

	SaveDevice(device *Device) error

	SaveChannel(deviceId string, channel *Channel) error

	UpdateDeviceStatus(deviceId string, status OnlineStatus) error

	UpdateChannelStatus(channelId, status string) error

	RefreshHeartbeat(deviceId string) error

	QueryDevice(id string) (*Device, error)

	QueryDevices(page int, size int) ([]*Device, int, error)

	QueryChannel(deviceId string, channelId string) (*Channel, error)

	QueryChannels(deviceId string, page, size int) ([]*Channel, int, error)

	LoadPlatforms() ([]*GBPlatformRecord, error)

	QueryPlatform(id string) (*GBPlatformRecord, error)

	SavePlatform(platform *GBPlatformRecord) error

	DeletePlatform(platform *GBPlatformRecord) error

	UpdatePlatform(platform *GBPlatformRecord) error

	UpdatePlatformStatus(serverId string, status OnlineStatus) error

	BindChannels(id string, channels [][2]string) ([][2]string, error)

	UnbindChannels(id string, channels [][2]string) ([][2]string, error)

	// QueryPlatformChannel 查询级联设备的某个通道, 返回通道所属设备ID、通道.
	QueryPlatformChannel(platformId string, channelId string) (string, *Channel, error)

	LoadStreams() (map[string]*Stream, error)

	SaveStream(stream *Stream) error

	DeleteStream(time int64) error

	//QueryStream(pate int, size int)

	// QueryForwardSink 查询转发流Sink
	QueryForwardSink(stream StreamID, sink string) (*Sink, error)

	QueryForwardSinks(stream StreamID) (map[string]*Sink, error)

	// SaveForwardSink 保存转发流Sink
	SaveForwardSink(stream StreamID, sink *Sink) error

	DeleteForwardSink(stream StreamID, sink string) error

	DeleteForwardSinks(stream StreamID) error

	Del(key string) error
}
