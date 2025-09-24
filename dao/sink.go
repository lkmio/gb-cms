package dao

import (
	"gb-cms/common"
	"gorm.io/gorm"
)

// SinkModel 级联/对讲/网关转发流Sink
type SinkModel struct {
	GBModel
	SinkID       string                 `json:"sink_id"`                      // 流媒体服务器中的sink id
	StreamID     common.StreamID        `json:"stream_id"`                    // 拉取流的id, 目前和source id一致
	SinkStreamID common.StreamID        `json:"sink_stream_id" gorm:"unique"` // 广播使用, 每个广播设备的唯一ID
	Protocol     int                    `json:"protocol,omitempty"`           // 拉流协议, @See stack.TransStreamRtmp
	Dialog       *common.RequestWrapper `json:"dialog,omitempty"`
	CallID       string                 `json:"call_id,omitempty"`
	ServerAddr   string                 `json:"server_addr,omitempty"` // 级联上级地址
	CreateTime   int64                  `json:"create_time"`
	SetupType    common.SetupType       // 流转发类型
	RemoteAddr   string
}

func (d *SinkModel) TableName() string {
	return "lkm_sink"
}

type daoSink struct {
}

func (d *daoSink) LoadSinks() (map[string]*SinkModel, error) {
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

func (d *daoSink) QuerySink(stream common.StreamID, sinkId string) (*SinkModel, error) {
	var sink SinkModel
	db.Where("stream_id =? and sink_id =?", stream, sinkId).Take(&sink)
	return &sink, db.Error
}

func (d *daoSink) QuerySinks(stream common.StreamID) (map[string]*SinkModel, error) {
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

func (d *daoSink) CreateSink(sink *SinkModel) error {
	return DBTransaction(func(tx *gorm.DB) error {
		return tx.Create(sink).Error
	})
}

func (d *daoSink) SaveSink(sink *SinkModel) error {
	return DBTransaction(func(tx *gorm.DB) error {
		return tx.Save(sink).Error
	})
}

func (d *daoSink) DeleteSink(sinkId string) (*SinkModel, error) {
	var sink SinkModel
	tx := db.Where("sink_id =?", sinkId).Take(&sink)
	if tx.Error != nil {
		return nil, tx.Error
	}

	return &sink, DBTransaction(func(tx *gorm.DB) error {
		return tx.Where("sink_id =?", sinkId).Unscoped().Delete(&SinkModel{}).Error
	})
}

func (d *daoSink) DeleteSinksByStreamID(stream common.StreamID) ([]*SinkModel, error) {
	var sinks []*SinkModel
	tx := db.Where("stream_id =?", stream).Find(&sinks)
	if tx.Error != nil {
		return nil, tx.Error
	}

	return sinks, DBTransaction(func(tx *gorm.DB) error {
		return tx.Where("stream_id =?", stream).Unscoped().Delete(&SinkModel{}).Error
	})
}

func (d *daoSink) QuerySinkByCallID(callID string) (*SinkModel, error) {
	var sinks SinkModel
	tx := db.Where("call_id =?", callID).Find(&sinks)
	if tx.Error != nil {
		return nil, tx.Error
	}

	return &sinks, nil
}

func (d *daoSink) DeleteSinkByCallID(callID string) (*SinkModel, error) {
	var sink SinkModel
	tx := db.Where("call_id =?", callID).First(&sink)
	if tx.Error != nil {
		return nil, tx.Error
	}

	return &sink, DBTransaction(func(tx *gorm.DB) error {
		return tx.Where("call_id =?", callID).Unscoped().Delete(&SinkModel{}).Error
	})
}

func (d *daoSink) DeleteSinkBySinkStreamID(sinkStreamId common.StreamID) (*SinkModel, error) {
	var sink SinkModel
	tx := db.Where("sink_stream_id =?", sinkStreamId).First(&sink)
	if tx.Error != nil {
		return nil, tx.Error
	}

	return &sink, DBTransaction(func(tx *gorm.DB) error {
		return tx.Where("sink_stream_id =?", sinkStreamId).Unscoped().Delete(&SinkModel{}).Error
	})
}

func (d *daoSink) DeleteSinks() ([]*SinkModel, error) {
	var sinks []*SinkModel
	tx := db.Find(&sinks)
	if tx.Error != nil {
		return nil, tx.Error
	}

	return sinks, DBTransaction(func(tx *gorm.DB) error {
		return tx.Unscoped().Delete(&SinkModel{}).Error
	})
}

func (d *daoSink) DeleteSinksByIds(ids []uint) error {
	return DBTransaction(func(tx *gorm.DB) error {
		return tx.Where("id in?", ids).Unscoped().Delete(&SinkModel{}).Error
	})
}

func (d *daoSink) DeleteSinksByServerAddr(addr string) ([]*SinkModel, error) {
	var sinks []*SinkModel
	tx := db.Where("server_addr =?", addr).Find(&sinks)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return sinks, DBTransaction(func(tx *gorm.DB) error {
		return tx.Where("server_addr =?", addr).Unscoped().Delete(&SinkModel{}).Error
	})
}

func (d *daoSink) QuerySinkCountByProtocol(protocol int) (int, error) {
	var count int64
	tx := db.Model(&SinkModel{}).Where("protocol = ?", protocol).Count(&count)
	if tx.Error != nil {
		return 0, tx.Error
	}
	return int(count), nil
}

func (d *daoSink) Count() (int, error) {
	var count int64
	tx := db.Model(&SinkModel{}).Count(&count)
	if tx.Error != nil {
		return 0, tx.Error
	}
	return int(count), nil
}

// QueryStreamIds 指定多个protocol查询streamIds
func (d *daoSink) QueryStreamIds(protocols []int, page, size int) ([]string, int, error) {
	// 查询总数
	var total int64
	tx := db.Model(&SinkModel{}).Where("protocol in ?", protocols).Group("stream_id").Count(&total)
	if tx.Error != nil {
		return nil, 0, tx.Error
	}

	var streamIds []string
	// 分页查询
	tx = db.Model(&SinkModel{}).Select("stream_id").Where("protocol in ?", protocols).Group("stream_id").Offset((page - 1) * size).Limit(size).Find(&streamIds)
	if tx.Error != nil {
		return nil, 0, tx.Error
	}

	return streamIds, int(total), nil
}

func (d *daoSink) QuerySinkBySinkStreamID(sinkStreamId common.StreamID) (*SinkModel, error) {
	var sink SinkModel
	tx := db.Where("sink_stream_id =?", sinkStreamId).First(&sink)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return &sink, nil
}
