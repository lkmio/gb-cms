package main

import (
	"context"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
	"time"
)

const (
	DBNAME = "lkm_gb.db"
	//DBNAME = ":memory:"
)

var (
	db          *gorm.DB
	TaskQueue   = make(chan *SaveTask, 1024)
	DeviceDao   = &daoDevice{}
	ChannelDao  = &daoChannel{}
	PlatformDao = &daoPlatform{}
	StreamDao   = &daoStream{}
	SinkDao     = &daoSink{}
)

func init() {
	db_, err := gorm.Open(sqlite.Open(DBNAME), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true,
			TablePrefix:   "lkm_",
		},
	})

	if err != nil {
		panic(err)
	}

	db = db_

	tx := db.Exec("PRAGMA journal_mode=WAL;")
	if tx.Error != nil {
		panic(tx.Error)
	}

	// 每次启动释放空间
	tx = db.Exec("VACUUM;")
	if tx.Error != nil {
		panic(tx.Error)
	}

	s, err := db.DB()
	s.SetMaxOpenConns(40)
	s.SetMaxIdleConns(10)

	// devices
	// channels
	// platforms
	// streams
	// sinks
	if err = db.AutoMigrate(&Device{}); err != nil {
		panic(err)
	} else if err = db.AutoMigrate(&Channel{}); err != nil {
		panic(err)
	} else if err = db.AutoMigrate(&SIPUAParams{}); err != nil {
		panic(err)
	} else if err = db.AutoMigrate(&Stream{}); err != nil {
		panic(err)
	} else if err = db.AutoMigrate(&Sink{}); err != nil {
		panic(err)
	} else if err = db.AutoMigrate(&DBPlatformChannel{}); err != nil {
		panic(err)
	}

	StartSaveTask()
}

type SaveTask struct {
	cb     func(tx *gorm.DB) error
	err    error
	cancel context.CancelFunc
}

func StartSaveTask() {
	go func() {
		for {
			var tasks []*SaveTask
			for len(TaskQueue) > 0 {
				select {
				case task := <-TaskQueue:
					tasks = append(tasks, task)
				}
			}

			if len(tasks) == 0 {
				time.Sleep(50 * time.Millisecond)
				continue
			}

			err := db.Transaction(func(tx *gorm.DB) error {
				for _, task := range tasks {
					task.err = task.cb(tx)
				}
				return nil
			})

			if err != nil {
				Sugar.Errorf("DBTransaction error: %s", err)
			}

			for _, task := range tasks {
				task.cancel()
			}
		}
	}()
}

func DBTransaction(cb func(tx *gorm.DB) error) error {
	ctx, cancel := context.WithCancel(context.Background())
	task := &SaveTask{
		cb:     cb,
		cancel: cancel,
	}

	TaskQueue <- task
	<-ctx.Done()
	return task.err
}

// OnExpires Redis设备ID到期回调
func OnExpires(db int, id string) {
	Sugar.Infof("设备心跳过期 device: %s", id)

	device, _ := DeviceDao.QueryDevice(id)
	if device == nil {
		Sugar.Errorf("设备不存在 device: %s", id)
		return
	}

	device.Close()
}
