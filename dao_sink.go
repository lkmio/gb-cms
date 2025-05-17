package main

import (
	"fmt"
	"gorm.io/gorm"
)

type DaoSink interface {
	LoadForwardSinks() (map[string]*Sink, error)

	// QueryForwardSink 查询转发流Sink
	QueryForwardSink(stream StreamID, sink string) (*Sink, error)

	QueryForwardSinks(stream StreamID) (map[string]*Sink, error)

	// SaveForwardSink 保存转发流Sink
	SaveForwardSink(stream StreamID, sink *Sink) error

	DeleteForwardSink(stream StreamID, sink string) (*Sink, error)

	DeleteForwardSinksByStreamID(stream StreamID) ([]*Sink, error)

	DeleteForwardSinks() ([]*Sink, error)

	DeleteForwardSinksByIds(ids []uint) error

	QueryForwardSinkByCallID(callID string) (*Sink, error)

	DeleteForwardSinkByCallID(callID string) (*Sink, error)

	DeleteForwardSinkBySinkStreamID(sinkStreamID StreamID) (*Sink, error)
}

type daoSink struct {
}

func (d *daoSink) LoadForwardSinks() (map[string]*Sink, error) {
	var sinks []*Sink
	tx := db.Find(&sinks)
	if tx.Error != nil {
		return nil, tx.Error
	}

	sinkMap := make(map[string]*Sink)
	for _, sink := range sinks {
		sinkMap[sink.SinkID] = sink
	}
	return sinkMap, nil
}

func (d *daoSink) QueryForwardSink(stream StreamID, sinkId string) (*Sink, error) {
	var sink Sink
	db.Where("stream_id =? and sink_id =?", stream, sinkId).Take(&sink)
	return &sink, db.Error
}

func (d *daoSink) QueryForwardSinks(stream StreamID) (map[string]*Sink, error) {
	var sinks []*Sink
	tx := db.Where("stream_id =?", stream).Find(&sinks)
	if tx.Error != nil {
		return nil, tx.Error
	}

	sinkMap := make(map[string]*Sink)
	for _, sink := range sinks {
		sinkMap[sink.SinkID] = sink
	}
	return sinkMap, nil
}

func (d *daoSink) SaveForwardSink(stream StreamID, sink *Sink) error {
	var old Sink
	tx := db.Select("id").Where("sink_id =?", sink.SinkID).Take(&old)
	if tx.Error == nil {
		return fmt.Errorf("sink already exists")
	}

	return DBTransaction(func(tx *gorm.DB) error {
		return tx.Save(sink).Error
	})
}

func (d *daoSink) DeleteForwardSink(stream StreamID, sinkId string) (*Sink, error) {
	var sink Sink
	tx := db.Where("sink_id =?", sinkId).Take(&sink)
	if tx.Error != nil {
		return nil, tx.Error
	}

	return &sink, DBTransaction(func(tx *gorm.DB) error {
		return tx.Where("sink_id =?", sinkId).Unscoped().Delete(&Sink{}).Error
	})
}

func (d *daoSink) DeleteForwardSinksByStreamID(stream StreamID) ([]*Sink, error) {
	var sinks []*Sink
	tx := db.Where("stream_id =?", stream).Find(&sinks)
	if tx.Error != nil {
		return nil, tx.Error
	}

	return sinks, DBTransaction(func(tx *gorm.DB) error {
		return tx.Where("stream_id =?", stream).Unscoped().Delete(&Sink{}).Error
	})
}

func (d *daoSink) QueryForwardSinkByCallID(callID string) (*Sink, error) {
	var sinks Sink
	tx := db.Where("call_id =?", callID).Find(&sinks)
	if tx.Error != nil {
		return nil, tx.Error
	}

	return &sinks, nil
}

func (d *daoSink) DeleteForwardSinkByCallID(callID string) (*Sink, error) {
	var sink Sink
	tx := db.Where("call_id =?", callID).First(&sink)
	if tx.Error != nil {
		return nil, tx.Error
	}

	return &sink, DBTransaction(func(tx *gorm.DB) error {
		return tx.Where("call_id =?", callID).Unscoped().Delete(&Sink{}).Error
	})
}

func (d *daoSink) DeleteForwardSinkBySinkStreamID(sinkStreamId StreamID) (*Sink, error) {
	var sink Sink
	tx := db.Where("sink_stream_id =?", sinkStreamId).First(&sink)
	if tx.Error != nil {
		return nil, tx.Error
	}

	return &sink, DBTransaction(func(tx *gorm.DB) error {
		return tx.Where("sink_stream_id =?", sinkStreamId).Unscoped().Delete(&Sink{}).Error
	})
}

func (d *daoSink) DeleteForwardSinks() ([]*Sink, error) {
	var sinks []*Sink
	tx := db.Find(&sinks)
	if tx.Error != nil {
		return nil, tx.Error
	}

	return sinks, DBTransaction(func(tx *gorm.DB) error {
		return tx.Unscoped().Delete(&Sink{}).Error
	})
}

func (d *daoSink) DeleteForwardSinksByIds(ids []uint) error {
	return DBTransaction(func(tx *gorm.DB) error {
		return tx.Where("id in?", ids).Unscoped().Delete(&Sink{}).Error
	})
}
