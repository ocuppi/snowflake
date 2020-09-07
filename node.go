package snowflake

import (
	"errors"
	"sync"
	"time"
)

var (
	ErrorTimeOverflow      = errors.New("time overflowed its bit allowance")
	ErrorNodeOverflow      = errors.New("node ID overflowed its bit allowance")
	ErrorSnowflakeOverflow = errors.New("total bits allocated is greater than 63")
)

type Node struct {
	mutex        sync.Mutex
	lastGenerate int64 // how many ms since epoch the last ID was generated

	epoch   time.Time
	id      int64
	counter int64

	timeBits  uint8
	nodeBits  uint8
	countBits uint8

	maxTime    int64
	maxNode    int64
	maxCounter int64
}

func NewNode(nodeID uint32, epoch time.Time, timeBits, nodeBits, counterBits uint8) (*Node, error) {
	if timeBits+nodeBits+counterBits > 63 {
		return nil, ErrorSnowflakeOverflow
	}
	n := &Node{}
	n.mutex.Lock() // Do not allow ID generation during setup
	now := time.Now()
	n.epoch = now.Add(epoch.Sub(now)) // force monotonic clock usage to avoid
	n.id = int64(nodeID)

	n.timeBits = timeBits
	n.nodeBits = nodeBits
	n.countBits = counterBits

	n.maxTime = maxValueBits(timeBits)
	n.maxNode = maxValueBits(nodeBits)
	n.maxCounter = maxValueBits(counterBits)

	n.lastGenerate = n.msSinceEpoch()

	if int64(nodeID) > n.maxNode {
		return nil, ErrorNodeOverflow
	}
	n.mutex.Unlock()
	return n, nil
}

// Generate creates a snowflake based off the state of the Node. Panics if the time is greater than the
// allocated bit count to prevent incorrect ID generation. This should only happen if the node was created with
// insufficient space for the time. Generate will wait for the next millisecond, should it generate 2^countBits
// Snowflakes in the same millisecond.
// For example, if timeBits is 32, any ids created after epoch + 1.08 years would panic.
func (n *Node) Generate() Snowflake {
	n.mutex.Lock()
	defer n.mutex.Unlock()
	nowMs := n.msSinceEpoch()
	if nowMs == n.lastGenerate {
		n.counter++
		if n.counter > n.maxCounter {
			for nowMs <= n.lastGenerate { // wait until the next millisecond
				nowMs = n.msSinceEpoch()
				n.counter = 0
			}
		}
	} else {
		n.counter = 0
	}
	// check if the time would overflow the bits allocated to it.
	if nowMs > n.maxTime {
		panic(ErrorTimeOverflow)
	}
	n.lastGenerate = nowMs
	return Snowflake(nowMs<<(63-n.timeBits) | n.id<<(63-n.countBits) | n.counter)
}

func (n *Node) msSinceEpoch() int64 {
	return time.Since(n.epoch).Nanoseconds() / 1e6
}

func maxValueBits(n uint8) int64 {
	return -1 ^ (-1 << n)
}
