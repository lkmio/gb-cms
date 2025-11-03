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
	CreatedAt time.Time `json:"-" xml:"-"`
	UpdatedAt time.Time `json:"-" xml:"-"`
}

type ChannelInfo struct {
	PTZType                  string     `json:"ptz_type" xml:"PTZType,omitempty"`                                    // <!--  摄像机结构类型，标识摄像机类型：1-球机；2-半球；3-固定枪机；4-遥控枪机；5-遥控半球；6-多目设备的全景/拼接通道；7-多目设备的分割通道。当为摄像机时可选。-->
	PhotoelectricImagingType string     `json:"photoelectric_imaging_type" xml:"PhotoelectricImagingType,omitempty"` // <!--  摄像机光电成像类型。1-可见光成像；2-热成像；3-雷达成像；4-X光成像；5-深度光场成像；9-其他。可多值，用英文半角“/”分割。当为摄像机时可选。-->
	CapturePositionType      string     `json:"capture_position_type" xml:"CapturePositionType,omitempty"`           // <!--  摄像机采集部位类型。采用附录O中的规定。当为摄像机时可选。-->
	RoomType                 int        `json:"room_type" xml:"RoomType,omitempty" gorm:"default:1"`                 // <!--  摄像机安装位置室外、室内属性。1-室外、2-室内。当为摄像机时可选，缺省为1。-->
	SupplyLightType          int        `json:"supply_light_type" xml:"SupplyLightType,omitempty" gorm:"default:1"`  // <!--  摄像机补光属性。1-无补光；2-红外补光；3-白光补光；4-激光补光；9-其他。当为摄像机时可选，缺省为1。-->
	DirectionType            *int       `json:"direction_type" xml:"DirectionType,omitempty"`                        // <!--  摄像机监视方位（光轴方向）属性。1-东（西向东）、2-西（东向西）、3-南（北向南）、4-北（南向北）、5-东南（西北到东南）、6-东北（西南到东北）、7-西南（东北到西南）、8-西北（东南到西北）。当为摄像机时且为固定摄像机或设置看守位摄像机时可选。-->
	Resolution               string     `json:"resolution" xml:"Resolution,omitempty"`                               // <!--  摄像机支持的分辨率，可多值，用英文半角“/”。分辨率取值参见附录G中SDP f字段规定。当为摄像机时可选。-->
	StreamNumberList         string     `json:"stream_number_list" xml:"StreamNumberList,omitempty"`                 // <!--  摄像机支持的码流编号列表，用于实时点播时指定码流编号（可选），多个取值间用英文半角“/”分割。如“0/1/2”,表示支持主码流，子码流1，子码流2，以此类推。-->
	DownloadSpeed            string     `json:"download_speed" xml:"DownloadSpeed,omitempty"`                        // <!-- 下载倍速（可选），可多值，用英文半角“/”分割，如设备支持1,2,4倍速下载则应写为“1/2/4”-->
	SVCSpaceSupportMode      *int       `json:"svc_space_support_mode" xml:"SVCSpaceSupportMode,omitempty"`          // <!--  空域编码能力，取值0-不支持；1-1级增强（1个增强层）；2-2级增强（2个增强层）；3-3级增强（3个增强层）（可选）-->
	SVCTimeSupportMode       *int       `json:"svc_time_support_mode" xml:"SVCTimeSupportMode,omitempty"`            // <!--  时域编码能力，取值0-不支持；1-1级增强；2-2级增强；3-3级增强（可选）-->
	SSVCRatioSupportList     string     `json:"ssvc_ratio_support_list" xml:"SSVCRatioSupportList,omitempty"`        // <!--  SSVC增强层与基本层比例能力，多个取值间用英文半角“/”分割。如“4:3/2:1/4:1/6:1/8:1”等具体比例值一种或多种（可选）-->
	MobileDeviceType         *int       `json:"mobile_device_type" xml:"MobileDeviceType,omitempty"`                 // <!--  移动采集设备类型（仅移动采集设备适用，必选）；1-移动机器人载摄像机；2-执法记录仪；3-移动单兵设备；4-车载视频记录设备；5-无人机载摄像机；9-其他 -->
	HorizontalFieldAngle     float64    `json:"horizontal_field_angle" xml:"HorizontalFieldAngle,omitempty"`         // <!--  摄像机水平视场角（可选），取值范围大于0度小于等于360度 -->
	VerticalFieldAngle       float64    `json:"vertical_field_angle" xml:"VerticalFieldAngle,omitempty"`             // <!--  摄像机竖直视场角（可选），取值范围大于0度小于等于360度 -->
	MaxViewDistance          float64    `json:"max_view_distance" xml:"MaxViewDistance,omitempty"`                   // <!--  摄像机可视距离（可选），单位：米 -->
	GrassrootsCode           string     `json:"grassroots_code" xml:"GrassrootsCode,omitempty"`                      // <!--  基层组织编码（必选，非基层建设时为“000000”），编码规则采用附录E.3中规定的格式。 -->
	PointType                *int       `json:"point_type" xml:"PointType,omitempty"`                                // <!--  监控点位类型（当为摄像机时必选），1-一类视频监控点；2-二类视频监控点；3-三类视频监控点；9-其他点位。-->
	PointCommonName          string     `json:"point_common_name" xml:"PointCommonName,omitempty"`                   // <!--  点位俗称（可选），监控点位附近如有标志性建筑、场所或监控点位处于公众约定俗成的地点，可以填写标志性建设名称和地点俗称 -->
	MAC                      string     `json:"mac" xml:"MAC,omitempty"`                                             // <!-- 设备MAC地址（可选），用“XX-XX-XX-XX-XX-XX”格式表达，其中“XX”表示2位十六进制数，用英文半角“-”隔开  -->
	FunctionType             string     `json:"function_type" xml:"FunctionType,omitempty"`                          // <!--  摄像机卡口功能类型，01-人脸卡口；02-人员卡口；03-机动车卡口；04-非机动车卡口；05-物品卡口；99-其他。可多值，用英文半角“/”分割。当为摄像机时可选-->
	EncodeType               string     `json:"encode_type" xml:"EncodeType,omitempty"`                              // <!-- 摄像机视频编码格式（可选），取值参见附录G中SDP f字段规定。-->
	InstallTime              *time.Time `json:"install_time" xml:"InstallTime,omitempty"`                            // <!--  摄像机安装使用时间。一类视频监控点必选；二类、三类可选 -->
	ManagementUnit           string     `json:"management_unit" xml:"ManagementUnit,omitempty"`                      // <!--  摄像机所属管理单位名称（可选）-->
	ContactInfo              string     `json:"contact_info" xml:"ContactInfo,omitempty"`                            // <!--  摄像机所属管理单位联系人的联系方式（电话号码，可多值，用英文半角“/”分割）。一类视频监控点必填；二类、三类选填 -->
	RecordSaveDays           *int       `json:"record_save_days" xml:"RecordSaveDays,omitempty"`                     // <!--  录像保存天数（可选），一类视频监控点必填；二类、三类选填-->
	IndustrialClassification string     `json:"industrial_classification" xml:"IndustrialClassification,omitempty"`  // <!--  国民经济行业分类代码（可选），代码见GB/T 4754第5章 -->
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
	Info            *ChannelInfo        `json:"-" xml:"Info" gorm:"embedded"`
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
