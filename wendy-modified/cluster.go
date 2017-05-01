package wendy

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"sync"
	"time"
	"crypto/cipher"
	"crypto/aes"
	"crypto/rand"
	"fmt"
	"io/ioutil"
)

type StateMask struct {
	Mask byte
	Rows []int
	Cols []int
}

const (
    nS = byte(1)
    all = nS
)

type stateTables struct {
	NeighborhoodSet *[]*Node     `json:"ns,omitempty"`
	EOL             bool           `json:"eol,omitempty"`
}

// Cluster holds the information about the state of the network. It is the main interface to the distributed network of Nodes.
type Cluster struct {
	self               *Node
	Neighborhoodset    *NeighborhoodSet
	kill               chan bool
	lastStateUpdate    time.Time
	applications       []Application
	log                *log.Logger
	logLevel           int
	heartbeatFrequency int
	networkTimeout     int
	credentials        Credentials
	joined             bool
	lock               *sync.RWMutex
    working            bool
    blocked            bool
    waitQueue          []*Message
}

//newLeaves call OnNewLeaves handler
func (c *Cluster) newLeaves(leaves []*Node) {
	// c.lock.RLock()
	// defer c.lock.RUnlock()
	// c.debug("[newLeaves]Sending newLeaves notifications.")
	for _, app := range c.applications {
		app.OnNewLeaves(leaves)
		// c.debug("[newLeaves]Sent newLeaves notification %d of %d.", i+1, len(c.applications))
	}
	// c.debug("[newLeaves]Sent newLeaves notifications.")
}
//fanoutjoin call OnNodeJoin handler
func (c *Cluster) fanOutJoin(node Node) {
	// c.lock.RLock()
	// defer c.lock.RUnlock()
	for _, app := range c.applications {
		// c.debug("[fanOutJoin]Announcing node join.")
		app.OnNodeJoin(node)
		// c.debug("[fanOutJoin]Announced node join.")
	}
}

func (c *Cluster) marshalCredentials() []byte {
	// c.lock.RLock()
	// defer c.lock.RUnlock()
	if c.credentials == nil {
		return []byte{}
	}
	return c.credentials.Marshal()
}

func (c *Cluster) getNetworkTimeout() int {
	// c.lock.RLock()
	// defer c.lock.RUnlock()
	return c.networkTimeout
}

func (c *Cluster) isJoined() bool {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.joined
}
func (c *Cluster) isBlocked() bool {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.blocked
}
func (c *Cluster) isWorking() bool {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.working
}

// ID returns an identifier for the Cluster. It uses the ID of the current Node.
func (c *Cluster) ID() NodeID {
	return c.self.ID
}

// String returns a string representation of the Cluster, in the form of its ID.
func (c *Cluster) String() string {
	return c.ID().String()
}

// GetIP returns the IP address to use when communicating with a Node.
func (c *Cluster) GetIP(node Node) string {
	return c.self.GetIP(node)
}

// SetLogger sets the log.Logger that the Cluster, along with its child routingTable and leafSet, will write to.
func (c *Cluster) SetLogger(l *log.Logger) {
	c.log = l
	// c.table.log = l
	// c.leafset.log = l
}

// SetLogLevel sets the level of logging that will be written to the Logger. It will be mirrored to the child routingTable and leafSet.
//
// Use wendy.LogLevelDebug to write to the most verbose level of logging, helpful for debugging.
//
// Use wendy.LogLevelWarn (the default) to write on events that may, but do not necessarily, indicate an error.
//
// Use wendy.LogLevelError to write only when an event occurs that is undoubtedly an error.
func (c *Cluster) SetLogLevel(level int) {
	c.logLevel = level
	// c.table.logLevel = level
	// c.leafset.logLevel = level
}

// SetHeartbeatFrequency sets the frequency in seconds with which heartbeats will be sent from this Node to test the health of other Nodes in the Cluster.
func (c *Cluster) SetHeartbeatFrequency(freq int) {
	c.heartbeatFrequency = freq
}

// SetNetworkTimeout sets the number of seconds before which network requests will be considered timed out and killed.
func (c *Cluster) SetNetworkTimeout(timeout int) {
	c.networkTimeout = timeout
}

// NewCluster creates a new instance of a connection to the network and intialises the state tables and channels it requires.
func NewCluster(self *Node, credentials Credentials) *Cluster {
	return &Cluster{
		self:               self,
		Neighborhoodset:    newNeighborhoodSet(self),
		kill:               make(chan bool),
		lastStateUpdate:    time.Now(),
		applications:       []Application{},
		log:                log.New(os.Stdout, "wendy("+self.ID.String()+") ", log.LstdFlags),
		logLevel:           LogLevelWarn,
		heartbeatFrequency: 60,
		networkTimeout:     5,
		credentials:        credentials,
		joined:             false,
		lock:               new(sync.RWMutex),
        working:            false,
        blocked:            false,
        waitQueue:          []*Message{},
	}
}

