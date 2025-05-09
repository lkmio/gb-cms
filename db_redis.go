package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	RedisKeyDevices        = "devices"      // 使用map保存所有设备信息(不包含通道信息)
	RedisKeyDevicesSort    = "devices_sort" // 使用zset有序保存所有设备ID(按照入库时间)
	RedisKeyChannels       = "channels"     // 使用map保存所有通道信息
	RedisKeyDeviceChannels = "%s_channels"  // 使用zset保存设备下的所有通道ID
	RedisKeyPlatforms      = "platforms"    // 使用zset有序保存所有级联设备
	RedisUniqueChannelID   = "%s_%s"        // 通道号的唯一ID, 设备_通道号

	// RedisKeyStreams 保存推拉流信息, 主要目的是程序崩溃重启后，恢复国标流的invite会话. 如果需要统计所有详细的推拉流信息，需要自行实现.
	RedisKeyStreams     = "streams"  //// 保存所有推流端信息
	RedisKeySinks       = "sinks"    //// 保存所有拉流端信息
	RedisKeyStreamSinks = "%s_sinks" //// 某路流下所有的拉流端

	RedisKeyDialogs      = "streams"
	RedisKeyForwardSinks = "forward_%s"
)

type RedisDB struct {
	utils         *RedisUtils
	platformsLock sync.Mutex
}

type ChannelKey string

func (c ChannelKey) Device() string {
	return strings.Split(string(c), "_")[0]
}

func (c ChannelKey) Channel() string {
	return strings.Split(string(c), "_")[1]
}

func (c ChannelKey) String() string {
	return string(c)
}

// DeviceChannelsKey 返回设备通道列表的主键
func DeviceChannelsKey(id string) string {
	return fmt.Sprintf(RedisKeyDeviceChannels, id)
}

func ForwardSinksKey(id string) string {
	return fmt.Sprintf(RedisKeyForwardSinks, id)
}

// GenerateChannelKey 使用设备号+通道号作为通道的主键，兼容通道号可能重复的情况
func GenerateChannelKey(device, channel string) ChannelKey {
	return ChannelKey(fmt.Sprintf(RedisUniqueChannelID, device, channel))
}

func (r *RedisDB) LoadOnlineDevices() (map[string]*Device, error) {
	executor, err := r.utils.CreateExecutor()
	if err != nil {
		return nil, err
	}

	keys, err := executor.Keys()
	if err != nil {
		return nil, err
	}

	devices := make(map[string]*Device, len(keys))
	for _, key := range keys {
		device, err := r.findDevice(key, executor)
		if err != nil || device == nil {
			continue
		}

		devices[key] = device
	}

	return devices, nil
}

func (r *RedisDB) findDevice(id string, executor Executor) (*Device, error) {
	value, err := executor.Key(RedisKeyDevices).HGet(id)
	if err != nil {
		return nil, err
	} else if value == nil {
		return nil, nil
	}

	device := &Device{}
	err = json.Unmarshal(value, device)
	if err != nil {
		return nil, err
	}

	return device, nil
}

func (r *RedisDB) findChannel(id ChannelKey, executor Executor) (*Channel, error) {
	value, err := executor.HGet(id.String())
	if err != nil {
		return nil, err
	} else if value == nil {
		return nil, nil
	}

	channel := &Channel{}
	err = json.Unmarshal(value, channel)
	if err != nil {
		return nil, err
	}

	return channel, nil
}

func (r *RedisDB) LoadDevices() (map[string]*Device, error) {
	executor, err := r.utils.CreateExecutor()
	if err != nil {
		return nil, err
	}

	entries, err := executor.Key(RedisKeyDevices).HGetAll()

	devices := make(map[string]*Device, len(entries))
	for k, v := range entries {
		device := &Device{}
		if err = json.Unmarshal(v, device); err != nil {
			continue
		}

		devices[k] = device
	}

	return devices, err
}

