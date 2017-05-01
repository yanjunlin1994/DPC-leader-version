package wendy

import (
	"errors"
	"log"
	"os"
	"strings"
	"sync"
    "strconv"
)

type NeighborhoodSet struct {
	self     *Node
	Nodes    []*Node
	log      *log.Logger
	logLevel int
	lock     *sync.RWMutex
}

func newNeighborhoodSet(self *Node) *NeighborhoodSet {
	return &NeighborhoodSet{
		self:     self,
		Nodes:    []*Node{self},
		log:      log.New(os.Stdout, "wendy#NeighborhoodSet("+self.ID.String()+")", log.LstdFlags),
		logLevel: LogLevelDebug,
		lock:     new(sync.RWMutex),
	}
}

var nsDuplicateInsertError = errors.New("Node already exists in neighborhood set.")

//route function for neighborhood
func (n *NeighborhoodSet) route(key NodeID) (*Node, error) {
	n.lock.RLock()
	defer n.lock.RUnlock()
	for _, node := range n.Nodes {
		if node == nil {
			break
		}
		if key.Equals(node.ID) {
			return node, nil
		}
	}
	return nil, nodeNotFoundError
}

func (n *NeighborhoodSet) insertNode(node Node) (*Node, error) {
	return n.insertValues(node.ID, node.LocalIP, node.GlobalIP, node.Region, node.Port, node.neighborhoodSetVersion)
}

func (n *NeighborhoodSet) insertValues(id NodeID, localIP, globalIP, region string, port int, nSVersion uint64) (*Node, error) {
	n.lock.Lock()
	defer n.lock.Unlock()
	if id.Equals(n.self.ID) {
		return nil, throwIdentityError("insert", "into", "neighborhood set")
	}
	insertNode := NewNode(id, localIP, globalIP, region, port)
	insertNode.updateVersions(nSVersion)
	// insertNode.setProximity(proximity)
	// newNS := [32]*Node{}
	// newNSpos := 0
	// score := n.self.Proximity(insertNode)
	dup := false
	for _, node := range n.Nodes {
		if node != nil && insertNode.ID.Equals(node.ID) {
			dup = true
			break
		}
		//      if (newNSpos <= 31) && (node == nil) {
		//          n.debug("[insertValues]Inserting node %s in neighborhood set.", insertNode.ID)
		//          n.Nodes[newNSpos] = insertNode
		// inserted = true
		// break
		//      }
		//      newNSpos++
	}
	if !dup {
		newSpot := -1
		for i, node := range n.Nodes {
			if strings.Compare(id.String(), node.ID.String()) == -1 {
				newSpot = i
				break
			}
		}

		if newSpot == -1 { // Add to end of list
			n.Nodes = append(n.Nodes, insertNode)
		} else { // Else insert into correct spot calculated above
			n.Nodes = append(n.Nodes[:newSpot], append([]*Node{insertNode}, n.Nodes[newSpot:]...)...)
		}
	}


	// if newNSpos > 31 {
	// 	break
	// }
	// if node == nil && !inserted && !dup {
	// 	n.Nodes[newNSpos] = insertNode
	// 	newNSpos++
	// 	inserted = true
	// 	break
	// }
	// if node != nil && insertNode.ID.Equals(node.ID) {
	// 	// insertNode.updateVersions(node.routingTableVersion, node.leafsetVersion, node.neighborhoodSetVersion)
	// 	// newNS[newNSpos] = insertNode
	// 	// newNSpos++
	// 	dup = true
	// 	break
	// }
	// if node != nil && n.self.Proximity(node) > score && !inserted && !dup {
	// 	newNS[newNSpos] = insertNode
	// 	newNSpos++
	// 	inserted = true
	// 	continue
	// }
	// if newNSpos <= 31 {
	// 	newNS[newNSpos] = node
	// 	newNSpos++
	// }

	// n.Nodes = newNS
	if dup {
		return nil, nsDuplicateInsertError
	} else {
		n.self.incrementNSVersion()
		return insertNode, nil
	}
	return nil, nil
}

func (n *NeighborhoodSet) getNode(id NodeID) (*Node, error) {
	n.lock.RLock()
	defer n.lock.RUnlock()
	if id.Equals(n.self.ID) {
		return nil, throwIdentityError("get", "from", "neighborhood set")
	}
	for _, node := range n.Nodes {
		if node == nil {
			break
		}
		if id.Equals(node.ID) {
			return node, nil
		}
	}
	return nil, nodeNotFoundError
}

