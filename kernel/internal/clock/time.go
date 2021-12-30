package clock

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/MixinNetwork/mixin/config"
)

// FIXME GLOBAL VARAIBLES

var (
	inTest   bool
	mutex    *sync.RWMutex
	mockDiff time.Duration
)

func init() {
	inTest = strings.Contains(config.BuildVersion, "BUILD_VERSION")
	mutex = new(sync.RWMutex)
	mockDiff = 0
}

func Reset() {
	if !inTest {
		panic(fmt.Errorf("clock reset not allowed in build version %s", config.BuildVersion))
	}

	mutex.Lock()
	defer mutex.Unlock()
	mockDiff = 0
}

func MockDiff(at time.Duration) {
	if !inTest {
		panic(fmt.Errorf("clock mock not allowed in build version %s", config.BuildVersion))
	}

	mutex.Lock()
	defer mutex.Unlock()
	mockDiff += at
}

func Now() time.Time {
	if !inTest {
		return time.Now()
	}

	mutex.RLock()
	defer mutex.RUnlock()
	return time.Now().Add(mockDiff)
}
