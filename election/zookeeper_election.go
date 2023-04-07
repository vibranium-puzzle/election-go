package election

import (
	"errors"
	"fmt"
	"github.com/go-zookeeper/zk"
	"github.com/vibranium-puzzle/election-go/shell"
	"log"
	"net"
	"sort"
	"strings"
	"sync"
	"time"
)

const ZkLockName = "latch-"

type ZooKeeperElectConfig struct {
	ZkServers       []string
	Path            string
	SessionTimeout  time.Duration
	PollPeriod      time.Duration
	ConnectionSleep time.Duration
}

type Participant struct {
	Id       string
	IsLeader bool
}

type ZooKeeperElection struct {
	config        ElectConfig
	zkConfig      ZooKeeperElectConfig
	startOnce     sync.Once
	conn          *zk.Conn
	rwMutex       sync.RWMutex
	myNode        string
	participants  []Participant
	leader        Participant
	firstDetected bool
	hasLeadership bool
	stop          chan struct{}
}

func NewZooKeeperElection(config ElectConfig, zkConfig ZooKeeperElectConfig) *ZooKeeperElection {
	if config.floatIPCheckInterval <= 0 {
		config.floatIPCheckInterval = 1 * time.Minute
	}
	if zkConfig.SessionTimeout <= 0 {
		zkConfig.SessionTimeout = 30 * time.Second
	}
	if zkConfig.PollPeriod <= 0 {
		zkConfig.PollPeriod = 5 * time.Second
	}
	if zkConfig.ConnectionSleep <= 0 {
		zkConfig.ConnectionSleep = 5 * time.Second
	}
	return &ZooKeeperElection{
		config:   config,
		zkConfig: zkConfig,
		stop:     make(chan struct{}),
	}
}

func (z *ZooKeeperElection) Start() {
	z.startOnce.Do(func() {
		go z.checkFloatIPs()
	})
	z.ensurePathExists()
	log.Printf("Starting ZooKeeper election")
	for {
		conn, sessionEvents, err := zk.Connect(z.zkConfig.ZkServers, time.Second)
		if err != nil {
			log.Printf("Unable to connect to Zookeeper: %v", err)
			time.Sleep(z.zkConfig.ConnectionSleep)
			continue
		}

		z.conn = conn

		go z.handleConnectionEvents(sessionEvents)

		time.Sleep(3 * time.Second)

		for {
			if err := z.createNode(); err != nil {
				log.Printf("Error creating node: %v", err)
				continue
			}

			break
		}

		go z.pollNodeList()

		break
	}
}

func (z *ZooKeeperElection) ensurePathExists() {
	conn, _, err := zk.Connect(z.zkConfig.ZkServers, z.zkConfig.SessionTimeout)
	if err != nil {
		log.Printf("Unable to connect to Zookeeper: %v", err)
		time.Sleep(z.zkConfig.ConnectionSleep)
		return
	}
	defer conn.Close()

	time.Sleep(3 * time.Second)

	if err := ensurePath(conn, z.zkConfig.Path); err != nil {
		log.Printf("Unable to ensure path exists: %v", err)
	}
}

// ensurePath creates the necessary nodes in ZooKeeper recursively.
func ensurePath(conn *zk.Conn, path string) error {
	if path == "" || path == "/" {
		return nil
	}

	parentPath := path[:strings.LastIndex(path, "/")]

	// Check if the parent path is empty or root, in which case it doesn't need to be created
	if parentPath != "" && parentPath != "/" {
		// Ensure the parent path exists
		err := ensurePath(conn, parentPath)
		if err != nil {
			return err
		}

		_, err = conn.Create(parentPath, []byte{}, 0, zk.WorldACL(zk.PermAll))
		if err != nil && err != zk.ErrNodeExists {
			return errors.New("failed to ensure the parent path exists")
		}
	}

	// Create the specified path itself
	_, err := conn.Create(path, []byte{}, 0, zk.WorldACL(zk.PermAll))
	if err != nil && err != zk.ErrNodeExists {
		return errors.New("failed to ensure the path itself exists")
	}

	return nil
}

func (z *ZooKeeperElection) handleConnectionEvents(sessionEvents <-chan zk.Event) {
	for {
		select {
		case <-z.stop:
			return
		case event := <-sessionEvents:
			switch event.State {
			case zk.StateDisconnected:
				log.Println("Disconnected from Zookeeper")
			case zk.StateConnected:
				log.Println("Connected to Zookeeper")
			case zk.StateExpired:
				log.Println("Zookeeper session expired, attempting to reconnect")
				z.Start()
			}
		}
	}
}

