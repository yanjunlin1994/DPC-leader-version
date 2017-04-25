package main
import (
	"os"
	"time"
    "sync"
    "log"
	"./wendy-modified"
    "gopkg.in/mgo.v2/bson"
)
const (
	LogLevelDebug = iota
	LogLevelWarn
	LogLevelError
)
//election status
const (
	NoElected = iota
	WaitLeaderVictory
	Elected
)
const (
    HaventPropose = iota
	WaitLeaderAgree
	ReceiveLeaderAgree
)
const (
    Notknow = iota
	Deny
	Agree
)
// var wgElect sync.WaitGroup //wait for election end
// var wgNewJobLeaderResponse sync.WaitGroup
var aNewCrackProposal *CrackProposal
type CrackProposal struct {
    status             int
    ifAgree            int
    wgElect            *sync.WaitGroup
    wgLeaderResponse   *sync.WaitGroup
    lock               *sync.RWMutex
}
type LeaderComm struct {
	self               *wendy.Node
    cluster            *wendy.Cluster
	lastHeardFrom      time.Time
	connected          bool  //if I am connected to the leader
    electionStatus     int
    currentLeader      wendy.NodeID
    ifLeader           bool  //if I am the leader
    hasProposal        bool
    // Leader             *Leader
	lock               *sync.RWMutex
    log                *log.Logger
	logLevel           int
}
func NewCrackProposal() *CrackProposal {
    return &CrackProposal{
        status:            HaventPropose,
        ifAgree:           Notknow,
        wgElect:           new(sync.WaitGroup),
        wgLeaderResponse:  new(sync.WaitGroup),
		lock:              new(sync.RWMutex),
	}
}
func NewLeaderComm(self *wendy.Node, cluster *wendy.Cluster) *LeaderComm {
	return &LeaderComm{
		self:               self,
        cluster:            cluster,
        lastHeardFrom:      time.Time{},
        connected:          false,
        electionStatus:     NoElected,
        currentLeader:      wendy.EmptyNodeID(),
        ifLeader:           false,
        hasProposal:        false,
        // Leader:             nil,
		lock:               new(sync.RWMutex),
        log:                log.New(os.Stdout, "LEADERCOMM ("+self.ID.String()+") ", log.LstdFlags),
		logLevel:           LogLevelDebug,
        // wgElect:               new(sync.WaitGroup),
	}
}
//propose a new cracking job
func (l *LeaderComm) ProposeNewJob() bool{
    l.debug("[ProposeNewJob]")
    aNewCrackProposal = NewCrackProposal()
    l.hasProposal = true
    if l.electionStatus != Elected {
        aNewCrackProposal.wgElect.Add(1)
        l.proposeElection()
        aNewCrackProposal.wgElect.Wait()
        l.debug("[ProposeNewJob] Finally finish election ")
    }
    return l.tellLeaderIwantToCrack()
}
//tell leader I want to crack a password
func (l *LeaderComm) tellLeaderIwantToCrack() bool {
    l.debug("[tellLeaderIwantToCrack] ")
    aNewCrackProposal.status = WaitLeaderAgree
    aNewCrackProposal.wgLeaderResponse.Add(1)
    msg := l.cluster.NewMessage(WANT_TO_CRACK, l.currentLeader, nil)
    l.cluster.Send(msg)
    aNewCrackProposal.wgLeaderResponse.Wait()
    if aNewCrackProposal.ifAgree == Agree {
        return true
    } else {
        return false
    }
}
func (l *LeaderComm) SendCrackJobDetailsToLeader(hashtype string, hash string, pwdlength int) {
    payload := CrackJobDetailsMessage{HashType: hashtype, Hash: hash, Pwdlength:pwdlength}
    data, err := bson.Marshal(payload)
    if err != nil {
        panic(err)
    }
    msg := l.cluster.NewMessage(CRACK_DETAIL, l.currentLeader, data)
    l.cluster.Send(msg)
}
func (l *LeaderComm) ReceiveLeaderJudgement(ifAgree bool) {
    l.debug("[ReceiveLeaderJudgement] ")
    if ifAgree {
        aNewCrackProposal.ifAgree = Agree
    } else {
        aNewCrackProposal.ifAgree = Deny
    }
    aNewCrackProposal.wgLeaderResponse.Done()
}
//propose an election and return the leader ID
func (l *LeaderComm) proposeElection() {
    l.debug("[proposeElection]")
    LeaderID := cluster.GetFirstNodeID()
    if (l.self.ID.Equals(LeaderID)) {
        l.debug("[proposeElection] I am the leader! ")
        aNewCrackProposal.wgElect.Done()
        l.BecomeLeader()
    } else {
        l.debug("[proposeElection] I am not the leader ")
        l.notifyLeaderElection(LeaderID)
    }
}
func (l *LeaderComm) ReceiveVictoryFromLeader(lid wendy.NodeID) {
    l.debug("[ReceiveVictoryFromLeader]")
    l.setNewLeader(lid)
    if (l.hasProposal) {
        aNewCrackProposal.wgElect.Done()
    }

}
func (l *LeaderComm) setNewLeader(lid wendy.NodeID) {
    l.debug("[setNewLeader]")
    l.lock.Lock()
    defer l.lock.Unlock()
    l.connected = true
    l.electionStatus = Elected
    l.currentLeader = lid
    l.lastHeardFrom = time.Now()
}
//notify the first node there is an election and you are the leader
func (l *LeaderComm) notifyLeaderElection(lid wendy.NodeID) {
    l.debug("[notifyLeaderElection]")
    l.electionStatus = WaitLeaderVictory
    msg := l.cluster.NewMessage(YOU_ARE_LEADER, lid, nil)
    l.cluster.Send(msg)
}
func (l *LeaderComm) BecomeLeader() {
    l.debug("[BecomeLeader]")
    l.electionStatus = Elected
    l.currentLeader = l.self.ID
    l.ifLeader = true
    l.broadcastVictory()
}
//Leader use this function to broadcast victory of election
func (l *LeaderComm) broadcastVictory() {
    l.broadCastMessage(LEADER_VIC, nil)
}
//broadcast messasge to all other nodes
func (l *LeaderComm) broadCastMessage(msgType byte, data []byte) {
    nodes := l.cluster.GetListOfNodes()
    if (len(nodes) < 2) {
        return
    }
    for _, nodeIterate := range nodes {
        if !nodeIterate.ID.Equals(l.self.ID) {
            msg := l.cluster.NewMessage(msgType, nodeIterate.ID, data)
            l.cluster.Send(msg)
        }
    }
}
func (l *LeaderComm) Test(format string, v ...interface{}) {
	l.log.Printf(format, v...)
}
func (l *LeaderComm) debug(format string, v ...interface{}) {
	if l.logLevel <= LogLevelDebug {
		l.log.Printf(format, v...)
	}
}
