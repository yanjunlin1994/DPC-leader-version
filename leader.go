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
type WholeJob struct {
    HashType           int
    HashValue          string
    Pwdlength          int
}
type Leader struct {
    isActive           bool  //if I am the leader now, I am active. Otherwise not active
    isBackup           bool  //if I am the back up node for leader
    BackUps            wendy.NodeID
	self               *wendy.Node
    cluster            *wendy.Cluster
	chosenProposerID   wendy.NodeID   //current proposer's ID
    routineTimeStamp   int           //Time stamp to communicate with every worker
    updateTimeStamp    int          //Time stamp to update status to back up
    JobMap             map[string]string
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
func NewLeader(self *wendy.Node, cluster *wendy.Cluster) *Leader {
	return &Leader{
        isActive:           false,
        isBackup:           false,
        BackUps:            wendy.EmptyNodeID(),
		self:               self,
        cluster:            cluster,
        chosenProposerID:   wendy.EmptyNodeID(),
        routineTimeStamp:   -1,
        updateTimeStamp:    -1,
        JobMap:             map[string]string{},
        wholeJob:           nil,
		lock:               new(sync.RWMutex),
        log:                log.New(os.Stdout, "LEADER ("+self.ID.String()+") ", log.LstdFlags),
		logLevel:           LogLevelDebug,
	}
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
    le.contactBackup()
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
    le.wholeJob = NewWholeJob(ht, Hash, Pwdlength)

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
	limit := keySpace / (2 * nodesCount)
    pos := 0
    for {
        if (pos + limit >  keySpace) {
            str := strconv.Itoa(pos) + "," + strconv.Itoa(keySpace)
            le.JobMap[str] = "undone"
            le.debug("[constructJobMap] str: " + str)
            break
        }
        str := strconv.Itoa(pos) + "," + strconv.Itoa(pos + limit)
        le.JobMap[str] = "undone"
        pos = pos + limit
        le.debug("[constructJobMap] str: " + str)
    }
    return nil
}
func (le *Leader) contactBackup() {
    //TODO: if can't contact, use another node
    le.debug("[contactBackup]")
    //----- chose one backup for now
    le.setBackup(le.chosenProposerID)
    payload := InitializeBackUpMessage{chosenProposerID: le.chosenProposerID.String(),
                                BackUps: le.BackUps.String(), JobMap: le.JobMap}
    data, err := bson.Marshal(payload)
    if err != nil {
        panic(err)
    }

    message := le.cluster.NewMessage(INIT_BACKUP, le.BackUps, data)
    err = le.cluster.Send(message)
    if err != nil {
        panic(err.Error())
    }

}
func (le *Leader) removeBackUp() {

}
func (le *Leader) setBackup(bid wendy.NodeID) {
    le.BackUps = bid
}
//unmarshall
func (le *Leader) BackUpReceiveInitFromLeader(msg wendy.Message) {
    le.debug("[BackUpReceiveInitFromLeader]")
    newInitializeBackUpMessage := &InitializeBackUpMessage{}
    err := bson.Unmarshal(msg.Value, &newInitializeBackUpMessage)
    if (err != nil) {
        panic(err)
    }
    le.lock.Lock()
    le.chosenProposerID, _ =  wendy.NodeIDFromBytes([]byte(newInitializeBackUpMessage.chosenProposerID))
    le.BackUps, _ = wendy.NodeIDFromBytes([]byte(newInitializeBackUpMessage.BackUps))
    le.JobMap = newInitializeBackUpMessage.JobMap
    le.isBackup = true
    le.lock.Unlock()

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

func (le *Leader) updateToBackup() {


}
func (le *Leader) replaceBackup() {


}
func (le *Leader) UpdateToBackUp() {
    le.debug("[UpdateToBackUp]")
    le.increaseUpdateTimeStamp()
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