func (r *RedisDB) SaveDevice(device *Device) error {
	data, err := json.Marshal(device)
	if err != nil {
		return err
	}

	executor, err := r.utils.CreateExecutor()
	if err != nil {
		return err
		// 保存设备信息
	} else if err = executor.Key(RedisKeyDevices).HSet(device.ID, string(data)); err != nil {
		return err
	}

	return r.UpdateDeviceStatus(device.ID, device.Status)
}

func (r *RedisDB) SaveChannel(deviceId string, channel *Channel) error {
	data, err := json.Marshal(channel)
	if err != nil {
		return err
	}

	channelKey := GenerateChannelKey(deviceId, channel.DeviceID)
	executor, err := r.utils.CreateExecutor()
	if err != nil {
		return err
		// 保存通道信息
	} else if err = executor.Key(RedisKeyChannels).HSet(channelKey.String(), string(data)); err != nil {
		return err
		// 通道关联到Device
	} else if err = executor.Key(fmt.Sprintf(RedisKeyDeviceChannels, deviceId)).ZAddWithNotExists(float64(time.Now().UnixMilli()), channelKey); err != nil {
		return err
	}

	return nil
}

func (r *RedisDB) UpdateDeviceStatus(deviceId string, status OnlineStatus) error {
	executor, err := r.utils.CreateExecutor()
	if err != nil {
		return err
	}

	// 如果在线, 设置有效期key, 添加到设备排序表
	if ON == status {
		// 设置有效期key
		if err = executor.Key(deviceId).Set(nil); err != nil {
			return err
		} else if err = executor.SetExpires(Config.AliveExpires); err != nil {
			return err
			// 排序Device，根据入库时间
		} else if err = executor.Key(RedisKeyDevicesSort).ZAddWithNotExists(float64(time.Now().UnixMilli()), deviceId); err != nil {
			return err
		}
	} else {
		// 删除有效key
		return executor.Key(deviceId).Del()
	}

	return nil
}

func (r *RedisDB) UpdateChannelStatus(channelId, status string) error {
	//TODO implement me
	panic("implement me")
}

func (r *RedisDB) RefreshHeartbeat(deviceId string) error {
	executor, err := r.utils.CreateExecutor()
	if err != nil {
		return err
	} else if err = executor.Key(deviceId).Set(strconv.FormatInt(time.Now().UnixMilli(), 10)); err != nil {
		return err
	}

	return executor.SetExpires(Config.AliveExpires)
}

func (r *RedisDB) QueryDevice(id string) (*Device, error) {
	executor, err := r.utils.CreateExecutor()
	if err != nil {
		return nil, err
	}

	return r.findDevice(id, executor)
}

func (r *RedisDB) QueryDevices(page int, size int) ([]*Device, int, error) {
	executor, err := r.utils.CreateExecutor()
	if err != nil {
		return nil, 0, err
	}

	keys, err := executor.Key(RedisKeyDevicesSort).ZRangeWithAsc(page, size)
	if err != nil {
		return nil, 0, err
	}

	var devices []*Device
	for _, key := range keys {
		device, err := r.findDevice(key, executor)
		if err != nil {
			continue
		}

		devices = append(devices, device)
	}

	// 查询总记录数
	total, err := executor.Key(RedisKeyDevicesSort).CountZSet()
	if err != nil {
		return nil, 0, err
	}

	return devices, total, nil
}

func (r *RedisDB) QueryChannel(deviceId string, channelId string) (*Channel, error) {
	executor, err := r.utils.CreateExecutor()
	if err != nil {
		return nil, err
	}

	executor.Key(RedisKeyChannels)
	return r.findChannel(GenerateChannelKey(deviceId, channelId), executor)
}

