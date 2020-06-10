package logger

import (
	"fmt"
	"log"
	"regexp"
	"sync/atomic"

	"github.com/cornelk/hashmap"
)

const (
	ERROR   = 1
	INFO    = 2
	VERBOSE = 3
	DEBUG   = 7
)

// FIXME GLOBAL VARAIBLES

var (
	level   int
	limiter int
	filter  *regexp.Regexp
	counter *hashmap.HashMap
)

func init() {
	counter = &hashmap.HashMap{}
}

func SetLevel(l int) {
	level = l
}

func SetLimiter(l int) {
	limiter = l
}

func SetFilter(pattern string) error {
	if pattern == "" {
		return nil
	}
	// https://github.com/google/re2/wiki/Syntax
	reg, err := regexp.Compile(pattern)
	if err != nil {
		return err
	}
	filter = reg
	return nil
}

func Println(v ...interface{}) {
	if level >= INFO {
		log.Println(v...)
	}
}

func Printf(format string, v ...interface{}) {
	if level >= INFO {
		log.Printf(format, v...)
	}
}

func Verbosef(format string, v ...interface{}) {
	printfAtLevel(VERBOSE, format, v...)
}

func Debugf(format string, v ...interface{}) {
	printfAtLevel(DEBUG, format, v...)
}

func printfAtLevel(l int, format string, v ...interface{}) {
	if level < l {
		return
	}
	out := filterOutput(format, v...)
	if out == "" {
		return
	}
	if !limiterAvailable(out) {
		return
	}
	log.Print(out)
}

func limiterAvailable(out string) bool {
	if limiter == 0 {
		return true
	}
	var i int64
	val, _ := counter.GetOrInsert(out, &i)
	actual := (val).(*int64)
	count := atomic.LoadInt64(actual)
	atomic.AddInt64(actual, 1)
	return count < int64(limiter)
}

func filterOutput(format string, v ...interface{}) string {
	out := fmt.Sprintf(format, v...)
	if filter == nil || filter.MatchString(out) {
		return out
	}
	return ""
}
