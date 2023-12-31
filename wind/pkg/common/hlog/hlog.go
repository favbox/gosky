package hlog

import (
	"io"
	"log"
	"os"
)

var (
	// 提供默认记录器供使用
	logger FullLogger = &defaultLogger{
		std:   log.New(os.Stderr, "", log.LstdFlags|log.Lshortfile|log.Lmicroseconds),
		depth: 4,
	}

	// 提供系统记录器供使用
	sysLogger FullLogger = &systemLogger{
		logger: &defaultLogger{
			std:   log.New(os.Stderr, "", log.LstdFlags|log.Lshortfile|log.Lmicroseconds),
			depth: 4,
		},
		prefix: systemLogPrefix,
	}
)

// SetOutput 设置默认记录器和系统记录器的输出器。
// 默认为 os.Stderr。
func SetOutput(w io.Writer) {
	logger.SetOutput(w)
	sysLogger.SetOutput(w)
}

// SetLevel 设置日志级别，低于该级别将不会输出日志。
// 默认记录器和系统记录器的默认记录级别为 LevelTrace。
// 注意：该方法非并发安全。
func SetLevel(lv Level) {
	logger.SetLevel(lv)
}

// DefaultLogger 返回 wind 的默认记录器。
func DefaultLogger() FullLogger {
	return logger
}

// SystemLogger 返回 wind 系统日志记录器。
// 该函数不建议业务端使用。
func SystemLogger() FullLogger {
	return sysLogger
}

// SetSystemLogger 设置系统记录器。
// 注意：该方法非并发安全，在使用该包中 SystemLogger 和全局函数后不得调用。
func SetSystemLogger(v FullLogger) {
	sysLogger = &systemLogger{
		logger: v,
		prefix: systemLogPrefix,
	}
}

// SetLogger 设置默认记录器和系统记录器。
// 注意：该方法非并发安全，在使用该包中 DefaultLogger 或 SystemLogger 或 全局函数后不得调用。
func SetLogger(v FullLogger) {
	logger = v
	SetSystemLogger(v)
}
