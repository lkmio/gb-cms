package dao

import (
	"fmt"
	"gb-cms/common"
	"gorm.io/gorm"
)

// SinkModel 级联/对讲/网关转发流Sink
type SinkModel struct {
	GBModel
	SinkID       string                 `json:"sink_id"`            // 流媒体服务器中的sink id
	StreamID     common.StreamID        `json:"stream_id"`          // 推流ID
	SinkStreamID common.StreamID        `json:"sink_stream_id"`     // 广播使用, 每个广播设备的唯一ID
	Protocol     string                 `json:"protocol,omitempty"` // 转发流协议, gb_cascaded/gb_talk/gb_gateway
	Dialog       *common.RequestWrapper `json:"dialog,omitempty"`
	CallID       string                 `json:"call_id,omitempty"`
	ServerAddr   string                 `json:"server_addr,omitempty"` // 级联上级地址
	CreateTime   int64                  `json:"create_time"`
	SetupType    common.SetupType       // 流转发类型
}

func (d *SinkModel) TableName() string {
	return "lkm_sink"
}

type daoSink struct {
}

func (d *daoSink) LoadForwardSinks() (map[string]*SinkModel, error) {
	var sinks []*SinkModel
	tx := db.Find(&sinks)
	if tx.Error != nil {
		return nil, tx.Error
	}

	sinkMap := make(map[string]*SinkModel)
	for _, sink := range sinks {
		sinkMap[sink.SinkID] = sink
	}
	return sinkMap, nil
}

func (d *daoSink) QueryForwardSink(stream common.StreamID, sinkId string) (*SinkModel, error) {
	var sink SinkModel
	db.Where("stream_id =? and sink_id =?", stream, sinkId).Take(&sink)
	return &sink, db.Error
}

func (d *daoSink) QueryForwardSinks(stream common.StreamID) (map[string]*SinkModel, error) {
	var sinks []*SinkModel
	tx := db.Where("stream_id =?", stream).Find(&sinks)
	if tx.Error != nil {
		return nil, tx.Error
	}

	sinkMap := make(map[string]*SinkModel)
	for _, sink := range sinks {
		sinkMap[sink.SinkID] = sink
	}
	return sinkMap, nil
}

func (d *daoSink) SaveForwardSink(stream common.StreamID, sink *SinkModel) error {
	var old SinkModel
	tx := db.Select("id").Where("sink_id =?", sink.SinkID).Take(&old)
	if tx.Error == nil {
		return fmt.Errorf("sink already exists")
	}

	return DBTransaction(func(tx *gorm.DB) error {
		return tx.Save(sink).Error
	})
}

func (d *daoSink) DeleteForwardSink(stream common.StreamID, sinkId string) (*SinkModel, error) {
	var sink SinkModel
	tx := db.Where("sink_id =?", sinkId).Take(&sink)
	if tx.Error != nil {
		return nil, tx.Error
	}

	return &sink, DBTransaction(func(tx *gorm.DB) error {
		return tx.Where("sink_id =?", sinkId).Unscoped().Delete(&SinkModel{}).Error
	})
}

func (d *daoSink) DeleteForwardSinksByStreamID(stream common.StreamID) ([]*SinkModel, error) {
	var sinks []*SinkModel
	tx := db.Where("stream_id =?", stream).Find(&sinks)
	if tx.Error != nil {
		return nil, tx.Error
	}

	return sinks, DBTransaction(func(tx *gorm.DB) error {
		return tx.Where("stream_id =?", stream).Unscoped().Delete(&SinkModel{}).Error
	})
}

func (d *daoSink) QueryForwardSinkByCallID(callID string) (*SinkModel, error) {
	var sinks SinkModel
	tx := db.Where("call_id =?", callID).Find(&sinks)
	if tx.Error != nil {
		return nil, tx.Error
	}

	return &sinks, nil
}

func (d *daoSink) DeleteForwardSinkByCallID(callID string) (*SinkModel, error) {
	var sink SinkModel
	tx := db.Where("call_id =?", callID).First(&sink)
	if tx.Error != nil {
		return nil, tx.Error
	}

	return &sink, DBTransaction(func(tx *gorm.DB) error {
		return tx.Where("call_id =?", callID).Unscoped().Delete(&SinkModel{}).Error
	})
}

func (d *daoSink) DeleteForwardSinkBySinkStreamID(sinkStreamId common.StreamID) (*SinkModel, error) {
	var sink SinkModel
	tx := db.Where("sink_stream_id =?", sinkStreamId).First(&sink)
	if tx.Error != nil {
		return nil, tx.Error
	}

	return &sink, DBTransaction(func(tx *gorm.DB) error {
		return tx.Where("sink_stream_id =?", sinkStreamId).Unscoped().Delete(&SinkModel{}).Error
	})
}

func (d *daoSink) DeleteForwardSinks() ([]*SinkModel, error) {
	var sinks []*SinkModel
	tx := db.Find(&sinks)
	if tx.Error != nil {
		return nil, tx.Error
	}

	return sinks, DBTransaction(func(tx *gorm.DB) error {
		return tx.Unscoped().Delete(&SinkModel{}).Error
	})
}

func (d *daoSink) DeleteForwardSinksByIds(ids []uint) error {
	return DBTransaction(func(tx *gorm.DB) error {
		return tx.Where("id in?", ids).Unscoped().Delete(&SinkModel{}).Error
	})
}

func (d *daoSink) DeleteForwardSinksByServerAddr(addr string) ([]*SinkModel, error) {
	var sinks []*SinkModel
	tx := db.Where("server_addr =?", addr).Find(&sinks)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return sinks, DBTransaction(func(tx *gorm.DB) error {
		return tx.Where("server_addr =?", addr).Unscoped().Delete(&SinkModel{}).Error
	})
}
