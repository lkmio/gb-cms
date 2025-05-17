package main

type DaoPlatform interface {
	LoadPlatforms() ([]*SIPUAParams, error)

	QueryPlatform(addr string) (*SIPUAParams, error)

	SavePlatform(platform *SIPUAParams) error

	DeletePlatform(addr string) error

	UpdatePlatform(platform *SIPUAParams) error

	UpdatePlatformStatus(addr string, status OnlineStatus) error

	BindChannels(addr string, channels [][2]string) ([][2]string, error)

	UnbindChannels(addr string, channels [][2]string) ([][2]string, error)

	// QueryPlatformChannel 查询级联设备的某个通道, 返回通道所属设备ID、通道.
	QueryPlatformChannel(addr string, channelId string) (string, *Channel, error)

	QueryPlatformChannels(addr string) ([]*Channel, error)
}

type daoPlatform struct {
}

func (d *daoPlatform) LoadPlatforms() ([]*SIPUAParams, error) {
	var platforms []*SIPUAParams
	tx := db.Find(&platforms)
	if tx.Error != nil {
		return nil, tx.Error
	}

	return platforms, nil
}

func (d *daoPlatform) QueryPlatform(addr string) (*SIPUAParams, error) {
	var platform SIPUAParams
	tx := db.Where("server_addr =?", addr).First(&platform)
	if tx.Error != nil {
		return nil, tx.Error
	}

	return &platform, nil
}

func (d *daoPlatform) SavePlatform(platform *SIPUAParams) error {
	var old SIPUAParams
	tx := db.Where("server_addr =?", platform.ServerAddr).First(&old)
	if tx.Error == nil {
		platform.ID = old.ID
	}
	return db.Save(platform).Error
}

func (d *daoPlatform) DeletePlatform(addr string) error {
	return db.Where("server_addr =?", addr).Unscoped().Delete(&SIPUAParams{}).Error
}

func (d *daoPlatform) UpdatePlatform(platform *SIPUAParams) error {
	//TODO implement me
	panic("implement me")
}

func (d *daoPlatform) UpdatePlatformStatus(addr string, status OnlineStatus) error {
	return db.Model(&SIPUAParams{}).Where("server_addr =?", addr).Update("status", status).Error
}

type DBPlatformChannel struct {
	GBModel
	DeviceID   string `json:"device_id"`
	Channel    string `json:"channel_id"`
	ServerAddr string `json:"server_addr"`
}

func (d *DBPlatformChannel) TableName() string {
	return "lkm_platform_channel"
}

func (d *daoPlatform) BindChannels(addr string, channels [][2]string) ([][2]string, error) {
	var res [][2]string
	for _, channel := range channels {

		var old DBPlatformChannel
		_ = db.Where("device_id =? and channel_id =? and server_addr =?", channel[0], channel[1], addr).First(&old)
		if old.ID == 0 {
			_ = db.Create(&DBPlatformChannel{
				DeviceID: channel[0],
				Channel:  channel[1],
			})
		}
		res = append(res, channel)
	}

	return res, nil
}

func (d *daoPlatform) UnbindChannels(addr string, channels [][2]string) ([][2]string, error) {
	var res [][2]string
	for _, channel := range channels {
		tx := db.Unscoped().Delete(&DBPlatformChannel{}, "device_id =? and channel_id =? and server_addr =?", channel[0], channel[1], addr)
		if tx.Error == nil {
			res = append(res, channel)
		} else {
			Sugar.Errorf("解绑级联设备通道失败. device_id: %s, channel_id: %s err: %s", channel[0], channel[1], tx.Error)
		}
	}

	return res, nil
}

func (d *daoPlatform) QueryPlatformChannel(addr string, channelId string) (string, *Channel, error) {
	var platformChannel DBPlatformChannel
	tx := db.Model(&DBPlatformChannel{}).Where("channel_id =? and server_addr =?", channelId, addr).First(&platformChannel)
	if tx.Error != nil {
		return "", nil, tx.Error
	}

	var channel Channel
	tx = db.Where("device_id =? and channel_id =?", platformChannel.DeviceID, platformChannel.Channel).First(&channel)
	if tx.Error != nil {
		return "", nil, tx.Error
	}

	return platformChannel.DeviceID, &channel, nil
}

func (d *daoPlatform) QueryPlatformChannels(addr string) ([]*Channel, error) {
	var platformChannels []*DBPlatformChannel
	tx := db.Where("server_addr =?", addr).Find(&platformChannels)
	if tx.Error != nil {
		return nil, tx.Error
	}

	var channels []*Channel
	for _, platformChannel := range platformChannels {
		var channel Channel
		tx = db.Where("device_id =? and channel_id =?", platformChannel.DeviceID, platformChannel.Channel).First(&channel)
		if tx.Error == nil {
			channels = append(channels, &channel)
		} else {
			Sugar.Errorf("查询级联设备通道失败. device_id: %s, channel_id: %s err: %s", platformChannel.DeviceID, platformChannel.Channel, tx.Error)
		}

	}

	return channels, nil
}