// Stop gracefully shuts down the local connection to the Cluster, removing the local Node from the Cluster and preventing it from receiving or sending further messages.
//
// Before it disconnects the Node, Stop contacts every Node it knows of to warn them of its departure. If a graceful disconnect is not necessary, Kill should be used instead. Nodes will remove the Node from their state tables next time they attempt to contact it.

func (c *Cluster) Stop() {
	c.debug("[Stop]Sending graceful exit message.")
	msg := c.NewMessage(NODE_EXIT, c.self.ID, []byte{})
    nodes := c.Neighborhoodset.list()
	for _, node := range nodes {
        //send stop message to every node in neighborhood
		err := c.send(msg, node)
		if err != nil {
			c.fanOutError(err)
		}
	}
	c.Kill()
}

// Kill shuts down the local connection to the Cluster, removing the local Node from the Cluster and preventing it from receiving or sending further messages.
//
// Unlike Stop, Kill immediately disconnects the Node without sending a message to let other Nodes know of its exit.
func (c *Cluster) Kill() {
	c.debug("[Kill]Exiting the cluster.")
	c.kill <- true
}

// RegisterCallback allows anything that fulfills the Application interface to be hooked into the Wendy's callbacks.
func (c *Cluster) RegisterCallback(app Application) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.applications = append(c.applications, app)
}

// Listen starts the Cluster listening for events, including all the individual listeners for each state sub-object.
//
// Note that Listen does *not* join a Node to the Cluster. The Node must announce its presence before the Node is considered active in the Cluster.
func (c *Cluster) Listen() error {
    // ln; ch<-conn
	portstr := strconv.Itoa(c.self.Port)
	c.debug("[Listen]Listening on port %d", c.self.Port)
	ln, err := net.Listen("tcp", ":"+portstr)//ln: listen fd
	if err != nil {
		return err
	}
	defer ln.Close()

	connections := make(chan net.Conn)
	go func(ln net.Listener, ch chan net.Conn) {
		for {
			conn, err := ln.Accept()
			if err != nil {
				c.fanOutError(err)
				return
			}
			// c.debug("[Listen]Connection received.")
			ch <- conn//ch : the connection
		}
	}(ln, connections)
    go func() {
        c.debug("[Listen]spin heartbeat thread")
		for {
            c.debug("[Listen]Send heartbeat")
            c.sendHeartbeats()
            time.Sleep(time.Duration(c.heartbeatFrequency) * time.Second)
		}
	}()
	for {
		select {
		case <-c.kill:
            c.debug("[Listen]calling c.kill")
			return nil
		case conn := <-connections:
			// c.debug("[Listen]Handling connection.")
			go c.handleClient(conn)
			break
		}
	}
	return nil
}

// Send routes a message through the Cluster.
func (c *Cluster) Send(msg Message) error {
	// c.debug("[Send]Getting target for message %s", msg.Key)
	target, err := c.Route(msg.Key)
	if err != nil {
		return err
	}
	// if target.ID.Equals(c.self.ID) && msg.Purpose != UNBLOCK_CLUSTER && msg.Purpose < WANT_TO_CRACK {
	// 	// c.debug("[Send]Couldn't find a target. Delivering message %s", msg.Key)
	// 	if msg.Purpose > NODE_ANN && msg.Purpose != UNBLOCK_CLUSTER {
	// 		c.deliver(msg)
	// 	}
	// 	return nil
	// }
	err = c.send(msg, target)
	if err == deadNodeError {
		// c.remove(target.ID)
        exitmsg := c.NewMessage(NODE_EXIT, target.ID, []byte{})
        snodes := c.Neighborhoodset.list()
        for _, snode := range snodes {
            c.debug("[Send] inform nodes someone exits")
            err := c.send(exitmsg, snode)
            if err != nil {
                c.fanOutError(err)
            }
            // if !(snode.ID.Equals(c.self.ID)) {
            //
            // }
        }
	}
	return err
}
// Route checks the leafSet and routingTable to see if there's an appropriate match for the NodeID. If there is a better match than the current Node, a pointer to that Node is returned. Otherwise, nil is returned (and the message should be delivered).
func (c *Cluster) Route(key NodeID) (*Node, error) {
    target, err := c.Neighborhoodset.route(key)
	if err != nil {
        // c.debug("[Route] route fails")
		if err != nodeNotFoundError {
            // c.debug("[Route] route fails for nodeNotFoundError")
			return nil, err
		}
        return nil, nil
	}
    if target != nil {
		// c.debug("[Route]Target acquired in neighborhood.")
		return target, nil
	}
	return nil, nil
}

