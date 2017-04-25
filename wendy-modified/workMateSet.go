package wendy
//
// import (
// 	"errors"
// 	"log"
// 	"os"
// 	"strings"
// 	"sync"
// )
//
// type WorkMateSet struct {
// 	self     *Node
// 	Nodes    []*Node
// 	log      *log.Logger
// 	logLevel int
// 	lock     *sync.RWMutex
// }
//
// func newWorkMateSetSet() *WorkMateSet {
// 	return &WorkMateSet{
// 		self:     self,
// 		Nodes:    []*Node{},
// 		log:      log.New(os.Stdout, "wendy#WorkMateSet("+self.ID.String()+")", log.LstdFlags),
// 		logLevel: LogLevelWarn,
// 		lock:     new(sync.RWMutex),
// 	}
// }
//
// var nsDuplicateInsertError = errors.New("Node already exists in WorkMate set.")
//
// func (w *WorkMateSetSet) route(key NodeID) (*Node, error) {
// 	w.lock.RLock()
// 	defer w.lock.RUnlock()
// 	for _, node := range n.Nodes {
// 		if node == nil {
// 			break
// 		}
// 		if key.Equals(node.ID) {
// 			return node, nil
// 		}
// 	}
// 	return nil, nodeNotFoundError
// }
//
// func (w *WorkMateSetSet) insertNode(node Node) (*Node, error) {
// 	return w.insertValues(node.ID, node.LocalIP, node.GlobalIP, node.Region, node.Port)
// }
//
// func (w *WorkMateSetSet) dumpNodes(nodes []*Node) (error) {
// 	w.lock.Lock()
// 	defer w.lock.Unlock()
//     w.Nodes := []*Node{}
// 	for _, node := range nodes {
// 		w.Nodes = append(w.Nodes, node)
//     }
// }
//
// func (w *WorkMateSetSet) getNode(id NodeID) (*Node, error) {
// 	w.lock.RLock()
// 	defer w.lock.RUnlock()
// 	if id.Equals(w.self.ID) {
// 		return nil, throwIdentityError("get", "from", "neighborhood set")
// 	}
// 	for _, node := range w.Nodes {
// 		if node == nil {
// 			break
// 		}
// 		if id.Equals(node.ID) {
// 			return node, nil
// 		}
// 	}
// 	return nil, nodeNotFoundError
// }
//
// //export all the Nodes in neighborhood
// func (w *WorkMateSetSet) export() []*Node {
// 	w.lock.RLock()
// 	defer w.lock.RUnlock()
// 	return w.Nodes
// }
//
// func (w *WorkMateSetSet) nodesCount() int {
// 	w.lock.RLock()
// 	defer w.lock.RUnlock()
// 	return len(w.Nodes)
// }
//
// func (w *WorkMateSetSet) list() []*Node {
// 	w.lock.RLock()
// 	defer w.lock.RUnlock()
// 	nodes := []*Node{}
// 	for _, node := range n.Nodes {
// 		if node != nil {
// 			nodes = append(nodes, node)
// 		}
// 	}
// 	return nodes
// }
//
// func (n *NeighborhoodSet) removeNode(id NodeID) (*Node, error) {
// 	n.lock.Lock()
// 	defer n.lock.Unlock()
// 	if id.Equals(n.self.ID) {
// 		return nil, throwIdentityError("remove", "from", "neighborhood set")
// 	}
// 	pos := -1
// 	var node *Node
// 	for index, entry := range n.Nodes {
// 		if entry == nil || entry.ID.Equals(id) {
// 			pos = index
// 			node = entry
// 			break
// 		}
// 	}
// 	if pos == -1 || pos > len(n.Nodes) {
// 		return nil, nodeNotFoundError
// 	}
// 	var slice []*Node
// 	if len(n.Nodes) == 1 {
// 		slice = []*Node{}
// 	} else if pos+1 == len(n.Nodes) {
// 		slice = n.Nodes[:pos]
// 	} else if pos == 0 {
// 		slice = n.Nodes[1:]
// 	} else {
// 		slice = append(n.Nodes[:pos], n.Nodes[pos+1:]...)
// 	}
// 	for i, _ := range n.Nodes {
// 		if i < len(slice) {
// 			n.Nodes[i] = slice[i]
// 		} else {
// 			n.Nodes[i] = nil
// 		}
// 	}
// 	n.self.incrementNSVersion()
// 	return node, nil
// }
//
// func (n *NeighborhoodSet) debug(format string, v ...interface{}) {
// 	if n.logLevel <= LogLevelDebug {
// 		n.log.Printf(format, v...)
// 	}
// }
//
// func (n *NeighborhoodSet) warn(format string, v ...interface{}) {
// 	if n.logLevel <= LogLevelWarn {
// 		n.log.Printf(format, v...)
// 	}
// }
//
// func (n *NeighborhoodSet) err(format string, v ...interface{}) {
// 	if n.logLevel <= LogLevelError {
// 		n.log.Printf(format, v...)
// 	}
// }