func (r *RedisDB) QueryChannels(deviceId string, page, size int) ([]*Channel, int, error) {
	executor, err := r.utils.CreateExecutor()
	if err != nil {
		return nil, 0, err
	}

	id := fmt.Sprintf(RedisKeyDeviceChannels, deviceId)
	keys, err := executor.Key(id).ZRangeWithAsc(page, size)
	if err != nil {
		return nil, 0, err
	}

	executor.Key(RedisKeyChannels)
	var channels []*Channel
	for _, key := range keys {
		channel, err := r.findChannel(ChannelKey(key), executor)
		if err != nil {
			continue
		} else if channel == nil {
			continue
		}

		channels = append(channels, channel)
	}

	// 查询总记录数
	total, err := executor.Key(id).CountZSet()
	if err != nil {
		return nil, 0, err
	}

	return channels, total, nil
}

func (r *RedisDB) LoadPlatforms() ([]*GBPlatformRecord, error) {
	executor, err := r.utils.CreateExecutor()
	if err != nil {
		return nil, err
	}

	var platforms []*GBPlatformRecord
	pairs, err := executor.Key(RedisKeyPlatforms).ZRange()
	if err == nil {
		for _, pair := range pairs {
			platform := &GBPlatformRecord{}
			if err := json.Unmarshal([]byte(pair[0]), platform); err != nil {
				continue
			}

			platform.CreateTime = pair[1]
			platforms = append(platforms, platform)
		}
	}

	return platforms, err
}

func (r *RedisDB) findPlatformWithServerID(id string) (*GBPlatformRecord, error) {
	platforms, err := r.LoadPlatforms()
	if err != nil {
		return nil, err
	}

	for _, platform := range platforms {
		if platform.SeverID == id {
			return platform, nil
		}
	}

	return nil, err
}

func (r *RedisDB) QueryPlatform(id string) (*GBPlatformRecord, error) {
	return r.findPlatformWithServerID(id)
}

func (r *RedisDB) SavePlatform(platform *GBPlatformRecord) error {
	r.platformsLock.Lock()
	defer r.platformsLock.Unlock()

	executor, err := r.utils.CreateExecutor()
	if err != nil {
		return err
	}

	platforms, _ := r.LoadPlatforms()
	for _, old := range platforms {
		if old.SeverID == platform.SeverID {
			return fmt.Errorf("id冲突")
		} else if old.ServerAddr == platform.ServerAddr {
			return fmt.Errorf("地址冲突")
		}
	}

	data, err := json.Marshal(platform)
	if err != nil {
		return err
	}

	return executor.Key(RedisKeyPlatforms).ZAddWithNotExists(platform.CreateTime, data)
}

func (r *RedisDB) DeletePlatform(v *GBPlatformRecord) error {
	r.platformsLock.Lock()
	defer r.platformsLock.Unlock()

	executor, err := r.utils.CreateExecutor()
	if err != nil {
		return err
	}

	platform, _ := r.findPlatformWithServerID(v.SeverID)
	if platform == nil {
		return fmt.Errorf("platform with ID %s not find", v.SeverID)
	}

	// 删除所有通道, 没有事务
	if err = executor.Key(fmt.Sprintf(RedisKeyDeviceChannels, platform.SeverID)).Del(); err != nil {
		return err
	}

	return executor.Key(RedisKeyPlatforms).ZDelWithScore(platform.CreateTime)
}

func (r *RedisDB) UpdatePlatform(platform *GBPlatformRecord) error {
	r.platformsLock.Lock()
	defer r.platformsLock.Unlock()

	executor, err := r.utils.CreateExecutor()
	if err != nil {
		return err
	}

	oldPlatform, _ := r.findPlatformWithServerID(platform.SeverID)
	if oldPlatform == nil {
		return fmt.Errorf("platform with ID %s not find", platform.SeverID)
	}

	data, err := json.Marshal(platform)
	if err != nil {
		return err
	}

	return executor.ZAdd(oldPlatform.CreateTime, data)
}