// Join expresses a Node's desire to join the Cluster, kicking off a process that will populate its child leafSet, NeighborhoodSet and routingTable. Once that process is complete, the Node can be said to be fully participating in the Cluster.
//
// The IP and port passed to Join should be those of a known Node in the Cluster. The algorithm assumes that the known Node is close in proximity to the current Node, but that is not a hard requirement.
func (c *Cluster) Join(ip string, port int) error {
	credentials := c.marshalCredentials()
	c.debug("[Join]Sending join message to %s:%d", ip, port)
    //join message contains: NODE_JOIN, my ID, credentials
	msg := c.NewMessage(NODE_JOIN, c.self.ID, credentials)
	address := ip + ":" + strconv.Itoa(port)
	return c.SendToIP(msg, address)
}

func (c *Cluster) fanOutError(err error) {
	for _, app := range c.applications {
		app.OnError(err)
	}
}

func (c *Cluster) sendHeartbeats() {
	msg := c.NewMessage(HEARTBEAT, c.self.ID, []byte{})
    nodes := c.Neighborhoodset.HeartbeatReceivers()

	for _, node := range nodes {
		if node == nil {
			break
		}
		err := c.send(msg, node)
		if err == deadNodeError {
			// err = c.remove(node.ID)
			if err != nil {
				c.fanOutError(err)
			}
            exitmsg := c.NewMessage(NODE_EXIT, node.ID, []byte{})
            snodes := c.Neighborhoodset.list()
        	for _, snode := range snodes {
                c.debug("[sendHeartbeats] inform nodes exit")
                err := c.send(exitmsg, snode)
                if err != nil {
                    c.fanOutError(err)
                }
                // if !(snode.ID.Equals(c.self.ID)) {
                //
                // }
        	}
			continue
		}
	}
}
//to handler
func (c *Cluster) deliver(msg Message) {
	if msg.Purpose <= NODE_ANN {
		c.warn("[deliver]Received utility message %s to the deliver function. Purpose was %d.", msg.Key, msg.Purpose)
		return
	}
    //should unlock the cluster when calling deliver function
	// c.lock.RLock()
	// defer c.lock.RUnlock()
    if msg.Purpose == NEW_JOB {
        c.debug("[deliver] It is a new job message")
        c.lock.RLock()
        c.working = true
        c.lock.RUnlock()
    }
	for _, app := range c.applications {
		app.OnDeliver(msg)
	}
}
func (c *Cluster) deliverLeaderElection(msg Message) {
    c.debug("[deliverLeaderElection]")
	for _, app := range c.applications {
		app.OnLeaderElectionDeliver(msg)
	}
}
func (c *Cluster) deliverNewProposalfromClient(msg Message) {
    c.debug("[deliverNewProposalfromClient]")
	for _, app := range c.applications {
		app.OnNewProposalDeliver(msg)
	}
}
func (c *Cluster) deliverNewBackUpInitFromLeader(msg Message) {
    c.debug("[deliverNewBackUpInitFromLeaderl]")
	for _, app := range c.applications {
		app.OnNewBackUpInit(msg)
	}
}
func (c *Cluster) deliverFirstJobFromLeader(msg Message) {
    c.debug("[deliverFirstJobFromLeader]")
	for _, app := range c.applications {
		app.OnFirstJob(msg)
	}
}
func (c *Cluster) deliverFoundPass(msg Message) {
    c.debug("[deliverFoundPass]")
	for _, app := range c.applications {
		app.OnReceiveFoundPass(msg)
	}
}
func (c *Cluster) deliverAskAnotherPieceFromClient(msg Message) {
    c.debug("[deliverAskAnotherPieceFromClient]")
	for _, app := range c.applications {
		app.OnAskAnotherPiece(msg)
	}
}
func (c *Cluster) deliverRecvAnotherPieceFromLeader(msg Message) {
    c.debug("[deliverRecvAnotherPieceFromLeader]")
	for _, app := range c.applications {
		app.OnRecvAnotherPiece(msg)
	}
}
func (c *Cluster) deliverUpdateBackUpFromLeader(msg Message) {
    c.debug("[deliverUpdateBackUpFromLeader]")
	for _, app := range c.applications {
		app.OnBackUpRecvUpdate(msg)
	}
}
func (c *Cluster) deliverNodeDontWantToWorkFromClient(msg Message) {
    c.debug("[deliverNodeDontWantToWorkFromClient]")
    for _, app := range c.applications {
        app.OnNodeExit(msg.Sender)
    }
}


