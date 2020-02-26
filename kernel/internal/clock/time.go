package clock

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/MixinNetwork/mixin/config"
)

var (
	mutex    *sync.RWMutex
	mockDiff time.Duration
)

func init() {
	mutex = new(sync.RWMutex)
	mockDiff = 0
}

func MockDiff(at time.Duration) {
	if !inTest() {
		panic(fmt.Errorf("clock mock not allowed in environment %s or build version %s", config.Custom.Environment, config.BuildVersion))
	}

	mutex.Lock()
	defer mutex.Unlock()
	mockDiff += at
}

func Now() time.Time {
	if !inTest() {
		return time.Now()
	}

	mutex.RLock()
	defer mutex.RUnlock()
	return time.Now().Add(mockDiff)
}

func inTest() bool {
	return config.Custom.Environment == "test" && strings.Contains(config.BuildVersion, "BUILD_VERSION")
}
