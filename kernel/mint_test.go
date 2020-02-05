package kernel

import (
	"testing"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/stretchr/testify/assert"
)

func TestPledgeAmount(t *testing.T) {
	assert := assert.New(t)

	for b := 0; b < 365; b++ {
		since := time.Duration(b*24) * time.Hour
		assert.Equal(common.NewInteger(10000), pledgeAmount(since))
	}
	for b := 365; b < 365*2; b++ {
		since := time.Duration(b*24) * time.Hour
		assert.Equal(common.NewInteger(11000), pledgeAmount(since))
	}
	for b := 365 * 2; b < 365*3; b++ {
		since := time.Duration(b*24) * time.Hour
		assert.Equal(common.NewInteger(11900), pledgeAmount(since))
	}
	for b := 365 * 3; b < 365*4; b++ {
		since := time.Duration(b*24) * time.Hour
		assert.Equal(common.NewInteger(12710), pledgeAmount(since))
	}
	for b := 365 * 5; b < 365*6; b++ {
		since := time.Duration(b*24) * time.Hour
		assert.Equal(common.NewIntegerFromString("14095.1"), pledgeAmount(since))
	}
	for b := 365 * 7; b < 365*8; b++ {
		since := time.Duration(b*24) * time.Hour
		assert.Equal(common.NewIntegerFromString("15217.031"), pledgeAmount(since))
	}
	for b := 365 * 10; b < 365*11; b++ {
		since := time.Duration(b*24) * time.Hour
		assert.Equal(common.NewIntegerFromString("16513.215599"), pledgeAmount(since))
	}
}