//initiate workmateset using current neighborhoodset
// func (c *Cluster) initWorkMateSet() {
//     c.debug("[initWorkMateSet] initiating working set")
//     nodes := c.GetListOfNodes()
//     c.WorkMateSet.dumpNodes(nodes)
// }
func (c *Cluster) handleClient(conn net.Conn) {
	defer conn.Close()

	// Steps: decoding -> decrypting -> unmarshalling

	//1. decoding
	var encryptedMsg []byte
	decoder := json.NewDecoder(conn)
	err := decoder.Decode(&encryptedMsg)

	//2. extracting key and then decrypting
	key, err := ioutil.ReadFile("key.txt")
	if err != nil {
		fmt.Print(err)
	}
	decryptedmsg, err:= decrypt(encryptedMsg, key)
	if err != nil {
		return
	}

	//3. unmarshalling
	var msg Message
	err = json.Unmarshal(decryptedmsg, &msg)

	if err != nil {
		c.fanOutError(err)
		return
	}

	valid := c.credentials == nil
	if !valid {
		//fmt.Println("Credentials being received is", msg.Credentials)
		valid = c.credentials.Valid(msg.Credentials)
//		fmt.Println("Are credentials valid?", valid)
	}
	if !valid {
		c.warn("Credentials did not match. Supplied credentials: %s", msg.Credentials)
		return
	}
	if msg.Purpose != NODE_JOIN {
		node, _ := c.get(msg.Sender.ID)
		if node != nil {
			node.updateLastHeardFrom()
		}
	}
	conn.Write([]byte(`{"status": "Received."}`))
	// c.debug("[handleClient]Got message with purpose %v", msg.Purpose)
	msg.Hop = msg.Hop + 1
	switch msg.Purpose {
	case NODE_JOIN:
		c.onNodeJoin(msg)
		break
	case NODE_ANN:
		c.onNodeAnnounce(msg)
		break
	case NODE_EXIT:
		c.onNodeExit(msg)
		break
	case HEARTBEAT:
		// c.lock.RLock()
		// defer c.lock.RUnlock()
		// for _, app := range c.applications {
		// 	app.OnHeartbeat(msg.Sender)
		// }
		break
	case STAT_DATA:
		c.onStateReceived(msg)
		break
	case STAT_REQ:
		c.onStateRequested(msg)
		break
	// case NODE_RACE:
	// 	c.onRaceCondition(msg)
	// 	break
	// case NODE_REPR:
	// 	c.onRepairRequest(msg)
	// 	break
    case BLOCK_CLUSTER:
        c.onBlockRequest(msg)
        break
    case UNBLOCK_CLUSTER:
        c.onUnBlockRequest(msg)
        break

    case LEADER_VIC:
        c.deliverLeaderElection(msg)
        break
    case YOU_ARE_LEADER:
        c.deliverLeaderElection(msg)
        break

    case WANT_TO_CRACK:
        c.deliverNewProposalfromClient(msg)
        break
    case LEADER_JUDGE:
        c.deliverNewProposalfromClient(msg)
        break
    case CRACK_DETAIL:
        c.deliverNewProposalfromClient(msg)
        break
    case INIT_BACKUP:
        c.deliverNewBackUpInitFromLeader(msg)
        break
    case FIRST_JOB:
        c.deliverFirstJobFromLeader(msg)
    case ASK_ANOTHER_PIECE:
        c.deliverAskAnotherPieceFromClient(msg)
    case GIVE_ANOTHER_PIECE:
        c.deliverRecvAnotherPieceFromLeader(msg)
    case FOUND_PASS:
        c.deliverFoundPass(msg)
    case UPDATE_BACKUP:
        c.deliverUpdateBackUpFromLeader(msg)
    case I_STOP:
        c.deliverNodeDontWantToWorkFromClient(msg)
	default:
        c.debug("DANGEROUS")
		c.onMessageReceived(msg)
	}
}

func (c *Cluster) send(msg Message, destination *Node) error {
	if destination == nil {
		return errors.New("[send]Can't send to a nil node.")
	}
	if c.self == nil {
		return errors.New("[send]Can't send from a nil node.")
	}
	address := c.GetIP(*destination)
	// c.debug("[send]Sending message %s with purpose %d to %s", msg.Key, msg.Purpose, address)
	// start := time.Now()
	err := c.SendToIP(msg, address)
	if err != nil {
	}
	return err
}

// SendToIP sends a message directly to an IP using the Wendy networking logic.
func (c *Cluster) SendToIP(msg Message, address string) error {
	// c.debug("[SendToIP]Sending message %s", string(msg.Value))
	conn, err := net.DialTimeout("tcp", address, time.Duration(c.getNetworkTimeout())*time.Second)
	if err != nil {
		c.debug(err.Error())
		return deadNodeError
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(time.Duration(c.getNetworkTimeout()) * time.Second))

	//Steps: marshalling -> encrypting -> encoding

	//1. marshalling
	forencryption, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	//2. extract key and then encrypt
	key, err := ioutil.ReadFile("key.txt")
	if err != nil {
		fmt.Print(err)
	}
	encryptedmsg, err:= encrypt(forencryption, key)
	if err != nil {
		return err
	}

	//3. encoding
	encoder := json.NewEncoder(conn)
	err = encoder.Encode(encryptedmsg)
	if err != nil {
		return err
	}
	// c.debug("[SendToIP]Sent message %s  with purpose %d to %s", msg.Key, msg.Purpose, address)
	_, err = conn.Read(nil)
	if err != nil {
		if neterr, ok := err.(net.Error); ok && neterr.Timeout() {
			return deadNodeError
		}
		if err == io.EOF {
			err = nil
		}
	}
	return err
}

