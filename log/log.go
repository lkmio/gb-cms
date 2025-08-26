package log

import (
	"github.com/natefinch/lumberjack"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
)

var (
	Sugar *zap.SugaredLogger
)

func InitLogger(leve zapcore.LevelEnabler,
	name string, maxSize, maxBackup, maxAge int, compress bool) {

	var sinks []zapcore.Core
	writeSyncer := getLogWriter(name, maxSize, maxBackup, maxAge, compress)
	encoder := getEncoder()

	fileCore := zapcore.NewCore(encoder, writeSyncer, leve)

	sinks = append(sinks, fileCore)
	//打印到控制台
	sinks = append(sinks, zapcore.NewCore(encoder, zapcore.AddSync(os.Stdout), leve))

	core := zapcore.NewTee(sinks...)

	logger := zap.New(core, zap.AddCaller())
	Sugar = logger.Sugar()
}

func getEncoder() zapcore.Encoder {
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	return zapcore.NewConsoleEncoder(encoderConfig)
}

// 配置日志保存规则
// @name      日志文件名, 可包含路径
// @maxSize   单个日志文件最大大小(M)
// @maxBackup 日志文件最多生成多少个
// @maxAge	  日志文件最多保存多少天
func getLogWriter(name string, maxSize, maxBackup, maxAge int, compress bool) zapcore.WriteSyncer {
	lumberJackLogger := &lumberjack.Logger{
		Filename:   name,
		MaxSize:    maxSize,
		MaxBackups: maxBackup,
		MaxAge:     maxAge,
		Compress:   compress,
	}
	return zapcore.AddSync(lumberJackLogger)
}
