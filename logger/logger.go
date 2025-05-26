package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
	// "go.elastic.co/apm/module/apmzap/v2"
)

var errorLogger *zap.SugaredLogger
var logger *zap.Logger
// var appConfig = config.InitConfig()

var levelMap = map[string]zapcore.Level{
	"debug":  zapcore.DebugLevel,
	"info":   zapcore.InfoLevel,
	"warn":   zapcore.WarnLevel,
	"error":  zapcore.ErrorLevel,
	"dpanic": zapcore.DPanicLevel,
	"panic":  zapcore.PanicLevel,
	"fatal":  zapcore.FatalLevel,
}

func GetLoggerLevel(lvl string) zapcore.Level {
	if level, ok := levelMap[lvl]; ok {
		return level
	}
	return zapcore.InfoLevel
}

func init() {
	level := GetLoggerLevel("debug")

	encoder := zap.NewProductionEncoderConfig()
	encoder.EncodeTime = zapcore.ISO8601TimeEncoder
	core := zapcore.NewTee(
		zapcore.NewCore(zapcore.NewJSONEncoder(encoder), zapcore.AddSync(os.Stdout), level),
	)
	logger = zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))
	errorLogger = logger.Sugar()
}

func WithTrace(messsage string, fields ...interface{}) {
	errorLogger.With(fields).Debug(messsage)
}

func Debug(args ...interface{}) {
	errorLogger.Debug(args...)
}

func Debugf(template string, args ...interface{}) {
	errorLogger.Debugf(template, args...)
}

func Info(args ...interface{}) {
	errorLogger.Info(args...)
}

func Infof(template string, args ...interface{}) {
	errorLogger.Infof(template, args...)
}

func Warn(args ...interface{}) {
	errorLogger.Warn(args...)
}

func Warnf(template string, args ...interface{}) {
	errorLogger.Warnf(template, args...)
}

func Error(args ...interface{}) {
	errorLogger.Error(args...)
}

func Errorf(template string, args ...interface{}) {
	errorLogger.Errorf(template, args...)
}

func DPanic(args ...interface{}) {
	errorLogger.DPanic(args...)
}

func DPanicf(template string, args ...interface{}) {
	errorLogger.DPanicf(template, args...)
}

func Panic(args ...interface{}) {
	errorLogger.Panic(args...)
}

func Panicf(template string, args ...interface{}) {
	errorLogger.Panicf(template, args...)
}

func Fatal(args ...interface{}) {
	errorLogger.Fatal(args...)
}

func Fatalf(template string, args ...interface{}) {
	errorLogger.Fatalf(template, args...)
}