// Our message handlers!

// A node wants to join the cluster. We need to route its message as we normally would, but we should also send it our state tables as appropriate.
func (c *Cluster) onNodeJoin(msg Message) {
	c.debug("\033[4;31m[onNodeJoin]Node %s joined!\033[0m", msg.Key)
	// mask := StateMask{
	// 	Mask: rT,
	// 	Rows: []int{},
	// 	Cols: []int{},
	// }
	// row := c.self.ID.CommonPrefixLen(msg.Key)
	// if msg.Hop == 1 {
	// 	// send only the matching routing table rows
	// 	for i := 0; i < row; i++ {
	// 		mask.Rows = append(mask.Rows, i)
	// 		msg.Hop++
	// 	}
	// 	// also send neighborhood set, if I'm the first node to get the message
	// 	mask.Mask = mask.Mask | nS
	// } else {
	// 	// send only the routing table rows that match the hop
	// 	if msg.Hop < row {
	// 		mask.Rows = append(mask.Rows, msg.Hop)
	// 	}
	// }
	next, err := c.Route(msg.Key)
	if err != nil {
		c.fanOutError(err)
	}
	eol := false
	if next == nil {
		// also send leaf set, if I'm the last node to get the message
		// mask.Mask = mask.Mask | lS
		eol = true
	}
    if (c.blocked == false) {
        c.onNodeJoinProcess(msg, eol)
    } else {
        //add to wait queue
        c.enWaitQueue(&msg)
    }

	// forward the message on to the next destination
	// err = c.Send(msg)
	// if err != nil {
	// 	c.fanOutError(err)
	// }
}
//process the node join request
func (c *Cluster) onNodeJoinProcess(msg Message, eol bool) {
    err := c.sendStateTables(msg.Sender, eol)
	if err != nil {
		if err != deadNodeError {
			c.fanOutError(err)
		}
	}
}

//block the new node and add this node to the waitQueue
func (c *Cluster) enWaitQueue(msg *Message) {
    c.lock.Lock()
    defer c.lock.Unlock()
    c.debug("\x1b[33;1m Node %s join wait queue\x1b[0m", msg.Sender.ID)
    c.waitQueue = append(c.waitQueue, msg)
}

//receive the block request
func (c *Cluster) onBlockRequest(msg Message) {
    c.SetBlocked()
}
//receive the unblock request
func (c *Cluster) onUnBlockRequest(msg Message) {
    c.UnBlock()
}
//set the cluster to be blocked
func (c *Cluster) SetBlocked() {
    c.debug("[SetBlocked]")
    c.lock.Lock()
    defer c.lock.Unlock()
    c.blocked = true
	fmt.Println("Coming out of lock")
}

//unblock the cluster and process the waiting queue.
func (c *Cluster) UnBlock()  {
    c.debug("[UnBlock]")
    if c.blocked == false {
        return
    }
    c.lock.Lock()
    c.blocked = false
    c.lock.Unlock()
    c.processQueue()
}

//process the wait queue after the cluster was unblocked
func (c *Cluster) processQueue() {
    var waitTime int = 2
    var waitQueuelen int = len(c.waitQueue)
    for i := 0; i < waitQueuelen; i++ {
        msg, err := c.deWaitQueue()
        if err != nil {
            c.debug("[processQueue]" + err.Error())
            return
        }

        c.debug("[processQueue]" + msg.String())
	    time.Sleep(time.Duration(1) * time.Second)
        c.onNodeJoinProcess(*msg, true)
        time.Sleep(time.Duration(waitTime + 2) * time.Second)
    }
    //start processing
}

//pop a join message from the wait queue
func (c *Cluster) deWaitQueue() (*Message, error){
	c.lock.Lock()
	defer c.lock.Unlock()
	var queueEmptyError = errors.New("[deWaitQueue] waitQueue empty")
	if len(c.waitQueue) == 0 {
		return nil, queueEmptyError
	}
	//dequeue
	joinRequest := c.waitQueue[0]
	c.waitQueue = c.waitQueue[1:]
	return joinRequest, nil
}

