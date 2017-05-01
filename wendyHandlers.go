package main

import (
	"fmt"
	"./wendy-modified"
	"gopkg.in/mgo.v2/bson"
	"encoding/json"
    "strconv"
)
// Distributed Password Cracker struct
type WendyHandlers struct {
	node    *wendy.Node
	// cluster *wendy.Cluster
}
//--------------------------------Leader Communication part--------------------
func (app *WendyHandlers) OnLeaderElectionDeliver(msg wendy.Message) {
    fmt.Println("[OnLeaderElectionDeliver] Message purpose is: ", msg.Purpose)

	switch p := msg.Purpose; p {

    	case LEADER_VIC: // leader Victory message
            fmt.Println("[OnLeaderElectionDeliver]LEADER_VIC")
            leaderComm.ReceiveVictoryFromLeader(msg.Sender.ID)
        case YOU_ARE_LEADER: // leader is notified that he is the leader
            fmt.Println("[OnLeaderElectionDeliver] YOU_ARE_LEADER")
            afterBecomeALeader()
    }
}
func afterBecomeALeader() {
    leader.SetActive()
    leaderComm.BecomeLeader()
}
func (app *WendyHandlers) OnNewProposalDeliver(msg wendy.Message) {
    fmt.Println("[OnNewProposalDeliver] Message purpose is: ", msg.Purpose)

	switch p := msg.Purpose; p {

    	case WANT_TO_CRACK: //leader receive someone's request to crack
            fmt.Println("[OnNewProposalDeliver]WANT_TO_CRACK")
            err := leader.CheckNewProposalAndRespond(msg)
            if err != nil {
                panic(err)
            }
        case LEADER_JUDGE://requester receive Leader judgement
            fmt.Println("[OnNewProposalDeliver]LEADER_JUDGE")
            var agree bool
            err := json.Unmarshal(msg.Value, &agree)
            if err != nil {
                fmt.Println(err)
                return
            }
            fmt.Printf("[OnNewProposalDeliver] %t \n", agree)
            leaderComm.ReceiveLeaderJudgement(agree)
        case CRACK_DETAIL:
            fmt.Println("[OnNewProposalDeliver]CRACK_DETAIL")
            real := leader.CheckCorrectProposer(msg.Sender.ID)
            if !real {
                fmt.Println("[OnNewProposalDeliver]wrong proposer, ignore")
                return
            }
            jobDetail := &CrackJobDetailsMessage{}
    		err := bson.Unmarshal(msg.Value, jobDetail)
    		if err != nil {
    			panic(err)
    		}
            leader.PreapreJob(jobDetail.HashType, jobDetail.Hash, jobDetail.Pwdlength)
    }
}
func (app *WendyHandlers) OnNewBackUpInit(msg wendy.Message) {
    fmt.Println("[Handler] OnNewBackUpInit")
    leader.BackUpReceiveInitFromLeader(msg)
}
func (app *WendyHandlers) OnFirstJob(msg wendy.Message) {
    fmt.Println("[Handler] OnFirstJob")
    newJob := &NewJobMessage{}
    err := bson.Unmarshal(msg.Value, newJob)
    if err != nil {
        panic(err)
    }
    fmt.Println("[Handler] OnFirstJob No." + strconv.Itoa(newJob.SeqNum) +" type: " + strconv.Itoa(newJob.HashType) +
                " hashvalue: " + newJob.HashValue + " length: " +
                strconv.Itoa(newJob.Pwdlength) + " start: " +
                strconv.Itoa(newJob.Start) + " end: " + strconv.Itoa(newJob.End))

    job = NewJob(selfnode, cluster, leaderComm, newJob.SeqNum, newJob.HashType,
    newJob.HashValue, newJob.Pwdlength, newJob.Start, (newJob.End - newJob.Start))
    go job.StartJob()

}
func (app *WendyHandlers) OnReceiveFoundPass(msg wendy.Message) {
    fmt.Println("[Handler] OnReceiveFoundPass")
    job.ReceiveFoundPass(msg)
}
func (app *WendyHandlers) OnAskAnotherPiece(msg wendy.Message) {
    fmt.Println("[Handler] OnAskAnotherPiece, from client")
    askJob := &AskAnotherMessage{}
    err := bson.Unmarshal(msg.Value, askJob)
    if err != nil {
        panic(err)
    }
    leader.ReceiveRequestForAnotherPiece(msg.Sender.ID, askJob.SeqNum)

}
func (app *WendyHandlers) OnRecvAnotherPiece(msg wendy.Message) {
    fmt.Println("[Handler] OnRecvAnotherPiece, from Leader")
    newJob := &NewJobMessage{}
    err := bson.Unmarshal(msg.Value, newJob)
    if err != nil {
        panic(err)
    }
    job = NewJob(selfnode, cluster, leaderComm, newJob.SeqNum, newJob.HashType,
    newJob.HashValue, newJob.Pwdlength, newJob.Start, (newJob.End - newJob.Start))
    fmt.Println("[Handler] Another Piece No." + strconv.Itoa(newJob.SeqNum) +
                " type: " + strconv.Itoa(newJob.HashType) +
                " hashvalue: " + newJob.HashValue + " length: " +
                strconv.Itoa(newJob.Pwdlength) + " start: " +
                strconv.Itoa(newJob.Start) + " end: " + strconv.Itoa(newJob.End))
    go job.StartJob()
}
func (app *WendyHandlers) OnBackUpRecvUpdate(msg wendy.Message) {
    fmt.Println("[Handler] On BackUp RecvUpdate, from Leader")
    newUpdate := &UpdateBackUpMessage{}
    err := bson.Unmarshal(msg.Value, newUpdate)
    if err != nil {
        panic(err)
    }
    leader.BackUpUpdateBackUp(newUpdate.SeqNum, newUpdate.NodeiD, newUpdate.Status)
}
func (app *WendyHandlers) OnDeliver(msg wendy.Message) {
	fmt.Println("[Handler][OnDeliver] Message purpose is: ", msg.Purpose)
	switch p := msg.Purpose; p {

	case NEW_JOB: // New Job
        fmt.Println("[Handler] NEW_JOB")
	case STOP_JOB: // Stop Job
        fmt.Println("[Handler] STOP_JOB")
	case FOUND_PASS: // Found Password
        fmt.Println("[Handler] FOUND_PASS")
    }

}

