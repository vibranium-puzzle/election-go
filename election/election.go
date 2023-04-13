package election

import "time"

type Status string

const (
	Leader   Status = "leader"
	Follower Status = "follower"
)

type ElectListener interface {
	BecomeLeader()
	BecomeFollower()
}

type ElectConfig struct {
	ElectionListener     ElectListener
	FloatIpConfigList    []FloatIpConfig
	FloatIPCheckInterval time.Duration
}
