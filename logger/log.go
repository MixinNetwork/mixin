package logger

import "log"

func Println(v ...interface{}) {
	log.Println(v...)
}

func Printf(format string, v ...interface{}) {
	log.Printf(format, v...)
}

func Verbosef(format string, v ...interface{}) {
	log.Printf(format, v...)
}
