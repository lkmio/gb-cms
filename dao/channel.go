package dao

import (
	"fmt"
	"gb-cms/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

// GBModel 解决`Model`变量名与gorm.Model冲突
type GBModel struct {
	ID        uint      `gorm:"primarykey" xml:"-"`
	CreatedAt time.Time `json:"created_at" xml:"-"`
	UpdatedAt time.Time `json:"-" xml:"-"`
}

type ChannelModel struct {
	GBModel

	// RootID 是设备的根ID, 用于查询设备的所有通道.
	RootID   string `json:"root_id" xml:"-" gorm:"index"` // 根设备ID
	TypeCode int    `json:"-" xml:"-" gorm:"index"`       // 设备类型编码

	// GroupID 所在组ID. 扩展的数据库字段, 方便查询某个目录下的设备列表.
	// 如果ParentID不为空, ParentID作为组ID, 如果ParentID为空, BusinessGroupID作为组ID.
	GroupID string `json:"-" xml:"-" gorm:"index"`

	DeviceID        string              `json:"device_id" xml:"DeviceID" gorm:"index"`
	Name            string              `json:"name" xml:"Name,omitempty"`
	Manufacturer    string              `json:"manufacturer" xml:"Manufacturer,omitempty"`
	Model           string              `json:"model" xml:"Model,omitempty"`
	Owner           string              `json:"owner" xml:"Owner,omitempty"`
	CivilCode       string              `json:"civil_code" xml:"CivilCode,omitempty"`
	Block           string              `json:"block" xml:"Block,omitempty"`
	Address         string              `json:"address" xml:"Address,omitempty"`
	Parental        string              `json:"parental" xml:"Parental,omitempty"`
	ParentID        string              `json:"parent_id" xml:"ParentID,omitempty" gorm:"index"` // 父设备ID/系统ID/虚拟目录ID
	BusinessGroupID string              `json:"-" xml:"BusinessGroupID,omitempty" gorm:"index"`
	SafetyWay       string              `json:"safety_way" xml:"SafetyWay,omitempty"`
	RegisterWay     string              `json:"register_way" xml:"RegisterWay,omitempty"`
	CertNum         string              `json:"cert_num" xml:"CertNum,omitempty"`
	Certifiable     string              `json:"certifiable" xml:"Certifiable,omitempty"`
	ErrCode         string              `json:"err_code" xml:"ErrCode,omitempty"`
	EndTime         string              `json:"end_time" xml:"EndTime,omitempty"`
	Secrecy         string              `json:"secrecy" xml:"Secrecy,omitempty"`
	IPAddress       string              `json:"ip_address" xml:"IPAddress,omitempty"`
	Port            string              `json:"port" xml:"Port,omitempty"`
	Password        string              `json:"password" xml:"Password,omitempty"`
	Status          common.OnlineStatus `json:"status" xml:"Status,omitempty"`
	Longitude       string              `json:"longitude" xml:"Longitude,omitempty"`
	Latitude        string              `json:"latitude" xml:"Latitude,omitempty"`
	Setup           common.SetupType    `json:"setup,omitempty" xml:"-"`
	ChannelNumber   int                 `json:"channel_number" xml:"-"`           // 对应1078的通道号
	SubCount        int                 `json:"-" xml:"-"`                        // 子节点数量
	IsDir           bool                `json:"-" xml:"-"`                        // 是否是目录
	CustomID        *string             `gorm:"unique" xml:"-"`                   // 自定义通道ID
	Event           string              `json:"-" xml:"Event,omitempty" gorm:"-"` // <!-- 状态改变事件ON:上线,OFF:离线,VLOST:视频丢失,DEFECT:故障,ADD:增加,DEL:删除,UPDATE:更新(必选)-->
	DropMark        int                 `json:"-" xml:"-"`                        // 是否被过滤 0-不被过滤/非0-被过滤
}

func (d *ChannelModel) TableName() string {
	return "lkm_channel"
}

func (d *ChannelModel) Online() bool {
	return d.Status == common.ON
}

type daoChannel struct {
}

func (d *daoChannel) SaveChannel(channel *ChannelModel) error {
	return DBTransaction(func(tx *gorm.DB) error {
		var old ChannelModel
		if db.Select("id").Where("root_id =? and device_id =?", channel.RootID, channel.DeviceID).Take(&old).Error == nil {
			channel.ID = old.ID
		}
		return tx.Save(channel).Error
	})
}

func (d *daoChannel) SaveChannels(channels []*ChannelModel) error {
	return DBTransaction(func(tx *gorm.DB) error {
		return tx.Save(channels).Error
	})
}

func (d *daoChannel) UpdateChannelStatus(deviceId, channelId, status string) error {
	return db.Model(&ChannelModel{}).Where("root_id =? and device_id =?", deviceId, channelId).Update("status", status).Error
}

func (d *daoChannel) QueryChannelByID(id uint) (*ChannelModel, error) {
	var channel ChannelModel
	tx := db.Where("id =?", id).Take(&channel)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return &channel, nil
}

func (d *daoChannel) QueryChannel(deviceId string, channelId string) (*ChannelModel, error) {
	var channel ChannelModel
	tx := db.Where("root_id =? and device_id =?", deviceId, channelId).Take(&channel)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return &channel, nil
}

func (d *daoChannel) QueryChannels(deviceId, groupId string, page, size int, status string, keyword string, order, sort string, isDir bool) ([]*ChannelModel, int, error) {
	conditions := map[string]interface{}{}
	if deviceId != "" {
		conditions["root_id"] = deviceId
	}

	if groupId != "" {
		conditions["group_id"] = groupId
	}
	if status != "" {
		conditions["status"] = status
	}
	if isDir {
		conditions["is_dir"] = 1
	}

	cTx := db.Where(conditions).Where("(drop_mark != 1 OR drop_mark IS NULL)")

	if page > 0 {
		cTx.Limit(size).Offset((page - 1) * size)
	}
	if keyword != "" {
		cTx.Where("name like ? or device_id like ?", "%"+keyword+"%", "%"+keyword+"%")
	}

	var channels []*ChannelModel
	if sort != "ID" {
		if tx := cTx.Order("device_id " + order).Find(&channels); tx.Error != nil {
			return nil, 0, tx.Error
		}
	} else {
		if tx := cTx.Order("id " + order).Find(&channels); tx.Error != nil {
			return nil, 0, tx.Error
		}
	}

	countTx := db.Model(&ChannelModel{}).Select("id").Where(conditions).Where("(drop_mark != 1 OR drop_mark IS NULL)")
	if keyword != "" {
		countTx.Where("name like ? or device_id like ?", "%"+keyword+"%", "%"+keyword+"%")
	}

	// 重新统计子节点数量
	for _, channel := range channels {
		if !channel.IsDir {
			continue
		}

		var total int64
		// 统计子节点数量
		if tx := db.Model(&ChannelModel{}).Where("root_id =? and group_id =? and (drop_mark != 1 OR drop_mark IS NULL)", channel.RootID, channel.DeviceID).Select("id").Count(&total); tx.Error != nil {
			return nil, 0, tx.Error
		}

		channel.SubCount = int(total)
	}

	var total int64
	if tx := countTx.Count(&total); tx.Error != nil {
		return nil, 0, tx.Error
	}

	return channels, int(total), nil
}

func (d *daoChannel) QueryChannelsByRootID(rootId string) ([]*ChannelModel, error) {
	var channels []*ChannelModel
	tx := db.Where("root_id =?", rootId).Find(&channels)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return channels, nil
}

func (d *daoChannel) QueryChanelCount(deviceId string, hasDir bool) (int, error) {
	var total int64
	tx := db.Model(&ChannelModel{}).Where("root_id =? and (drop_mark != 1 OR drop_mark IS NULL)", deviceId)
	if !hasDir {
		tx.Where("is_dir =?", 0)
	}

	if tx = tx.Select("id").Count(&total); tx.Error != nil {
		return 0, tx.Error
	}
	return int(total), nil
}

func (d *daoChannel) QueryOnlineChanelCount(deviceId string, hasDir bool) (int, error) {
	var total int64
	tx := db.Model(&ChannelModel{}).Where("root_id =? and status =? and (drop_mark != 1 OR drop_mark IS NULL)", deviceId, "ON")
	if !hasDir {
		tx.Where("is_dir =?", 0)
	}

	if tx = tx.Select("id").Count(&total); tx.Error != nil {
		return 0, tx.Error
	}

	return int(total), nil
}

func (d *daoChannel) QueryChannelByTypeCode(codecs ...int) ([]*ChannelModel, error) {
	var channels []*ChannelModel
	tx := db.Where("type_code in ? and (drop_mark != 1 OR drop_mark IS NULL)", codecs).Find(&channels)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return channels, nil
}

func (d *daoChannel) ExistChannel(channelId string) bool {
	var channel ChannelModel
	if db.Select("id").Where("device_id =?", channelId).Take(&channel).Error == nil {
		return true
	}

	return false
}

func (d *daoChannel) SaveJTChannel(channel *ChannelModel) error {
	return DBTransaction(func(tx *gorm.DB) error {
		var old ChannelModel
		if tx.Select("id").Where("root_id =? and channel_number =?", channel.RootID, channel.ChannelNumber).Take(&old).Error == nil {
			return fmt.Errorf("channel number %d already exist", channel.ChannelNumber)
		} else if tx.Select("id").Where("device_id =?", channel.DeviceID).Take(&old).Error == nil {
			return fmt.Errorf("channel id %s already exist", channel.DeviceID)
		}
		return tx.Save(channel).Error
	})
}

func (d *daoChannel) DeleteChannels(deviceId string) error {
	return db.Where("root_id =?", deviceId).Unscoped().Delete(&ChannelModel{}).Error
}

func (d *daoChannel) DeleteChannel(deviceId string, channelId string) error {
	return db.Where("root_id =? and device_id =?", deviceId, channelId).Unscoped().Delete(&ChannelModel{}).Error
}

func (d *daoChannel) QueryChannelsByChannelID(channelId string) ([]*ChannelModel, error) {
	var channels []*ChannelModel
	tx := db.Where("device_id =?", channelId).Find(&channels)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return channels, nil
}

// QueryChannelByCustomID 根据自定义通道ID查询通道
func (d *daoChannel) QueryChannelByCustomID(customID string) (*ChannelModel, error) {
	var channel ChannelModel
	tx := db.Where("custom_id =?", customID).Take(&channel)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return &channel, nil
}

func (d *daoChannel) UpdateRootID(rootId, newRootId string) error {
	channel := &ChannelModel{
		RootID:   newRootId,
		GroupID:  newRootId,
		ParentID: newRootId,
	}
	return db.Model(channel).Where("root_id =?", rootId).Select("root_id", "group_id", "parent_id").Updates(channel).Error
}

func (d *daoChannel) UpdateChannel(channel *ChannelModel) error {
	return DBTransaction(func(tx *gorm.DB) error {
		return tx.Model(channel).Where("id =?", channel.ID).Updates(channel).Error
	})
}

func (d *daoChannel) TotalCount() (int, error) {
	var total int64
	tx := db.Model(&ChannelModel{}).Where("(drop_mark != 1 OR drop_mark IS NULL)").Select("id").Count(&total)
	if tx.Error != nil {
		return 0, tx.Error
	}
	return int(total), nil
}

func (d *daoChannel) OnlineCount(ids []string) (int, error) {
	var total int64
	tx := db.Model(&ChannelModel{}).Where("status =? and root_id in ? and (drop_mark != 1 OR drop_mark IS NULL)", "ON", ids).Select("id").Count(&total)
	if tx.Error != nil {
		return 0, tx.Error
	}
	return int(total), nil
}

func (d *daoChannel) QuerySubChannelCount(rootId string, groupId string, hasDir bool) (int, error) {
	var total int64
	tx := db.Model(&ChannelModel{}).Where("root_id =? and group_id =? and (drop_mark != 1 OR drop_mark IS NULL)", rootId, groupId)
	if !hasDir {
		tx.Where("is_dir =?", 0)
	}

	if tx = tx.Select("id").Count(&total); tx.Error != nil {
		return 0, tx.Error
	}
	return int(total), nil
}

func (d *daoChannel) QueryOnlineSubChannelCount(rootId string, groupId string, hasDir bool) (int, error) {
	var total int64
	tx := db.Model(&ChannelModel{}).Where("root_id =? and group_id =? and status =? and (drop_mark != 1 OR drop_mark IS NULL)", rootId, groupId, "ON")
	if !hasDir {
		tx.Where("is_dir =?", 0)
	}
	if tx = tx.Select("id").Count(&total); tx.Error != nil {
		return 0, tx.Error
	}
	return int(total), nil
}

// UpdateCustomID 更新自定义通道ID
func (d *daoChannel) UpdateCustomID(rootId, channelId string, customID string) error {
	return db.Model(&ChannelModel{}).Where("root_id =? and device_id =?", rootId, channelId).Update("custom_id", customID).Error
}

func (d *daoChannel) QueryChannelsByParentID(rootId string, parentId string) ([]*ChannelModel, error) {
	var channels []*ChannelModel
	tx := db.Where("root_id =? and parent_id =?", rootId, parentId).Find(&channels)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return channels, nil
}

func (d *daoChannel) QueryChannelName(rootId string, channelId string) (string, error) {
	var channel ChannelModel
	tx := db.Select("name").Where("root_id =? and device_id =?", rootId, channelId).Take(&channel)
	if tx.Error != nil {
		return "", tx.Error
	}

	return channel.Name, nil
}

func (d *daoChannel) QueryCustomID(rootId string, channelId string) (string, error) {
	var channel ChannelModel
	tx := db.Select("custom_id").Where("root_id =? and device_id =?", rootId, channelId).Take(&channel)
	if tx.Error != nil || channel.CustomID == nil {
		return "", tx.Error
	}

	return *channel.CustomID, nil
}

// DropChannel 过滤通道
func (d *daoChannel) DropChannel(rootId string, typeCodes []string, tx *gorm.DB) error {
	// 如果rootId为空, 过滤所有typeCode相同的通道
	// 如果typeCodecs为空, 所有通道都不被过滤
	update := func(tx *gorm.DB) error {
		var conditions []clause.Expression
		if rootId != "" {
			conditions = append(conditions, gorm.Expr("root_id = ?", rootId))
		} else {
			// 全局过滤时,跳过单独设置过滤的设备
			var rootIds []string
			tx.Model(DeviceModel{}).Where("drop_channel_type != '' and drop_channel_type is not null").Pluck("device_id", &rootIds)
			if len(rootIds) > 0 {
				conditions = append(conditions, gorm.Expr("root_id NOT IN ?", rootIds))
			}
		}

		// 处理typeCodes条件
		if len(typeCodes) > 0 {
			// 先重置所有符合条件的通道为不过滤
			conditions = append(conditions, gorm.Expr("type_code NOT IN ?", typeCodes))
			if err := tx.Model(&ChannelModel{}).Clauses(conditions...).Update("drop_mark", 0).Error; err != nil {
				return err
			}

			// 设置指定typeCodes的通道为过滤
			conditions[len(conditions)-1] = gorm.Expr("type_code IN ?", typeCodes)
			return tx.Model(&ChannelModel{}).Clauses(conditions...).Update("drop_mark", 1).Error
		}

		// typeCodes为空时，重置所有符合条件的通道为不过滤
		return tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Model(&ChannelModel{}).Clauses(conditions...).Update("drop_mark", 0).Error
	}

	if tx != nil {
		return update(tx)
	}

	return DBTransaction(func(tx *gorm.DB) error {
		return update(tx)
	})
}