func (r *RedisDB) UpdatePlatformStatus(serverId string, status OnlineStatus) error {
	r.platformsLock.Lock()
	defer r.platformsLock.Unlock()

	executor, err := r.utils.CreateExecutor()
	if err != nil {
		return err
	}

	oldPlatform, _ := r.findPlatformWithServerID(serverId)
	if oldPlatform == nil {
		return fmt.Errorf("platform with ID %s not find", serverId)
	}

	oldPlatform.Status = status
	data, err := json.Marshal(oldPlatform)
	if err != nil {
		return err
	}

	return executor.ZAdd(oldPlatform.CreateTime, data)
}

func (r *RedisDB) BindChannels(id string, channels [][2]string) ([][2]string, error) {
	r.platformsLock.Lock()
	defer r.platformsLock.Unlock()

	platform, err := r.QueryPlatform(id)
	if err != nil {
		return nil, err
	} else if platform == nil {
		return nil, fmt.Errorf("platform with ID %s not find", platform.SeverID)
	}

	executor, err := r.utils.CreateExecutor()
	if err != nil {
		return nil, err
	}

	// 返回成功的设备通道号
	var result [][2]string
	for _, v := range channels {
		deviceId := v[0]
		channelId := v[1]

		channelKey := GenerateChannelKey(deviceId, channelId)
		// 检查通道是否存在, 以及通道是否冲突
		channel, err := r.findChannel(channelKey, executor.Key(RedisKeyChannels))
		if err != nil {
			Sugar.Errorf("添加通道失败, err: %s device: %s channel: %s", err.Error(), deviceId, channelId)
		} else if channel == nil {
			Sugar.Errorf("添加通道失败, 通道不存在. device: %s channel: %s", deviceId, channelId)
		} else if score, _ := executor.Key(DeviceChannelsKey(id)).ZGetScore(channelKey); score != nil {
			Sugar.Errorf("添加通道失败, 通道冲突. device: %s channel: %s", deviceId, channelId)
		} else if err = executor.Key(DeviceChannelsKey(id)).ZAddWithNotExists(time.Now().UnixMilli(), channelKey); err != nil {
			Sugar.Errorf("添加通道失败, err: %s device: %s channel: %s", err.Error(), deviceId, channelId)
		} else {
			result = append(result, v)
		}
	}

	return result, nil
}

func (r *RedisDB) UnbindChannels(id string, channels [][2]string) ([][2]string, error) {
	r.platformsLock.Lock()
	defer r.platformsLock.Unlock()

	platform, err := r.QueryPlatform(id)
	if err != nil {
		return nil, err
	} else if platform == nil {
		return nil, fmt.Errorf("platform with ID %s not find", platform.SeverID)
	}

	executor, err := r.utils.CreateExecutor()
	if err != nil {
		return nil, err
	}

	// 返回成功的设备通道号
	var result [][2]string
	for _, v := range channels {
		if err := executor.Key(DeviceChannelsKey(id)).ZDel(GenerateChannelKey(v[0], v[1])); err != nil {
			continue
		}

		result = append(result, v)
	}

	return result, nil
}

func (r *RedisDB) QueryPlatformChannel(platformId string, channelId string) (string, *Channel, error) {
	executor, err := r.utils.CreateExecutor()
	if err != nil {
		return "", nil, err
	}

	score, err := executor.Key(DeviceChannelsKey(platformId)).ZGetScore(channelId)
	if err != nil {
		return "", nil, err
	}

	deviceId := score.(string)
	channel, err := r.findChannel(GenerateChannelKey(deviceId, channelId), executor.Key(RedisKeyChannels))
	if err != nil {
		return "", nil, err
	}

	return deviceId, channel, nil
}