// A node has joined the cluster. We need to decide if it belongs in our state tables and if the Nodes in the state tables it sends us belong in our state tables. If the version of our state tables it sends to us doesn't match our local version, we need to resend our state tables to prevent a race condition.
func (c *Cluster) onNodeAnnounce(msg Message) {
	c.debug("\033[4;31m[onNodeAnnounce]Node %s announced its presence!\033[0m", msg.Key)
	// conflicts := byte(0)
	// if c.self.leafsetVersion > msg.LSVersion {
	// 	c.debug("Expected LSVersion %d, got %d", c.self.leafsetVersion, msg.LSVersion)
	// 	conflicts = conflicts | lS
	// }
	// if c.self.routingTableVersion > msg.RTVersion {
	// 	c.debug("Expected RTVersion %d, got %d", c.self.routingTableVersion, msg.RTVersion)
	// 	conflicts = conflicts | rT
	// }
	// if c.self.neighborhoodSetVersion > msg.NSVersion {
	// 	c.debug("Expected NSVersion %d, got %d", c.self.neighborhoodSetVersion, msg.NSVersion)
	// 	conflicts = conflicts | nS
	// }
	// if conflicts > 0 {
	// 	c.debug("Uh oh, %s hit a race condition. Resending state.", msg.Key)
	// 	err := c.sendRaceNotification(msg.Sender, StateMask{Mask: conflicts})
	// 	if err != nil {
	// 		c.fanOutError(err)
	// 	}
	// 	return
	// }
	// c.debug("No conflicts!")
	err := c.insertAnnounceMessage(msg)
	if err != nil {
		c.fanOutError(err)
	}
	// c.debug("[onNodeAnnounce]About to fan out join messages...")
	c.fanOutJoin(msg.Sender)
}

// node exit, remove node from neighborhoodset
func (c *Cluster) onNodeExit(msg Message) {
	c.debug("[onNodeExit]Node %s left. :(", msg.Key)
	err := c.remove(msg.Key)
	if err != nil {
		c.fanOutError(err)
		return
	}
}

func (c *Cluster) onStateReceived(msg Message) {
    // var waitTime int = 2
	err := c.insertMessage(msg)
	if err != nil {
		c.debug(err.Error())
		c.fanOutError(err)
	}
	var state stateTables
	err = json.Unmarshal(msg.Value, &state)
	if err != nil {
		c.debug(err.Error())
		c.fanOutError(err)
		return
	}

	// c.debug("[onStateReceived]State received. EOL is %v, isJoined is %v.", state.EOL, c.isJoined())
	if !c.isJoined() && state.EOL {
		// c.debug("[onStateReceived]Haven't announced presence yet... %d seconds", waitTime)
		// time.Sleep(time.Duration(waitTime) * time.Second)
		err = c.announcePresence()
		if err != nil {
			c.fanOutError(err)
		}
	} else if !state.EOL {
		c.debug("[onStateReceived]Already announced presence.")
	} else {
		c.debug("[onStateReceived]Not end of line.")
	}
}

func (c *Cluster) onStateRequested(msg Message) {
	c.debug("%s wants to know about my state tables!", msg.Sender.ID)
	var mask StateMask
	err := json.Unmarshal(msg.Value, &mask)
	if err != nil {
		c.fanOutError(err)
		return
	}
	c.sendStateTables(msg.Sender, false)
}

// func (c *Cluster) onRaceCondition(msg Message) {
// 	c.debug("Race condition. Awkward.")
// 	err := c.insertMessage(msg)
// 	if err != nil {
// 		c.fanOutError(err)
// 	}
// 	err = c.announcePresence()
// 	if err != nil {
// 		c.fanOutError(err)
// 	}
// }
//
// func (c *Cluster) onRepairRequest(msg Message) {
// 	c.debug("Helping to repair %s", msg.Sender.ID)
// 	var mask StateMask
// 	err := json.Unmarshal(msg.Value, &mask)
// 	if err != nil {
// 		c.fanOutError(err)
// 		return
// 	}
// 	c.sendStateTables(msg.Sender, mask, false)
// }

func (c *Cluster) onMessageReceived(msg Message) {
    panic("onMessageReceived")
	c.debug("[onMessageReceived]Received message %s", msg.Key)
	err := c.Send(msg)
	if err != nil {
		c.fanOutError(err)
	}
}
//export the statetable (neighborhood)
func (c *Cluster) dumpStateTables() (stateTables, error) {
	var state stateTables
    neighborhoodSet := c.Neighborhoodset.export()
    state.NeighborhoodSet = &neighborhoodSet
	return state, nil
}
//  send statetable to a node
func (c *Cluster) sendStateTables(node Node, eol bool) error {
	state, err := c.dumpStateTables()
	if err != nil {
		return err
	}

	state.EOL = eol
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	msg := c.NewMessage(STAT_DATA, c.self.ID, data)
	target, err := c.get(node.ID)
	if err != nil {
		if _, ok := err.(IdentityError); !ok && err != nodeNotFoundError {
			return err
		} else if err == nodeNotFoundError {
            c.debug("[sendStateTables]node not in my set, Sending state tables to %s", node.ID)
			return c.send(msg, &node)
		}
	}
	c.debug("[sendStateTables]Sending state tables to %s", node.ID)
	return c.send(msg, target)
}

