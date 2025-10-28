package dao

import "gorm.io/gorm"

type BlacklistModel struct {
	GBModel
	Rule string
	Key  string `gorm:"uniqueIndex:idx_rule_key"` // 唯一
}

func (d *BlacklistModel) TableName() string {
	return "lkm_blacklist"
}

type daoBlacklist struct {
}

func (d daoBlacklist) SaveIP(ip string) error {
	return DBTransaction(func(tx *gorm.DB) error {
		return d.saveIP(tx, ip)
	})
}

func (d daoBlacklist) saveIP(tx *gorm.DB, ip string) error {
	err := tx.Create(&BlacklistModel{
		Key:  ip,
		Rule: "ip",
	}).Error
	if err == nil {
		_ = BlacklistManager.SaveIP(ip)
	}
	return err
}

func (d daoBlacklist) saveUA(tx *gorm.DB, ua string) error {
	err := tx.Create(&BlacklistModel{
		Key:  ua,
		Rule: "ua",
	}).Error

	if err == nil {
		_ = BlacklistManager.SaveUA(ua)
	}
	return err
}

func (d daoBlacklist) DeleteIP(ip string) error {
	return DBTransaction(func(tx *gorm.DB) error {
		err := tx.Delete(&BlacklistModel{}, "key = ? and rule = ?", ip, "ip").Error
		if err == nil {
			_ = BlacklistManager.DeleteIP(ip)
		}
		return err
	})
}

func (d daoBlacklist) SaveUA(ua string) error {
	return DBTransaction(func(tx *gorm.DB) error {
		return d.saveUA(tx, ua)
	})
}

func (d daoBlacklist) DeleteUA(ua string) error {
	return DBTransaction(func(tx *gorm.DB) error {
		err := tx.Delete(&BlacklistModel{}, "key = ? and rule = ?", ua, "ua").Error
		if err == nil {
			_ = BlacklistManager.DeleteUA(ua)
		}
		return err
	})
}

func (d daoBlacklist) QueryIP(ip string) (*BlacklistModel, error) {
	return BlacklistManager.QueryIP(ip)
}

func (d daoBlacklist) QueryUA(ua string) (*BlacklistModel, error) {
	return BlacklistManager.QueryUA(ua)
}

func (d daoBlacklist) Clear() error {
	return DBTransaction(func(tx *gorm.DB) error {
		return tx.Exec("DELETE FROM lkm_blacklist;").Error
	})
}

func (d daoBlacklist) Load() ([]*BlacklistModel, error) {
	var models []*BlacklistModel
	err := db.Find(&models).Error
	if err != nil {
		return nil, err
	}
	return models, nil
}

func (d daoBlacklist) Replace(iplist []string, ualist []string) error {
	return DBTransaction(func(tx *gorm.DB) error {
		err := tx.Exec("DELETE FROM lkm_blacklist;").Error
		if err != nil {
			return err
		}

		BlacklistManager.Clear()
		for _, ip := range iplist {
			if err = d.saveIP(tx, ip); err != nil {
				return err
			}
		}

		for _, ua := range ualist {
			if err = d.saveUA(tx, ua); err != nil {
				return err
			}
		}
		return nil
	})
}
