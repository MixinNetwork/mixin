package internal

import (
	"strings"

	"github.com/MixinNetwork/mixin/config"
)

var (
	inTest             bool
	mockRunAggregators bool
)

func init() {
	inTest = strings.Contains(config.BuildVersion, "BUILD_VERSION")
	mockRunAggregators = false
}

func MockRunAggregators() bool {
	return inTest && mockRunAggregators
}

func ToggleMockRunAggregators(mock bool) {
	mockRunAggregators = mock
}