func (z *ZooKeeperElection) createNode() error {
	nodePath, err := z.conn.Create(z.zkConfig.Path+"/"+ZkLockName, []byte{}, zk.FlagEphemeral|zk.FlagSequence, zk.WorldACL(zk.PermAll))
	if err != nil {
		return fmt.Errorf("unable to create a node: %v", err)
	}
	z.myNode = strings.TrimPrefix(nodePath, z.zkConfig.Path+"/")
	return nil
}

func (z *ZooKeeperElection) electLeader() error {
	z.rwMutex.Lock()
	defer z.rwMutex.Unlock()
	children, _, err := z.conn.Children(z.zkConfig.Path)
	if err != nil {
		return fmt.Errorf("unable to get children nodes: %v", err)
	}

	sort.Strings(children)
	z.participants = make([]Participant, len(children))

	prevHasLeadership := z.hasLeadership

	for i, child := range children {
		z.participants[i] = Participant{child, i == 0}
		if child == z.myNode {
			z.hasLeadership = i == 0
			z.leader = z.participants[0]
		}
	}

	if !z.firstDetected {
		if z.hasLeadership && !prevHasLeadership {
			log.Printf("Becoming leader (Node: %s)\n", z.myNode)
			z.upFloatIPs()
			z.arpFloatIPs()
			if z.config.ElectionListener != nil {
				z.config.ElectionListener.BecomeLeader()
			}
		} else if !z.hasLeadership && prevHasLeadership {
			log.Printf("Becoming follower (Node: %s, Leader: %v)\n", z.myNode, z.leader)
			z.downFloatIPs()
			if z.config.ElectionListener != nil {
				z.config.ElectionListener.BecomeFollower()
			}
		}
		z.firstDetected = true
	}
	return nil
}

func (z *ZooKeeperElection) pollNodeList() {
	ticker := time.NewTicker(z.zkConfig.PollPeriod)

	for {
		select {
		case <-z.stop:
			ticker.Stop()
			return
		case <-ticker.C:
			if err := z.electLeader(); err != nil {
				log.Printf("Error during leader election: %v", err)
				continue
			}
		}
	}
}

func (z *ZooKeeperElection) GetParticipants() ([]Participant, error) {
	z.rwMutex.RLock()
	defer z.rwMutex.RUnlock()
	if z.conn == nil || z.conn.State() != zk.StateConnected {
		return nil, fmt.Errorf("not connected to Zookeeper")
	}
	return z.participants, nil
}

func (z *ZooKeeperElection) GetLeader() (Participant, error) {
	z.rwMutex.RLock()
	defer z.rwMutex.RUnlock()
	if z.conn == nil || z.conn.State() != zk.StateConnected {
		return Participant{}, fmt.Errorf("not connected to Zookeeper")
	}
	return z.leader, nil
}

func (z *ZooKeeperElection) HasLeadership() bool {
	z.rwMutex.RLock()
	defer z.rwMutex.RUnlock()
	return z.hasLeadership
}

func (z *ZooKeeperElection) checkFloatIPs() {
	for {
		time.Sleep(z.config.floatIPCheckInterval)

		z.rwMutex.RLock()
		isLeader := z.hasLeadership
		z.rwMutex.RUnlock()

		if isLeader {
			z.ensureFloatIPsExist()
		} else {
			z.ensureFloatIPsNotExist()
		}
	}
}

func (z *ZooKeeperElection) ensureFloatIPsExist() {
	for _, floatIPConfig := range z.config.FloatIpConfigList {
		exists, err := isFloatIPExist(floatIPConfig.Dev, floatIPConfig.VirtualIp)
		if err != nil {
			log.Printf("Error checking float IP %s on device %s: %v\n", floatIPConfig.VirtualIp, floatIPConfig.Dev, err)
			continue
		}

		if !exists {
			log.Printf("Adding float IP %s on device %s\n", floatIPConfig.VirtualIp, floatIPConfig.Dev)
			cmd := floatIPConfig.UpCmd()
			rc, stdout, stderr, err := shell.Exec(cmd)
			if err != nil {
				log.Printf("Error running command '%s': %s", cmd, err)
			} else if rc != 0 {
				log.Printf("Command '%s' exited with non-zero return code %d. Stdout: '%s'. Stderr: '%s'", cmd, rc, stdout, stderr)
			} else {
				log.Printf("Command '%s' completed successfully. Stdout: '%s'. Stderr: '%s'", cmd, stdout, stderr)
			}
		}
	}
}