func (app *WendyHandlers) OnNodeJoin(node wendy.Node) {
	fmt.Println("[Handler]Node joined: ", node.ID)
    if (leader.GetIsWorking()) {
        fmt.Println("[Handler]Node joined. leader working")
        leader.HandleNewNode(node.ID)
    }
}

func (app *WendyHandlers) OnNodeExit(node wendy.Node) {
	fmt.Println("[Handler]Node left: ", node.ID)
    //Leader check whether it is working on a job, if yes, reset the job entry
    if (leader.GetIsWorking()) {
        fmt.Println("[Handler]Node left. leader working")
        leader.HandleNodeLeft(node.ID)
    }
    //backup check whether it is leader died. if yes, become the next leader
    if (node.ID.Equals(leaderComm.GetCurrentLeader())) && (leader.GetIsBackUp()) {
        fmt.Println("[Handler]Leader left. Backup become leader")
        leader.HandleNodeLeft(node.ID)
        leaderComm.BecomeLeader()
        leader.BackUpBecomeLeader()
        leader.ContactBackup()
        leaderComm.ProcessRequestToFormerLeader()
    }

}

func (app *WendyHandlers) OnError(err error) {
	fmt.Println("[Handler]Error: ", err.Error())
}
func (app *WendyHandlers) OnNewLeaves(leaves []*wendy.Node) {
	//fmt.Println("Leaf set changed: ", leaves)
}
func (app *WendyHandlers) OnHeartbeat(node wendy.Node) {
	//fmt.Println("Received heartbeat from ", node.ID)
}
