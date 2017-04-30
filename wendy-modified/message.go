package wendy

// Message represents the messages that are sent through the cluster of Nodes
type Message struct {
	Purpose     byte
	Sender      Node   // The Node a message originated at
	Key         NodeID // The message's ID
	Value       []byte // The message being passed
	Credentials []byte // The Credentials used to authenticate the Message
	// LSVersion   uint64 // The version of the leaf set, for join messages
	// RTVersion   uint64 // The version of the routing table, for join messages
	NSVersion uint64 // The version of the neighborhood set, for join messages
	Hop       int    // The number of hops the message has taken
}

const (
	NODE_JOIN = byte(iota) // Used when a Node wishes to join the cluster
	NODE_EXIT              // Used when a Node leaves the cluster
	HEARTBEAT              // Used when a Node is being tested
	STAT_DATA              // Used when a Node broadcasts state info
	STAT_REQ               // Used when a Node is requesting state info
	NODE_RACE              // Used when a Node hits a race condition
	NODE_REPR              // Used when a Node needs to repair its LeafSet
	NODE_ANN               // Used when a Node broadcasts its presence
)

const (
	NEW_JOB = 20         //Used when sending/receiving a new hash cracking job
	STOP_JOB = 21
	FOUND_PASS = 22
)
const (
    BLOCK_CLUSTER = 30
	UNBLOCK_CLUSTER = 31
)
const (
    LEADER_VIC = 60
    YOU_ARE_LEADER = 61
    WANT_TO_CRACK = 62
    LEADER_JUDGE = 63
    CRACK_DETAIL = 64
)
const (
    FIRST_JOB = 70
    ASK_ANOTHER_PIECE = 71
    I_STOP = 72
)
const (
    INIT_BACKUP = 90
)
// String returns a string representation of a message.
func (m *Message) String() string {
	return m.Key.String() + ": " + string(m.Value)
}

func (c *Cluster) NewMessage(purpose byte, key NodeID, value []byte) Message {
	var credentials []byte
	if c.credentials != nil {
		credentials = c.credentials.Marshal()
	}

	return Message{
		Purpose:     purpose,
		Sender:      *c.self,
		Key:         key,
		Value:       value,
		Credentials: credentials,
		// LSVersion:   c.self.leafsetVersion,
		// RTVersion:   c.self.routingTableVersion,
		NSVersion: c.self.neighborhoodSetVersion,
		Hop:       0,
	}
}
