package dao

import (
	"context"
	"gb-cms/log"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
	"os"
	"time"
)

const (
	DBNAME = "data/lkm_gb.db"
	//DBNAME = ":memory:"
)

var (
	db        *gorm.DB
	TaskQueue = make(chan *SaveTask, 1024)
	Device    = &daoDevice{}
	Channel   = &daoChannel{}
	Platform  = &daoPlatform{}
	Stream    = &daoStream{}
	Sink      = &daoSink{}
	JTDevice  = &daoJTDevice{}
	Blacklist = &daoBlacklist{}
)

func init() {
	// 创建data目录
	err := os.MkdirAll("./data", 0755)
	if err != nil {
		panic(err)
	}

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
	if err = db.AutoMigrate(&DeviceModel{}); err != nil {
		panic(err)
	} else if err = db.AutoMigrate(&ChannelModel{}); err != nil {
		panic(err)
	} else if err = db.AutoMigrate(&PlatformModel{}); err != nil {
		panic(err)
	} else if err = db.AutoMigrate(&StreamModel{}); err != nil {
		panic(err)
	} else if err = db.AutoMigrate(&SinkModel{}); err != nil {
		panic(err)
	} else if err = db.AutoMigrate(&PlatformChannelModel{}); err != nil {
		panic(err)
	} else if err = db.AutoMigrate(&JTDeviceModel{}); err != nil {
		panic(err)
	} else if err = db.AutoMigrate(&BlacklistModel{}); err != nil {
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
				log.Sugar.Errorf("DBTransaction error: %s", err)
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
