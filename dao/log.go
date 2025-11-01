package dao

import (
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"strings"
	"time"
)

type LogModel struct {
	GBModel
	Name         string `json:"Name"`
	Scheme       string `json:"Scheme"`
	Method       string `json:"Method"`
	RequestURI   string `json:"RequestURI"`
	RemoteAddr   string `json:"RemoteAddr"`
	RemoteRegion string `json:"RemoteRegion"`
	Status       string `json:"Status"`
	StatusCode   int    `json:"StatusCode"`
	StartAt      string `json:"StartAt"`
	Duration     int    `json:"Duration"`
	Username     string `json:"Username"`
	ExtInfo      string `json:"ExtInfo"`
	Description  string `json:"Description"`
}

func (l *LogModel) TableName() string {
	return "lkm_log"
}

type daoLog struct {
}

func (l *daoLog) Save(log *LogModel) error {
	return db.Create(log).Error
}

func (l *daoLog) Query(pageSize, pageNumber int, keywords, sort, order, method, startTime, endTime string) ([]*LogModel, int, error) {
	//Name         string    `json:"Name"`
	//Scheme       string    `json:"Scheme"`
	//Method       string    `json:"Method"`
	//RequestURI   string    `json:"RequestURI"`
	//RemoteAddr   string    `json:"RemoteAddr"`
	//RemoteRegion string    `json:"RemoteRegion"`
	//Status       string    `json:"Status"`
	//StatusCode   int       `json:"StatusCode"`
	//StartAt      time.Time `json:"StartAt"`
	//Duration     int       `json:"Duration"`
	//Username     string    `json:"Username"`
	//ExtInfo      string    `json:"ExtInfo"`
	//Description  string    `json:"Description"`

	var conditions []clause.Expression

	if method != "" {
		conditions = append(conditions, clause.Eq{
			Column: clause.Column{Table: clause.CurrentTable, Name: "method"},
			Value:  method,
		})
	}

	// 时间范围查询
	if startTime != "" && endTime != "" {
		conditions = append(conditions, gorm.Expr("start_at BETWEEN ? AND ?", startTime, endTime))
	}

	// 匹配所有字段
	if keywords != "" {
		orConditions := clause.OrConditions{}
		orConditions.Exprs = append(orConditions.Exprs, clause.Like{
			Column: clause.Column{Table: clause.CurrentTable, Name: "name"},
			Value:  "%" + keywords + "%",
		})
		orConditions.Exprs = append(orConditions.Exprs, clause.Like{
			Column: clause.Column{Table: clause.CurrentTable, Name: "scheme"},
			Value:  "%" + keywords + "%",
		})
		orConditions.Exprs = append(orConditions.Exprs, clause.Like{
			Column: clause.Column{Table: clause.CurrentTable, Name: "method"},
			Value:  "%" + keywords + "%",
		})
		orConditions.Exprs = append(orConditions.Exprs, clause.Like{
			Column: clause.Column{Table: clause.CurrentTable, Name: "request_uri"},
			Value:  "%" + keywords + "%",
		})
		orConditions.Exprs = append(orConditions.Exprs, clause.Like{
			Column: clause.Column{Table: clause.CurrentTable, Name: "remote_addr"},
			Value:  "%" + keywords + "%",
		})
		orConditions.Exprs = append(orConditions.Exprs, clause.Like{
			Column: clause.Column{Table: clause.CurrentTable, Name: "remote_region"},
			Value:  "%" + keywords + "%",
		})
		orConditions.Exprs = append(orConditions.Exprs, clause.Like{
			Column: clause.Column{Table: clause.CurrentTable, Name: "status"},
			Value:  "%" + keywords + "%",
		})
		orConditions.Exprs = append(orConditions.Exprs, clause.Like{
			Column: clause.Column{Table: clause.CurrentTable, Name: "username"},
			Value:  "%" + keywords + "%",
		})
		orConditions.Exprs = append(orConditions.Exprs, clause.Like{
			Column: clause.Column{Table: clause.CurrentTable, Name: "ext_info"},
			Value:  "%" + keywords + "%",
		})
		orConditions.Exprs = append(orConditions.Exprs, clause.Like{
			Column: clause.Column{Table: clause.CurrentTable, Name: "description"},
			Value:  "%" + keywords + "%",
		})

		orConditions.Exprs = append(orConditions.Exprs, clause.Like{
			Column: clause.Column{Table: clause.CurrentTable, Name: "status_code"},
			Value:  "%" + keywords + "%",
		})

		orConditions.Exprs = append(orConditions.Exprs, clause.Like{
			Column: clause.Column{Table: clause.CurrentTable, Name: "start_at"},
			Value:  "%" + keywords + "%",
		})

		orConditions.Exprs = append(orConditions.Exprs, clause.Like{
			Column: clause.Column{Table: clause.CurrentTable, Name: "duration"},
			Value:  "%" + keywords + "%",
		})

		orConditions.Exprs = append(orConditions.Exprs, clause.Like{
			Column: clause.Column{Table: clause.CurrentTable, Name: "created_at"},
			Value:  "%" + keywords + "%",
		})
		conditions = append(conditions, orConditions)
	}

	// 查询总数
	var total int64
	if db.Model(&LogModel{}).Select("id").Clauses(conditions...).Count(&total).Error != nil {
		return nil, 0, fmt.Errorf("查询日志总数失败")
	} else if total < 1 {
		return nil, 0, nil
	}

	// 转小写下划线
	switch strings.ToLower(sort) {
	case "startat":
		sort = "start_at"
	case "duration":
		sort = "duration"
	}

	// 分页查询
	var logs []*LogModel
	if db.Model(&LogModel{}).Clauses(conditions...).Offset((pageNumber-1)*pageSize).Limit(pageSize).Order(fmt.Sprintf("%s %s", sort, order)).Find(&logs).Error != nil {
		return nil, 0, fmt.Errorf("查询日志列表失败")
	}

	return logs, int(total), nil
}

func (l *daoLog) Clear() error {
	return DBTransaction(func(tx *gorm.DB) error {
		return tx.Exec("DELETE FROM lkm_log;").Error
	})
}

func (l *daoLog) DeleteExpired(expireTime time.Time) error {
	return DBTransaction(func(tx *gorm.DB) error {
		tx.Delete(&LogModel{}, "created_at < ?", expireTime.Format("2006-01-02 15:04:05"))
		return nil
	})
}
