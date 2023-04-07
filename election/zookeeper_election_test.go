package election

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestZooKeeperElection_Creation(t *testing.T) {
	config := ElectConfig{}

	zkConfig := ZooKeeperElectConfig{
		ZkServers:       []string{"127.0.0.1:2181"},
		Path:            "/election/Creation",
		SessionTimeout:  5 * time.Second,
		PollPeriod:      5 * time.Second,
		ConnectionSleep: 5 * time.Second,
	}

	z := NewZooKeeperElection(config, zkConfig)
	defer z.Close()
}

func TestZooKeeperElection_ElectLeader(t *testing.T) {
	// Initialize your ZooKeeper server and client connections here.
	// Then, create two ZooKeeperElection instances.
	// You may want to create a custom listener that can notify the test when an instance becomes leader or follower.

	electConfig := ElectConfig{}

	zkConfig := ZooKeeperElectConfig{
		ZkServers:       []string{"127.0.0.1:2181"},
		Path:            "/election/ElectLeader",
		SessionTimeout:  5 * time.Second,
		PollPeriod:      5 * time.Second,
		ConnectionSleep: 5 * time.Second,
	}

	election1 := NewZooKeeperElection(electConfig, zkConfig)
	election2 := NewZooKeeperElection(electConfig, zkConfig)

	election1.Start()
	election2.Start()

	// Wait for the leader election to complete.
	time.Sleep(10 * time.Second)

	// Check if only one instance has leadership.
	assert.True(t, (election1.HasLeadership() && !election2.HasLeadership()) || (!election1.HasLeadership() && election2.HasLeadership()))

	election1.Close()
	election2.Close()
}

func TestZooKeeperElection_LeaderReelection(t *testing.T) {
	// Initialize your ZooKeeper server and client connections here.
	// Then, create three ZooKeeperElection instances.
	// You may want to create a custom listener that can notify the test when an instance becomes leader or follower.

	electConfig := ElectConfig{}
	zkConfig := ZooKeeperElectConfig{
		ZkServers:       []string{"127.0.0.1:2181"},
		Path:            "/election/LeaderReelection",
		SessionTimeout:  5 * time.Second,
		PollPeriod:      5 * time.Second,
		ConnectionSleep: 5 * time.Second,
	}

	election1 := NewZooKeeperElection(electConfig, zkConfig)
	election2 := NewZooKeeperElection(electConfig, zkConfig)
	election3 := NewZooKeeperElection(electConfig, zkConfig)

	election1.Start()
	election2.Start()
	election3.Start()

	// Wait for the leader election to complete.
	time.Sleep(10 * time.Second)

	// Determine the current leader.
	var leader *ZooKeeperElection
	if election1.HasLeadership() {
		leader = election1
	} else if election2.HasLeadership() {
		leader = election2
	} else {
		leader = election3
	}

	// Close the current leader.
	leader.Close()

	// Wait for a new leader election to complete.
	time.Sleep(10 * time.Second)

	fmt.Println("election1:", election1.HasLeadership())
	fmt.Println("election2:", election2.HasLeadership())
	fmt.Println("election3:", election3.HasLeadership())
	// Check if only one instance has leadership.
	assert.True(t, (election1.HasLeadership() && !election2.HasLeadership() && !election3.HasLeadership()) ||
		(!election1.HasLeadership() && election2.HasLeadership() && !election3.HasLeadership()) ||
		(!election1.HasLeadership() && !election2.HasLeadership() && election3.HasLeadership()))

	if leader != election1 {
		election1.Close()
	}
	if leader != election2 {
		election2.Close()
	}
	if leader != election3 {
		election3.Close()

	}
}