// func (c *Cluster) sendRaceNotification(node Node, tables StateMask) error {
// 	state, err := c.dumpStateTables(tables)
// 	if err != nil {
// 		return err
// 	}
// 	data, err := json.Marshal(state)
// 	if err != nil {
// 		return err
// 	}
// 	msg := c.NewMessage(NODE_RACE, c.self.ID, data)
// 	target, err := c.get(node.ID)
// 	if err != nil {
// 		if _, ok := err.(IdentityError); !ok && err != nodeNotFoundError {
// 			return err
// 		} else if err == nodeNotFoundError {
// 			return c.send(msg, &node)
// 		}
// 	}
// 	c.debug("Sending state tables to %s to fix race condition", node.ID)
// 	return c.send(msg, target)
// }

func (c *Cluster) NumOfNodes()  int {
   return c.Neighborhoodset.nodesCount()
}

func (c *Cluster) GetListOfNodes() []*Node {
   nodes := c.Neighborhoodset.list()
   return nodes
}
func (c *Cluster) GetFirstNodeID() NodeID {
   nodeid := c.Neighborhoodset.firstNodeID()
   c.debug("[GetFirstNode]" + nodeid.String())
   return nodeid
}

func (c *Cluster) announcePresence() error {
	// c.debug("[announcePresence]Announcing presence...")
	state, err := c.dumpStateTables()
	if err != nil {
		return err
	}
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	msg := c.NewMessage(NODE_ANN, c.self.ID, data)
    nodes := c.Neighborhoodset.list()
	// Nodes := c.table.list([]int{}, []int{})
	// Nodes = append(Nodes, c.leafset.list()...)
	// Nodes = append(Nodes, c.neighborhoodset.list()...)
	sent := map[NodeID]bool{}
	for _, node := range nodes {
        if node.ID.Equals(c.self.ID) {
            continue
        }
		if node == nil {
			continue
		}
		// c.debug("[announcePresence]Saw node %s. nsVersion: %d", node.ID.String(), node.neighborhoodSetVersion)
		if _, set := sent[node.ID]; set {
			c.debug("[announcePresence]Skipping node %s, already sent announcement there.", node.ID.String())
			continue
		}
		c.debug("[announcePresence]Announcing presence to %s", node.ID)
		// c.debug("[announcePresence]Node: %s\tns: %d", node.ID.String(), node.neighborhoodSetVersion)
		// msg.LSVersion = node.leafsetVersion
		// msg.RTVersion = node.routingTableVersion
		msg.NSVersion = node.neighborhoodSetVersion
		err := c.send(msg, node)
        if err == deadNodeError {
    		// c.remove(target.ID)
            exitmsg := c.NewMessage(NODE_EXIT, node.ID, []byte{})
            snodes := c.Neighborhoodset.list()
            for _, snode := range snodes {
                c.debug("[announcePresence] inform nodes someone exit")
                err := c.send(exitmsg, snode)
                if err != nil {
                    c.fanOutError(err)
                }
                // if !(snode.ID.Equals(c.self.ID)) {
                //
                // }
            }
    	}
		sent[node.ID] = true
	}
	c.lock.Lock()
	defer c.lock.Unlock()
	c.joined = true
	return nil
}
//
// func (c *Cluster) repairLeafset(id NodeID) error {
// 	target, err := c.leafset.getNextNode(id)
// 	if err != nil {
// 		if err == nodeNotFoundError {
// 			c.warn("No node found when trying to repair the leafset. Was there a catastrophe?")
// 		} else {
// 			return err
// 		}
// 	}
// 	mask := StateMask{Mask: lS}
// 	data, err := json.Marshal(mask)
// 	if err != nil {
// 		return err
// 	}
// 	msg := c.NewMessage(NODE_REPR, id, data)
// 	return c.send(msg, target)
// }

// func (c *Cluster) repairTable(id NodeID) error {
// 	row := c.self.ID.CommonPrefixLen(id)
// 	reqRow := row
// 	col := int(id.Digit(row))
// 	targets := []*Node{}
// 	for len(targets) < 1 && row < len(c.table.Nodes) {
// 		targets = c.table.list([]int{row}, []int{})
// 		if len(targets) < 1 {
// 			row = row + 1
// 		}
// 	}
// 	mask := StateMask{Mask: rT, Rows: []int{reqRow}, Cols: []int{col}}
// 	data, err := json.Marshal(mask)
// 	if err != nil {
// 		return err
// 	}
// 	msg := c.NewMessage(NODE_REPR, c.self.ID, data)
// 	for _, target := range targets {
// 		err = c.send(msg, target)
// 		if err != nil {
// 			return err
// 		}
// 	}
// 	return nil
// }

