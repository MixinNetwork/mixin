package kernel

import (
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDeterminBestRound(t *testing.T) {
	assert := assert.New(t)

	root, err := ioutil.TempDir("", "mixin-self-test")
	assert.Nil(err)
	defer os.RemoveAll(root)

	node := setupTestNode(assert, root)
	assert.NotNil(node)

	best := node.determinBestRound(node.IdForNetwork, uint64(time.Now().UnixNano()))
	assert.NotNil(best)
}
