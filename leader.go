package main
import (
	"os"
    "sync"
    "log"
	"./wendy-modified"
    "encoding/json"
    "runtime"
    "gopkg.in/mgo.v2/bson"
    "strings"
    "strconv"
    "os/exec"
)
const (
    SPLIT_FOLD = 15
)
type WholeJob struct {
    HashType           int          `json:"hashtp,omitempty"`
    HashValue          string       `json:"hashvl,omitempty"`
    Pwdlength          int          `json:"pwdl,omitempty"`
}
type JobEntry struct {
    SeqNum          int     `json:"seqn,omitempty"`
    Start           int    `json:"st,omitempty"`
    End             int     `json:"ed,omitempty"`
    Node            string  `json:"nd,omitempty"`
    Status          int     `json:"stt,omitempty"`
}

type Leader struct {
    isActive           bool  //if I am the leader now, I am active. Otherwise not active
    isBackup           bool  //if I am the back up node for leader
    IsWorking          bool
    IsDone             bool
    BackUps            wendy.NodeID
	self               *wendy.Node
    cluster            *wendy.Cluster
	chosenProposerID   wendy.NodeID   //current proposer's ID
    routineTimeStamp   int           //Time stamp to communicate with every worker
    updateTimeStamp    int          //Time stamp to update status to back up
    JobMap             []JobEntry
    wholeJob           *WholeJob
	lock               *sync.RWMutex
    log                *log.Logger
	logLevel           int
}
func NewWholeJob(HashType int, HashValue string, Pwdlength int) *WholeJob {
	return &WholeJob{
        HashType:           HashType,
        HashValue:          HashValue,
        Pwdlength:          Pwdlength,
	}
}
func NewJobEntry(seqNum int, start int, end int, node string, status int) *JobEntry {
	return &JobEntry{
        SeqNum:          seqNum,
        Start:           start,
        End:             end,
        Node:            node,
        Status:          status,
	}
}
func NewLeader(self *wendy.Node, cluster *wendy.Cluster) *Leader {
	return &Leader{
        isActive:           false,
        isBackup:           false,
        IsWorking:          false,
        IsDone:             false,
        BackUps:            wendy.EmptyNodeID(),
		self:               self,
        cluster:            cluster,
        chosenProposerID:   wendy.EmptyNodeID(),
        routineTimeStamp:   -1,
        updateTimeStamp:    -1,
        JobMap:             []JobEntry{},
        wholeJob:           nil,
		lock:               new(sync.RWMutex),
        log:                log.New(os.Stdout, "LEADER ("+self.ID.String()+") ", log.LstdFlags),
		logLevel:           LogLevelDebug,
	}
}
func (le *Leader) LeaderReset() {
    le.isActive = false
    le.isBackup = false
    le.IsWorking = false
    le.IsDone = false
    le.BackUps = wendy.EmptyNodeID()
    le.chosenProposerID = wendy.EmptyNodeID()
    le.routineTimeStamp = -1
    le.updateTimeStamp = -1
    le.JobMap = []JobEntry{}
    le.wholeJob = nil
}
func (le *Leader) CheckNewProposalAndRespond(msg wendy.Message) error {
    le.debug("[CheckNewProposalAndRespond]")
    if le.checkIfAlreadyProposal(msg.Sender.ID) {
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
func (le *Leader) checkIfAlreadyProposal(id wendy.NodeID) bool{
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
func (le *Leader) PreapreJob(HashType string, Hash string, Pwdlength int) error {
    keySpace, nodesCount := le.calculateKeySpace(HashType, Hash, Pwdlength)
    le.constructJobMap(keySpace, nodesCount)
    le.ContactBackup()
    le.giveFirstJob()
    return nil
}
func (le *Leader) calculateKeySpace(HashType string, Hash string, Pwdlength int) (int, int) {
	var ht int
	var mask string
	var keySpace int

	if (strings.EqualFold(HashType, "MD5")) == true {
		ht = 0
	} else if (strings.EqualFold(HashType, "SHA1")) == true {
		ht = 100
	} else if (strings.EqualFold(HashType, "SHA256")) == true {
		ht = 1400
	}
    le.lock.Lock()
    le.wholeJob = NewWholeJob(ht, Hash, Pwdlength)
    le.IsWorking = true
    le.lock.Unlock()

	for i := 0; i < Pwdlength; i++ {
		mask = mask + "?l"
	}
	keySpace = le.getTotoalKeySpace(mask)
    if (keySpace == -1) {
        panic("keyspace calculation error")
    }
	nodesCount := le.cluster.NumOfNodes()
	le.debug(strconv.Itoa(nodesCount) + " nodes and total key space is " + strconv.Itoa(keySpace))
    return keySpace, nodesCount
}
func (le *Leader) constructJobMap(keySpace int, nodesCount int) error {
	limit := keySpace / (SPLIT_FOLD * nodesCount)
    pos := 0
    seq := 0
    for {
        if (pos + limit >  keySpace) {
            aJobEntry := NewJobEntry(seq, pos, keySpace, "none", -1)
            le.JobMap = append(le.JobMap, *aJobEntry)
            break
        }
        aJobEntry := NewJobEntry(seq, pos, pos + limit, "none", -1)
        le.JobMap = append(le.JobMap, *aJobEntry)

        pos = pos + limit
        seq++
    }
    le.printJobMap()
    return nil
}
//just for debug
func (le *Leader) printJobMap() {
    le.lock.RLock()
    defer le.lock.RUnlock()
    le.debug(">>>>>>>>>>>>>>>")
    for _, job := range le.JobMap {

        le.debug(strconv.Itoa(job.SeqNum) + " " + strconv.Itoa(job.Start) + " " +
                    strconv.Itoa(job.End) + " " + job.Node + " " + strconv.Itoa(job.Status))
	}
    le.debug("<<<<<<<<<<<<<<<")
}
func (le *Leader) giveFirstJob() {
    nodes := le.cluster.GetListOfNodes()
    if (len(nodes) < 2) {
        return
    }
    i := 0
    for _, nodeIterate := range nodes {
        jobentry := le.JobMap[i]
        payload := NewJobMessage{SeqNum: jobentry.SeqNum,
                                 HashType: le.wholeJob.HashType,
                                 HashValue: le.wholeJob.HashValue,
                                 Pwdlength: le.wholeJob.Pwdlength,
                                 Start: jobentry.Start,
                                 End:   jobentry.End}
        data, err := bson.Marshal(payload)
        msg := le.cluster.NewMessage(FIRST_JOB, nodeIterate.ID, data)
        err = le.cluster.Send(msg)
        if (err != nil) {
            le.debug("[giveFirstJob] This node died couldn't send ")
            continue
        } else {
            le.JobMap[i].Node = nodeIterate.ID.String()
            le.JobMap[i].Status = 0
            le.debug("[giveFirstJob] send to " + le.JobMap[i].Node)
            le.updateToBackup(i, le.JobMap[i].Node, 0)
            i++
        }
        // if !nodeIterate.ID.Equals(le.self.ID) {
        //
        //
        // }
    }

}
func (le *Leader) HandleNewNode(nodeid wendy.NodeID) {
    le.debug("[HandleNewNode]")
    if (le.IsDone) {
        return
    }
    msg := le.cluster.NewMessage(LEADER_VIC, nodeid, []byte{})
    le.cluster.Send(msg)
    aJobEntry := le.findAnUndoneJob()
    if (aJobEntry == nil) {
        return
    }
    jobIndex := aJobEntry.SeqNum
    le.SendAnotherPieceToClient(jobIndex, nodeid)
}
func (le *Leader) HandleNodeLeft(nodeid wendy.NodeID) {
    if (le.IsDone) {
        return
    }
    le.searchThisNodeUndoneWorkAndMarkUndone(nodeid.String())
}
func (le *Leader) searchThisNodeUndoneWorkAndMarkUndone(nodeid string) {
    if (le.IsDone) {
        return
    }
    le.debug("[searchThisNodeUndoneWorkAndMarkUndone]")
    le.lock.RLock()
    var jobIndex int = -1
    for i := 0; i < len(le.JobMap); i++ {
        if (le.JobMap[i].Node == nodeid) && (le.JobMap[i].Status == 0) {
            jobIndex = i
            break
        }
    }
    le.lock.RUnlock()
    if (jobIndex != -1) {
        le.debug("[searchThisNodeUndoneWorkAndMarkUndone] reset the entry")
        le.lock.Lock()
        le.debug("[searchThisNodeUndoneWorkAndMarkUndone] reset the entry lock acquire")
        le.JobMap[jobIndex].Node = "none"
        le.JobMap[jobIndex].Status = -1
        le.lock.Unlock()
    }
}
func (le *Leader) ReceiveRequestForAnotherPiece(nodeid wendy.NodeID, seq int) {
    if (le.IsDone) {
        return
    }
    le.debug("[ReceiveRequestForAnotherPiece]")
    le.markNodeLastJobDone(nodeid.String(), seq)
    aJobEntry := le.findAnUndoneJob()
    if (aJobEntry == nil) {
        return
    }
    jobIndex := aJobEntry.SeqNum
    le.debug("[ReceiveRequestForAnotherPiece] give it No." + strconv.Itoa(jobIndex))
    le.lock.Lock()
    le.JobMap[jobIndex].Node = nodeid.String()
    le.JobMap[jobIndex].Status = 0
    le.lock.Unlock()
    le.SendAnotherPieceToClient(jobIndex, nodeid)
}
func (le *Leader) markNodeLastJobDone(nodeid string, seq int) {
    if (le.IsDone) {
        return
    }
    le.debug("[markNodeLastJobDone] was " + strconv.Itoa(seq))
    le.lock.Lock()
    le.JobMap[seq].Status = 1
    le.lock.Unlock()
    le.updateToBackup(seq, nodeid, 1)
}

func (le *Leader) SendAnotherPieceToClient(jobIndex int, nodeid wendy.NodeID) {
    if (le.IsDone) {
        return
    }
    le.lock.RLock()
    jobentry := le.JobMap[jobIndex]
    payload := NewJobMessage{SeqNum: jobentry.SeqNum,
                             HashType: le.wholeJob.HashType,
                             HashValue: le.wholeJob.HashValue,
                             Pwdlength: le.wholeJob.Pwdlength,
                             Start: jobentry.Start,
                             End:   jobentry.End}
    le.lock.RUnlock()
    data, err := bson.Marshal(payload)
    msg := le.cluster.NewMessage(GIVE_ANOTHER_PIECE, nodeid, data)
    err = le.cluster.Send(msg)
    if (err != nil) {
        return
    } else {
        le.lock.Lock()
        le.JobMap[jobIndex].Node = nodeid.String()
        le.JobMap[jobIndex].Status = 0
        le.lock.Unlock()
        le.updateToBackup(jobIndex, nodeid.String(), 0)
    }
    le.printJobMap()
}
func (le *Leader) findAnUndoneJob() *JobEntry{
    le.lock.RLock()
    defer le.lock.RUnlock()
    for i := 0; i < len(le.JobMap); i++ {
        if (le.JobMap[i].Status == -1) {
            return &(le.JobMap[i])
        }
	}
    le.debug("[findAnUndoneJob] no undone job")
    return nil
}
func (le *Leader) ContactBackup() {
    if (le.IsDone) {
        return
    }
    le.debug("[ContactBackup]")
    //----- chose one backup for now
    nodes := le.cluster.GetListOfNodes()
    if (len(nodes) < 2) {
        return
    }
    for _, nodeIterate := range nodes {
        if (nodeIterate.ID.Equals(le.self.ID)) {
            continue
        }
        var backupID wendy.NodeID
        backupID = nodeIterate.ID
        payload := InitializeBackUpMessage{ChosenProposerID: le.chosenProposerID.String(), BackUps: backupID.String(), TheWholeJob: *(le.wholeJob), JobMap: le.JobMap}
        data, err := json.Marshal(payload)
        if err != nil {
            panic(err)
        }
        message := le.cluster.NewMessage(INIT_BACKUP, backupID, data)
        err = le.cluster.Send(message)
        if err != nil {
            le.debug("[ContactBackup] This Backup candidate died, change")
            continue
        } else {
            le.setBackup(backupID)
            break
        }
    }
}
func (le *Leader) updateToBackup(seqn int, nodeid string, stat int) {
    if (le.IsDone) {
        return
    }
    le.increaseUpdateTimeStamp()
    le.debug("[updateToBackup]")

    payload := UpdateBackUpMessage{SeqNum: seqn,
                                   NodeiD: nodeid,
                                   Status: stat}
    data, err := bson.Marshal(payload)
    if err != nil {
        panic(err)
    }
    msg := le.cluster.NewMessage(UPDATE_BACKUP, le.BackUps, data)
    err = le.cluster.Send(msg)
    if err != nil {
        le.debug("[updateToBackup] backup failse, find new")
        le.ContactBackup()
    }
}
func (le *Leader) BackUpUpdateBackUp(seqn int, nodeid string, stat int) {
    if (le.IsDone) {
        return
    }
    le.debug("[BackUpUpdateBackUp]")
    le.lock.Lock()
    le.JobMap[seqn].Node = nodeid
    le.JobMap[seqn].Status = stat
    le.lock.Unlock()
    le.printJobMap()
}
func (le *Leader) BackUpBecomeLeader() {
    le.lock.Lock()
    le.isActive = true
    le.isBackup = false
    le.lock.Unlock()

}
func (le *Leader) removeBackUp() {

}

func (le *Leader) setBackup(bid wendy.NodeID) {
    le.lock.Lock()
    le.BackUps = bid
    le.lock.Unlock()
}
//unmarshall
func (le *Leader) BackUpReceiveInitFromLeader(msg wendy.Message) {
    le.debug("[BackUpReceiveInitFromLeader]")
    newInitializeBackUpMessage := &InitializeBackUpMessage{}
    err := json.Unmarshal(msg.Value, newInitializeBackUpMessage)
    if (err != nil) {
        panic(err)
    }
    le.lock.Lock()
    le.chosenProposerID, _ =  wendy.NodeIDFromBytes([]byte(newInitializeBackUpMessage.ChosenProposerID))
    le.BackUps, _ = wendy.NodeIDFromBytes([]byte(newInitializeBackUpMessage.BackUps))
    le.JobMap = newInitializeBackUpMessage.JobMap
    le.wholeJob = NewWholeJob(newInitializeBackUpMessage.TheWholeJob.HashType,
                                newInitializeBackUpMessage.TheWholeJob.HashValue,
                                newInitializeBackUpMessage.TheWholeJob.Pwdlength)
    le.isBackup = true
    le.lock.Unlock()
    le.printJobMap()
}
func (le *Leader) getTotoalKeySpace(mask string) int {
	var keySpace int
	var app string
	var parts []string
    app = "./hashcat-3.5.0/hashcat"

	arg0 := "--keyspace"
	arg1 := "-a"
	arg2 := "3"
	arg3 := "--session"
	cmd := exec.Command(app, arg0, arg1, arg2,arg3, le.self.ID.String(), mask)

	stdout, err := cmd.Output()
	if err != nil {
		le.debug(err.Error())
		return -1
	}
	// parsing output
	if runtime.GOOS == "windows" {
		parts = strings.Split(string(stdout), "\r\n")
	} else  {
		parts = strings.Split(string(stdout), "\n")
	}

	keySpace, err = strconv.Atoi(parts[0])
	if err != nil {
		le.debug(err.Error())
		return -1
	}
	return keySpace
}


func (le *Leader) replaceBackup() {


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
func (le *Leader) GetIsWorking() bool {
    le.lock.RLock()
	defer le.lock.RUnlock()
    return le.IsWorking
}
func (le *Leader) GetIsBackUp() bool {
    le.lock.RLock()
	defer le.lock.RUnlock()
    return le.isBackup
}
func (le *Leader) GetIsDone() bool {
    le.lock.RLock()
	defer le.lock.RUnlock()
    return le.IsDone
}
func (le *Leader) GetActive() bool {
    le.lock.RLock()
	defer le.lock.RUnlock()
    return le.isActive
}
func (le *Leader) GetBackUp() bool {
    le.lock.RLock()
	defer le.lock.RUnlock()
    return le.isBackup
}
func (le *Leader) SetDone() {
    le.debug("[SetDone]")
    le.lock.Lock()
    le.IsDone = true
    le.lock.Unlock()
}
func (le *Leader) SetActive() {
    le.debug("[SetActive]")
    le.lock.Lock()
    le.isActive = true
    le.lock.Unlock()
}

func (le *Leader) debug(format string, v ...interface{}) {
	if le.logLevel <= LogLevelDebug {
		le.log.Printf(format, v...)
	}
}