//export all the Nodes in neighborhood
func (n *NeighborhoodSet) export() []*Node {
	n.lock.RLock()
	defer n.lock.RUnlock()
	return n.Nodes
}

func (n *NeighborhoodSet) nodesCount() int {
	n.lock.RLock()
	defer n.lock.RUnlock()
	return len(n.Nodes)
}
//hearbeat receives are my predecessor and successor
func (n *NeighborhoodSet) HeartbeatReceivers() []*Node{
    // n.debug("[heartbeatReceivers]Node numbers: " + strconv.Itoa(len(n.Nodes)))
    n.lock.RLock()
	defer n.lock.RUnlock()
    if len(n.Nodes) < 2 { //only contains myself or contains none
		return nil
	}
    receivers := []*Node{}
    pos := -1
	// var nodeMe *Node
    for index, entry := range n.Nodes {
		if entry == nil || entry.ID.Equals(n.self.ID) {
			pos = index
			// nodeMe = entry
			break
		}
	}
	if pos == -1 || pos > len(n.Nodes) { //abormal
		return nil
	}
    if len(n.Nodes) == 2 {
        receivers = append(receivers, n.Nodes[(pos + 1) % 2])//wrap around
        n.debug("[heartbeatReceivers]My pos in the set: " + strconv.Itoa(pos) + " | send to: " + strconv.Itoa((pos + 1) % 2))
	} else if pos == 0 {
        receivers = append(receivers, n.Nodes[len(n.Nodes) - 1], n.Nodes[1])
        n.debug("[heartbeatReceivers]My pos in the set: " + strconv.Itoa(pos) + " | send to: " + strconv.Itoa(len(n.Nodes) - 1) + " and " + strconv.Itoa(1))
    } else if pos == (len(n.Nodes) - 1) {
        receivers = append(receivers, n.Nodes[len(n.Nodes) - 2], n.Nodes[0])
        n.debug("[heartbeatReceivers]My pos in the set: " + strconv.Itoa(pos) + " | send to: " + strconv.Itoa(len(n.Nodes) - 2) + " and " + strconv.Itoa(0))
    } else {
        receivers = append(receivers, n.Nodes[pos - 1], n.Nodes[pos + 1])
        n.debug("[heartbeatReceivers]My pos in the set: " + strconv.Itoa(pos) + " | send to: " + strconv.Itoa(pos - 1) + " and " + strconv.Itoa(pos + 1))
    }
    return receivers
}

func (n *NeighborhoodSet) list() []*Node {
	n.lock.RLock()
	defer n.lock.RUnlock()
	nodes := []*Node{}
	for _, node := range n.Nodes {
		if node != nil {
			nodes = append(nodes, node)
		}
	}
	return nodes
}
//get the first node ID
func (n *NeighborhoodSet) firstNodeID() NodeID {
	n.lock.RLock()
	defer n.lock.RUnlock()
    firstNode := n.Nodes[0]
	return firstNode.ID
}

func (n *NeighborhoodSet) removeNode(id NodeID) (*Node, error) {
	n.lock.Lock()
	defer n.lock.Unlock()
	if id.Equals(n.self.ID) {
		return nil, throwIdentityError("remove", "from", "neighborhood set")
	}
	pos := -1
	var node *Node
	for index, entry := range n.Nodes {
		if entry == nil || entry.ID.Equals(id) {
			pos = index
			node = entry
			break
		}
	}
	if pos == -1 || pos > len(n.Nodes) {
		return nil, nodeNotFoundError
	}
	var slice []*Node
	if len(n.Nodes) == 1 {
		slice = []*Node{}
	} else if pos+1 == len(n.Nodes) {
		slice = n.Nodes[:pos]
	} else if pos == 0 {
		slice = n.Nodes[1:]
	} else {
		slice = append(n.Nodes[:pos], n.Nodes[pos+1:]...)
	}
    n.debug("[removeNode] number: " + strconv.Itoa(len(slice)));
    n.Nodes = n.Nodes[:len(slice)]
    for i, _ := range n.Nodes {
        n.Nodes[i] = slice[i]
	}
	n.self.incrementNSVersion()
	return node, nil
}

func (n *NeighborhoodSet) debug(format string, v ...interface{}) {
	if n.logLevel <= LogLevelDebug {
		n.log.Printf(format, v...)
	}
}

func (n *NeighborhoodSet) warn(format string, v ...interface{}) {
	if n.logLevel <= LogLevelWarn {
		n.log.Printf(format, v...)
	}
}

func (n *NeighborhoodSet) err(format string, v ...interface{}) {
	if n.logLevel <= LogLevelError {
		n.log.Printf(format, v...)
	}
}
