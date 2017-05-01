package main
import (
	"os"
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
//election Status
const (
	NoElected = iota
	WaitLeaderVictory
	Elected
    Crashed
)
const (
    HaventPropose = iota
	WaitLeaderAgree
	ReceiveLeaderAgree
    ReceiveLeaderDeny
)
var hasProposol bool = false
// var wgElect sync.WaitGroup //wait for election end
// var wgNewJobLeaderResponse sync.WaitGroup
var aNewCrackProposal *CrackProposal
type CrackProposal struct {
    Status             int
    wgElect            *sync.WaitGroup
    wgLeaderResponse   *sync.WaitGroup
}
type LeaderComm struct {
	self               *wendy.Node
    cluster            *wendy.Cluster
	// lastHeardFrom      time.Time
	Status             int
    currentLeader      wendy.NodeID
    ifLeader           bool  //if I am the leader
    PendingProcessQueue    []string
    // Leader             *Leader
	lock               *sync.RWMutex
    log                *log.Logger
	logLevel           int
}
func NewCrackProposal() *CrackProposal {
    return &CrackProposal{
        Status:            HaventPropose,
        wgElect:           new(sync.WaitGroup),
        wgLeaderResponse:  new(sync.WaitGroup),
	}
}
func NewLeaderComm(self *wendy.Node, cluster *wendy.Cluster) *LeaderComm {
	return &LeaderComm{
		self:               self,
        cluster:            cluster,
        // lastHeardFrom:      time.Time{},
        Status:             NoElected,
        currentLeader:      wendy.EmptyNodeID(),
        ifLeader:           false,
        PendingProcessQueue:[]string{},
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
    hasProposol = true
    aNewCrackProposal = NewCrackProposal()
    if l.Status == NoElected {
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
    aNewCrackProposal.Status = WaitLeaderAgree
    aNewCrackProposal.wgLeaderResponse.Add(1)
    msg := l.cluster.NewMessage(WANT_TO_CRACK, l.currentLeader, nil)
    l.cluster.Send(msg)
    aNewCrackProposal.wgLeaderResponse.Wait()
    if aNewCrackProposal.Status == ReceiveLeaderAgree {
        return true
    } else if aNewCrackProposal.Status == ReceiveLeaderDeny {
        return false
    } else {
        l.debug("DANGEROUS")
        return false
    }
}
func (l *LeaderComm) SendCrackJobDetailsToLeader(hashtype string, hash string, pwdlength int) {
    l.debug("[SendCrackJobDetailsToLeader] ")
    payload := CrackJobDetailsMessage{HashType: hashtype, Hash: hash, Pwdlength:pwdlength}
    data, err := bson.Marshal(payload)
    if err != nil {
        panic(err)
    }
    msg := l.cluster.NewMessage(CRACK_DETAIL, l.currentLeader, data)
    err = l.cluster.Send(msg)
    if err != nil {
        l.debug("[NotifyLeaderIStop] add to PendingProcessQueue")
        l.PendingProcessQueue = append(l.PendingProcessQueue, "I_STOP")
    }
}
func (l *LeaderComm) ReceiveLeaderJudgement(ifAgree bool) {
    l.debug("[ReceiveLeaderJudgement] ")
    if ifAgree {
        aNewCrackProposal.Status = ReceiveLeaderAgree
    } else {
        aNewCrackProposal.Status = ReceiveLeaderDeny
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
    if (hasProposol) {
        aNewCrackProposal.wgElect.Done()
    }

}
func (l *LeaderComm) setNewLeader(lid wendy.NodeID) {
    l.debug("[setNewLeader]")
    l.lock.Lock()
    defer l.lock.Unlock()
    l.Status = Elected
    l.currentLeader = lid
    // l.lastHeardFrom = time.Now()
}
//notify the first node there is an election and you are the leader
func (l *LeaderComm) notifyLeaderElection(lid wendy.NodeID) {
    l.debug("[notifyLeaderElection]")
    l.Status = WaitLeaderVictory
    msg := l.cluster.NewMessage(YOU_ARE_LEADER, lid, nil)
    l.cluster.Send(msg)
}
func (l *LeaderComm) BecomeLeader() {
    l.debug("[BecomeLeader]")
    l.Status = Elected
    l.currentLeader = l.self.ID
    l.ifLeader = true
    l.broadcastVictory()
}
//Leader use this function to broadcast victory of election
func (l *LeaderComm) broadcastVictory() {
    l.debug("broadcastVictory")
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
func (l *LeaderComm) NotifyLeaderIStop() {
    msg := l.cluster.NewMessage(I_STOP, l.currentLeader, []byte{})
    err := l.cluster.Send(msg)
    if err != nil {
        l.debug("[NotifyLeaderIStop] add to PendingProcessQueue")
        l.PendingProcessQueue = append(l.PendingProcessQueue, "I_STOP")
    }
}
func (l *LeaderComm) GoAskLeaderANewPiece(Seqn int) {
    // l.debug("[AskLeaderANewPiece]")
    l.debug("[AskLeaderANewPiece] leader is " + l.currentLeader.String())
    payload := AskAnotherMessage{SeqNum: Seqn}
    data, err := bson.Marshal(payload)
    if err != nil {
        panic(err)
    }
    msg := l.cluster.NewMessage(ASK_ANOTHER_PIECE, l.currentLeader, data)
    err = l.cluster.Send(msg)
    if err != nil {
        l.debug("[AskLeaderANewPiece] leader fail, add to PendingProcessQueue")
        l.PendingProcessQueue = append(l.PendingProcessQueue, "ASK_ANOTHER_PIECE")
    }
}
func (l *LeaderComm) ProcessPendingProcessQueue() {

}
func (l *LeaderComm) Test(format string, v ...interface{}) {
	l.log.Printf(format, v...)
}
func (l *LeaderComm) debug(format string, v ...interface{}) {
	if l.logLevel <= LogLevelDebug {
		l.log.Printf(format, v...)
	}
}
