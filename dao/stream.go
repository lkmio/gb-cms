package dao

import (
	"gb-cms/common"
	"github.com/ghettovoice/gosip/sip"
	"github.com/lkmio/avformat/utils"
	"gorm.io/gorm"
)

type StreamModel struct {
	GBModel
	DeviceID  string                 `gorm:"index"`                  // 下级设备ID, 统计某个设备的所有流/1078设备为sim number
	ChannelID string                 `gorm:"index"`                  // 下级通道ID, 统计某个设备下的某个通道的所有流/1078设备为 channel number
	StreamID  common.StreamID        `json:"stream_id" gorm:"index"` // 流ID
	Protocol  int                    `json:"protocol,omitempty"`     // 推流协议, rtmp/28181/1078/gb_talk
	Dialog    *common.RequestWrapper `json:"dialog,omitempty"`       // 国标流的SipCall会话
	SinkCount int32                  `json:"sink_count"`             // 拉流端计数(包含级联转发)
	SetupType common.SetupType
	CallID    string   `json:"call_id" gorm:"index"`
	Urls      []string `gorm:"serializer:json"` // 从流媒体服务器返回的拉流地址
	Name      string   `gorm:"index"`           // 视频通道名
}

func (s *StreamModel) TableName() string {
	return "lkm_stream"
}

func (s *StreamModel) SetDialog(dialog sip.Request) {
	s.Dialog = &common.RequestWrapper{dialog}
	id, _ := dialog.CallID()
	s.CallID = id.Value()
}

type DaoStream interface {
	LoadStreams() (map[string]*StreamModel, error)

	SaveStream(stream *StreamModel) (*StreamModel, bool)

	UpdateStream(stream *StreamModel) error

	DeleteStream(streamId common.StreamID) (*StreamModel, error)

	DeleteStreams() ([]*StreamModel, error)

	DeleteStreamsByIds(ids []uint) error

	QueryStream(streamId common.StreamID) (*StreamModel, error)

	QueryStreams(keyword string, page, size int) ([]*StreamModel, int, error)

	QueryStreamByCallID(callID string) (*StreamModel, error)

	DeleteStreamByCallID(callID string) (*StreamModel, error)

	DeleteStreamByDeviceID(deviceID string) ([]*StreamModel, error)
}

type daoStream struct {
}

func (d *daoStream) LoadStreams() (map[string]*StreamModel, error) {
	var streams []*StreamModel
	tx := db.Find(&streams)
	if tx.Error != nil {
		return nil, tx.Error
	}

	streamMap := make(map[string]*StreamModel)
	for _, stream := range streams {
		streamMap[string(stream.StreamID)] = stream
	}

	return streamMap, nil
}

func (d *daoStream) SaveStream(stream *StreamModel) (*StreamModel, bool) {
	var old StreamModel
	tx := db.Where("stream_id =?", stream.StreamID).Take(&old)
	if old.ID != 0 {
		return &old, false
	}
	// stream唯一必须不存在
	utils.Assert(tx.Error != nil)
	return nil, DBTransaction(func(tx *gorm.DB) error {
		return tx.Save(stream).Error
	}) == nil
}

func (d *daoStream) UpdateStream(stream *StreamModel) error {
	var old StreamModel
	tx := db.Where("stream_id =?", stream.StreamID).Take(&old)
	if tx.Error != nil {
		return tx.Error
	}

	stream.ID = old.ID
	return DBTransaction(func(tx *gorm.DB) error {
		return tx.Save(stream).Error
	})
}

func (d *daoStream) DeleteStream(streamId common.StreamID) (*StreamModel, error) {
	var stream StreamModel
	tx := db.Where("stream_id =?", streamId).Take(&stream)
	if tx.Error != nil {
		return nil, tx.Error
	}

	return &stream, DBTransaction(func(tx *gorm.DB) error {
		return tx.Where("stream_id =?", streamId).Unscoped().Delete(&StreamModel{}).Error
	})
}

func (d *daoStream) DeleteStreamsByIds(ids []uint) error {
	return DBTransaction(func(tx *gorm.DB) error {
		return tx.Where("id in ?", ids).Unscoped().Delete(&StreamModel{}).Error
	})
}

func (d *daoStream) DeleteStreams() ([]*StreamModel, error) {
	var streams []*StreamModel
	tx := db.Find(&streams)
	if tx.Error != nil {
		return nil, tx.Error
	}

	DBTransaction(func(tx *gorm.DB) error {
		for _, stream := range streams {
			_ = tx.Where("stream_id =?", stream.StreamID).Unscoped().Delete(&StreamModel{})
		}
		return nil
	})

	return streams, nil
}

func (d *daoStream) QueryStream(streamId common.StreamID) (*StreamModel, error) {
	var stream StreamModel
	tx := db.Where("stream_id =?", streamId).Take(&stream)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return &stream, nil
}

func (d *daoStream) QueryStreamByCallID(callID string) (*StreamModel, error) {
	var stream StreamModel
	tx := db.Where("call_id =?", callID).Take(&stream)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return &stream, nil
}

func (d *daoStream) DeleteStreamByCallID(callID string) (*StreamModel, error) {
	var stream StreamModel
	tx := db.Where("call_id =?", callID).Take(&stream)
	if tx.Error != nil {
		return nil, tx.Error
	}

	return &stream, DBTransaction(func(tx *gorm.DB) error {
		return tx.Where("call_id =?", callID).Unscoped().Delete(&StreamModel{}).Error
	})
}

func (d *daoStream) DeleteStreamByDeviceID(deviceID string) ([]*StreamModel, error) {
	var streams []*StreamModel
	tx := db.Where("device_id =?", deviceID).Find(&streams)
	if tx.Error != nil {
		return nil, tx.Error
	}
	_ = DBTransaction(func(tx *gorm.DB) error {
		for _, stream := range streams {
			_ = tx.Where("stream_id =?", stream.StreamID).Unscoped().Delete(&StreamModel{})
		}
		return nil
	})

	return streams, nil
}

func (d *daoStream) QueryStreams(keyword string, page, size int) ([]*StreamModel, int, error) {
	var streams []*StreamModel
	var total int64

	tx := db.Model(&StreamModel{}).Offset((page - 1) * size).Limit(size)
	if keyword != "" {
		tx.Where("name like ? or device_id like ? or channel_id like ?", "%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")
	}

	if tx = tx.Find(&streams); tx.Error != nil {
		return nil, 0, tx.Error
	}

	tx = db.Model(&StreamModel{})
	if keyword != "" {
		tx.Where("name like ? or device_id like ? or channel_id like ?", "%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")
	}

	if tx = tx.Count(&total); tx.Error != nil {
		return nil, 0, tx.Error
	}

	return streams, int(total), nil
}