func (z *ZooKeeperElection) ensureFloatIPsNotExist() {
	for _, floatIPConfig := range z.config.FloatIpConfigList {
		exists, err := isFloatIPExist(floatIPConfig.Dev, floatIPConfig.VirtualIp)
		if err != nil {
			log.Printf("Error checking float IP %s on device %s: %v\n", floatIPConfig.VirtualIp, floatIPConfig.Dev, err)
			continue
		}

		if exists {
			log.Printf("Removing float IP %s from device %s\n", floatIPConfig.VirtualIp, floatIPConfig.Dev)
			cmd := floatIPConfig.DownCmd()
			rc, stdout, stderr, err := shell.Exec(cmd)
			if err != nil {
				log.Printf("Error running command '%s': %s", cmd, err)
			} else if rc != 0 {
				log.Printf("Command '%s' exited with non-zero return code %d. Stdout: '%s'. Stderr: '%s'", cmd, rc, stdout, stderr)
			} else {
				log.Printf("Command '%s' completed successfully. Stdout: '%s'. Stderr: '%s'", cmd, stdout, stderr)
			}
		}
	}
}

func isFloatIPExist(dev, floatIP string) (bool, error) {
	iface, err := net.InterfaceByName(dev)
	if err != nil {
		return false, fmt.Errorf("error getting interface by name: %v", err)
	}

	addrs, err := iface.Addrs()
	if err != nil {
		return false, fmt.Errorf("error getting addresses for interface: %v", err)
	}

	for _, addr := range addrs {
		if ip, _, err := net.ParseCIDR(addr.String()); err == nil {
			if ip.String() == floatIP {
				return true, nil
			}
		}
	}

	return false, nil
}

func (z *ZooKeeperElection) upFloatIPs() {
	for _, floatIPConfig := range z.config.FloatIpConfigList {
		cmd := floatIPConfig.UpCmd()
		log.Printf("Executing UpCmd: %s", cmd)
		rc, stdout, stderr, err := shell.Exec(cmd)
		if err != nil {
			log.Printf("Error running command '%s': %s", cmd, err)
		} else if rc != 0 {
			log.Printf("Command '%s' exited with non-zero return code %d. Stdout: '%s'. Stderr: '%s'", cmd, rc, stdout, stderr)
		} else {
			log.Printf("Command '%s' completed successfully. Stdout: '%s'. Stderr: '%s'", cmd, stdout, stderr)
		}
	}
}

func (z *ZooKeeperElection) downFloatIPs() {
	for _, floatIPConfig := range z.config.FloatIpConfigList {
		cmd := floatIPConfig.DownCmd()
		rc, stdout, stderr, err := shell.Exec(cmd)
		if err != nil {
			log.Printf("Error running command '%s': %s", cmd, err)
		} else if rc != 0 {
			log.Printf("Command '%s' exited with non-zero return code %d. Stdout: '%s'. Stderr: '%s'", cmd, rc, stdout, stderr)
		} else {
			log.Printf("Command '%s' completed successfully. Stdout: '%s'. Stderr: '%s'", cmd, stdout, stderr)
		}
	}
}

func (z *ZooKeeperElection) arpFloatIPs() {
	for _, floatIPConfig := range z.config.FloatIpConfigList {
		cmd := floatIPConfig.ArpCmd()
		rc, stdout, stderr, err := shell.Exec(cmd)
		if err != nil {
			log.Printf("Error running command '%s': %s", cmd, err)
		} else if rc != 0 {
			log.Printf("Command '%s' exited with non-zero return code %d. Stdout: '%s'. Stderr: '%s'", cmd, rc, stdout, stderr)
		} else {
			log.Printf("Command '%s' completed successfully. Stdout: '%s'. Stderr: '%s'", cmd, stdout, stderr)
		}
	}
}

func (z *ZooKeeperElection) Close() {
	z.rwMutex.Lock()
	z.hasLeadership = false
	z.rwMutex.Unlock()
	close(z.stop)
	if z.conn != nil {
		z.conn.Close()
	}
}
