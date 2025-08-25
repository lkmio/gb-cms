package main

import (
	"github.com/lkmio/avformat/utils"
	"gorm.io/gorm"
)

type DaoStream interface {
	LoadStreams() (map[string]*Stream, error)

	SaveStream(stream *Stream) (*Stream, bool)

	UpdateStream(stream *Stream) error

	DeleteStream(streamId StreamID) (*Stream, error)

	DeleteStreams() ([]*Stream, error)

	DeleteStreamsByIds(ids []uint) error

	QueryStream(streamId StreamID) (*Stream, error)

	QueryStreamByCallID(callID string) (*Stream, error)

	DeleteStreamByCallID(callID string) (*Stream, error)

	DeleteStreamByDeviceID(deviceID string) ([]*Stream, error)
}

type daoStream struct {
}

func (d *daoStream) LoadStreams() (map[string]*Stream, error) {
	var streams []*Stream
	tx := db.Find(&streams)
	if tx.Error != nil {
		return nil, tx.Error
	}

	streamMap := make(map[string]*Stream)
	for _, stream := range streams {
		streamMap[string(stream.StreamID)] = stream
	}

	return streamMap, nil
}

func (d *daoStream) SaveStream(stream *Stream) (*Stream, bool) {
	var old Stream
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

func (d *daoStream) UpdateStream(stream *Stream) error {
	var old Stream
	tx := db.Where("stream_id =?", stream.StreamID).Take(&old)
	if tx.Error != nil {
		return tx.Error
	}

	stream.ID = old.ID
	return DBTransaction(func(tx *gorm.DB) error {
		return tx.Save(stream).Error
	})
}

func (d *daoStream) DeleteStream(streamId StreamID) (*Stream, error) {
	var stream Stream
	tx := db.Where("stream_id =?", streamId).Take(&stream)
	if tx.Error != nil {
		return nil, tx.Error
	}

	return &stream, DBTransaction(func(tx *gorm.DB) error {
		return tx.Where("stream_id =?", streamId).Unscoped().Delete(&Stream{}).Error
	})
}

func (d *daoStream) DeleteStreamsByIds(ids []uint) error {
	return DBTransaction(func(tx *gorm.DB) error {
		return tx.Where("id in ?", ids).Unscoped().Delete(&Stream{}).Error
	})
}

func (d *daoStream) DeleteStreams() ([]*Stream, error) {
	var streams []*Stream
	tx := db.Find(&streams)
	if tx.Error != nil {
		return nil, tx.Error
	}

	DBTransaction(func(tx *gorm.DB) error {
		for _, stream := range streams {
			_ = tx.Where("stream_id =?", stream.StreamID).Unscoped().Delete(&Stream{})
		}
		return nil
	})

	return streams, nil
}

func (d *daoStream) QueryStream(streamId StreamID) (*Stream, error) {
	var stream Stream
	tx := db.Where("stream_id =?", streamId).Take(&stream)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return &stream, nil
}

func (d *daoStream) QueryStreamByCallID(callID string) (*Stream, error) {
	var stream Stream
	tx := db.Where("call_id =?", callID).Take(&stream)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return &stream, nil
}

func (d *daoStream) DeleteStreamByCallID(callID string) (*Stream, error) {
	var stream Stream
	tx := db.Where("call_id =?", callID).Take(&stream)
	if tx.Error != nil {
		return nil, tx.Error
	}

	return &stream, DBTransaction(func(tx *gorm.DB) error {
		return tx.Where("call_id =?", callID).Unscoped().Delete(&Stream{}).Error
	})
}

func (d *daoStream) DeleteStreamByDeviceID(deviceID string) ([]*Stream, error) {
	var streams []*Stream
	tx := db.Where("device_id =?", deviceID).Find(&streams)
	if tx.Error != nil {
		return nil, tx.Error
	}
	_ = DBTransaction(func(tx *gorm.DB) error {
		for _, stream := range streams {
			_ = tx.Where("stream_id =?", stream.StreamID).Unscoped().Delete(&Stream{})
		}
		return nil
	})

	return streams, nil
}
