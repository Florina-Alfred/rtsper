package log

import (
	"log"
)

func Info(format string, v ...interface{}) {
	log.Printf("INFO: "+format, v...)
}

func Warn(format string, v ...interface{}) {
	log.Printf("WARN: "+format, v...)
}

func Error(format string, v ...interface{}) {
	log.Printf("ERROR: "+format, v...)
}