func (r *RedisDB) LoadStreams() (map[string]*Stream, error) {
	executor, err := r.utils.CreateExecutor()
	if err != nil {
		return nil, err
	}

	all, err := executor.Key(RedisKeyStreams).ZRange()
	if err != nil {
		return nil, err
	}

	streams := make(map[string]*Stream, len(all))
	for _, v := range all {
		stream := &Stream{}
		if err := json.Unmarshal([]byte(v[0]), stream); err != nil {
			Sugar.Errorf("解析stream失败, err: %s value: %s", err.Error(), hex.EncodeToString([]byte(v[0])))
			continue
		}

		streams[string(stream.ID)] = stream
	}

	return streams, nil
}

func (r *RedisDB) SaveStream(stream *Stream) error {
	data, err := json.Marshal(stream)
	if err != nil {
		return err
	}

	executor, err := r.utils.CreateExecutor()
	if err != nil {
		return err
	}

	//	return executor.Key(RedisKeyStreams).ZAddWithNotExists(stream.CreateTime, data)
	return executor.Key(RedisKeyStreams).ZAdd(stream.CreateTime, data)
}

func (r *RedisDB) DeleteStream(time int64) error {
	executor, err := r.utils.CreateExecutor()
	if err != nil {
		return err
	}

	return executor.Key(RedisKeyStreams).ZDelWithScore(time)
}

func (r *RedisDB) QueryForwardSink(stream StreamID, sinkId string) (*Sink, error) {
	executor, err := r.utils.CreateExecutor()
	if err != nil {
		return nil, err
	}

	data, err := executor.Key(ForwardSinksKey(string(stream))).HGet(sinkId)
	if err != nil {
		return nil, err
	}

	sink := &Sink{}
	if err = json.Unmarshal(data, sink); err != nil {
		return nil, err
	}

	return sink, nil
}

func (r *RedisDB) QueryForwardSinks(stream StreamID) (map[string]*Sink, error) {
	executor, err := r.utils.CreateExecutor()
	if err != nil {
		return nil, err
	}

	entries, err := executor.Key(ForwardSinksKey(string(stream))).HGetAll()
	if err != nil {
		return nil, err
	}

	var sinks map[string]*Sink
	if len(entries) > 0 {
		sinks = make(map[string]*Sink, len(entries))
	}

	for _, entry := range entries {
		sink := &Sink{}
		if err = json.Unmarshal(entry, sink); err != nil {
			return nil, err
		}

		sinks[sink.ID] = sink
	}

	return sinks, nil
}

func (r *RedisDB) SaveForwardSink(stream StreamID, sink *Sink) error {
	executor, err := r.utils.CreateExecutor()
	if err != nil {
		return err
	}

	data, err := json.Marshal(sink)
	if err != nil {
		return err
	}

	return executor.Key(ForwardSinksKey(string(stream))).HSet(sink.ID, data)
}

func (r *RedisDB) DeleteForwardSink(stream StreamID, sinkId string) error {
	executor, err := r.utils.CreateExecutor()
	if err != nil {
		return err
	}

	return executor.Key(ForwardSinksKey(string(stream))).HDel(sinkId)
}

func (r *RedisDB) Del(key string) error {
	executor, err := r.utils.CreateExecutor()
	if err != nil {
		return err
	}

	return executor.Key(key).Del()
}

func (r *RedisDB) DeleteForwardSinks(stream StreamID) error {
	return r.Del(ForwardSinksKey(string(stream)))
}

// OnExpires Redis设备ID到期回调
func (r *RedisDB) OnExpires(db int, id string) {
	Sugar.Infof("设备心跳过期 device: %s", id)

	device := DeviceManager.Find(id)
	if device == nil {
		Sugar.Errorf("设备不存在 device: %s", id)
		return
	}

	device.Close()
}

func NewRedisDB(addr, password string) *RedisDB {
	db := &RedisDB{
		utils: NewRedisUtils(addr, password),
	}

	for {
		err := StartExpiredKeysSubscription(db.utils, 0, db.OnExpires)
		if err == nil {
			break
		}

		Sugar.Errorf("监听redis过期key失败, err: %s", err.Error())
		time.Sleep(3 * time.Second)
	}

	return db
}
