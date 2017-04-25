package main
import (
	"os"
    "sync"
    "log"
	"./wendy-modified"
    "encoding/json"
)
const (
    NotBackup = 0  //not a back up of current leader
	BackupSuc = 1  //successor of current leader
	BackupPre = 2  //predecessor of current leader
)
type Leader struct {
    isActive           bool  //if I am the leader now, I am active. Otherwise not active
    isBackup           int  //if I am the back up node for leader
	self               *wendy.Node
    cluster            *wendy.Cluster
	chosenProposerID   wendy.NodeID   //current proposer's ID
    routineTimeStamp   int           //Time stamp to communicate with every worker
    updateTimeStamp    int          //Time stamp to update status to back up
	lock               *sync.RWMutex
    log                *log.Logger
	logLevel           int
}
func NewLeader(self *wendy.Node, cluster *wendy.Cluster) *Leader {
	return &Leader{
        isActive:           false,
        isBackup:           NotBackup,
		self:               self,
        cluster:            cluster,
        chosenProposerID:   wendy.EmptyNodeID(),
        routineTimeStamp:   -1,
        updateTimeStamp:    -1,
		lock:               new(sync.RWMutex),
        log:                log.New(os.Stdout, "LEADER ("+self.ID.String()+") ", log.LstdFlags),
		logLevel:           LogLevelDebug,
	}
}
func (le *Leader) CheckNewProposalAndRespond(msg wendy.Message) error {
    le.debug("[CheckNewProposalAndRespond]")
    if le.CheckIfAlreadyProposal(msg.Sender.ID) {
        data, err := json.Marshal(false)
        if err != nil {
            return err
        }
        msg := le.cluster.NewMessage(LEADER_JUDGE, msg.Sender.ID, data)
        err = le.cluster.Send(msg)
        if err != nil {
            return err
        }
    } else {
        data, err := json.Marshal(true)
        if err != nil {
            return err
        }
        msg := le.cluster.NewMessage(LEADER_JUDGE, msg.Sender.ID, data)
        err = le.cluster.Send(msg)
        if err != nil {
            return err
        }
    }
    return nil
}
func (le *Leader) ContackBackup()
func (le *Leader) UpdateToBackUp() {
    le.debug("[UpdateToBackUp]")
    le.increaseTimeStamp()
    //should update leader status to back up
}
func (le *Leader) increaseRoutineTimeStamp() {
    le.routineTimeStamp++
}
func (le *Leader) increaseUpdateTimeStamp() {
    le.updateTimeStamp++
}
func (le *Leader) checkUpdateTimeStamp(uts int) bool{
    if (uts == le.updateTimeStamp + 1) {
        return true
    }
    return false

}
func (le *Leader) CheckIfAlreadyProposal(id wendy.NodeID) bool{
    le.debug("[CheckIfAlreadyProposal]")
    le.lock.Lock()
    defer le.lock.Unlock()
    if le.chosenProposerID.IsEmpty() {
        le.debug("[CheckIfAlreadyProposal] No existing proposal")
        le.chosenProposerID = id
        return false
    } else {
        le.debug("[CheckIfAlreadyProposal] there is an existing proposal")
        return true
    }
}
func (le *Leader) CheckCorrectProposer(id wendy.NodeID) bool{
    if le.chosenProposerID.Equals(id) {
        return true
    }
    return false
}
func (le *Leader) SetActive() {
    le.debug("[SetActive]")
    // le.lock.Lock()
    // defer le.lock.Unlock()
    le.isActive = true
}
func (le *Leader) debug(format string, v ...interface{}) {
	if le.logLevel <= LogLevelDebug {
		le.log.Printf(format, v...)
	}
}