// func (c *Cluster) repairNeighborhood() error {
// 	targets := c.neighborhoodset.list()
// 	mask := StateMask{Mask: nS}
// 	data, err := json.Marshal(mask)
// 	if err != nil {
// 		return err
// 	}
// 	msg := c.NewMessage(NODE_REPR, c.self.ID, data)
// 	for _, target := range targets {
//         //send repari message to all its neighborhood
// 		err = c.send(msg, target)
// 		if err != nil {
// 			return err
// 		}
// 	}
// 	return nil
// }


func (c *Cluster) insertMessage(msg Message) error {
	var state stateTables
	err := json.Unmarshal(msg.Value, &state)
	if err != nil {
		c.debug("Error unmarshalling JSON: %s", err.Error())
		return err
	}
	// sender := &msg.Sender
	// c.debug("Updating versions for %s. NS: %d.", sender.ID.String(),msg.NSVersion)
	// sender.updateVersions(msg.NSVersion)
	// err = c.insert(*sender)
	if err != nil {
		return err
	}
	if state.NeighborhoodSet != nil {
		for _, node := range *(state.NeighborhoodSet) {
			if node == nil {
				continue
			}
			err = c.insert(*node)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *Cluster) insertAnnounceMessage(msg Message) error {
	// var state stateTables
	// err := json.Unmarshal(msg.Value, &state)
	// if err != nil {
	// 	c.debug("Error unmarshalling JSON: %s", err.Error())
	// 	return err
	// }
	// sender := &msg.Sender
	// c.debug("Updating versions for %s. NS: %d.", sender.ID.String(),msg.NSVersion)
	// sender.updateVersions(msg.NSVersion)
	// err = c.insert(*sender)
	// if err != nil {
	// 	return err
	// }
    err := c.insert(msg.Sender)
    if err != nil {
        return err
    }
	return nil
}
//insert a node to the neighborhoodset
func (c *Cluster) insert(node Node) error {
	if node.IsZero() {
        c.debug("[insert] empty node.")
		return nil
	}
	if node.ID.Equals(c.self.ID) {
		c.debug("[insert]Skipping inserting myself.")
		return nil
	}
	// c.debug("[insert]Inserting node %s", node.ID)
    resp, err := c.Neighborhoodset.insertNode(node)
    // c.printSets()
    if err != nil && err != nsDuplicateInsertError {
        return err
    }
    if resp != nil && err != nsDuplicateInsertError {
        c.debug("[insert]Inserted node %s in neighborhood set.", resp.ID)
    }
    if err == nsDuplicateInsertError {
        c.debug("[insert] " + err.Error())
    }
	return nil
}
func (c *Cluster) printSets() {
    c.debug("[printSets] printing nodes in the sets")
    c.lock.RLock()
	defer c.lock.RUnlock()
    for _, node := range c.Neighborhoodset.Nodes {
		if node == nil {
			break
		}
		c.debug("[printSets] " + node.ID.String())
	}
}
func (c *Cluster) remove(id NodeID) error {
    c.debug("[remove] remove the node %s", id)
	node, err := c.Neighborhoodset.removeNode(id)
	if (err != nil) && (err != nodeNotFoundError) {
		return err
	}
    if node == nil {
        c.debug("[remove] removed this node before already. ")
        return nil
    } else {
        for _, app := range c.applications {
    		c.debug("[remove]sending handler node exit.")
    		app.OnNodeExit(*node)
    		c.debug("[remove]sent handler node exit.")
    	}

    }

	return nil
}
// get the node from the Neighborhoodset according to node ID
func (c *Cluster) get(id NodeID) (*Node, error) {
	node, err := c.Neighborhoodset.getNode(id)
	if err == nodeNotFoundError {
        // c.debug("[get] node no found error")
	}
	return node, err
}

func (c *Cluster) debug(format string, v ...interface{}) {
	if c.logLevel <= LogLevelDebug {
		c.log.Printf(format, v...)
	}
}

func (c *Cluster) warn(format string, v ...interface{}) {
	if c.logLevel <= LogLevelWarn {
		c.log.Printf(format, v...)
	}
}

func (c *Cluster) err(format string, v ...interface{}) {
	if c.logLevel <= LogLevelError {
		c.log.Printf(format, v...)
	}
}

//----------------------------Code snippets for encrypting and decyrpting with AES 256 GCM mode---------------------------------
//Ref: https://astaxie.gitbooks.io/build-web-application-with-golang/en/09.6.html
func encrypt(plaintext []byte, key []byte) ([]byte, error) {
	cipherBlock, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(cipherBlock)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

func decrypt(ciphertext []byte, key []byte) ([]byte, error) {
	cipherBlock, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(cipherBlock)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}
