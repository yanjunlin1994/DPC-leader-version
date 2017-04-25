package main

// go get [package name]
import (
	"fmt"

	"./wendy-modified"
	"gopkg.in/mgo.v2/bson"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"runtime"
	"strings"
	"encoding/json"
)
// var wg sync.WaitGroup
// Distributed Password Cracker struct
type WendyHandlers struct {
	node    *wendy.Node
	cluster *wendy.Cluster
}

// Send a message
func (app *WendyHandlers) SendMessage(messType MessageType, destination string, contents interface{}) {
	// Create NodeID from destination string
	dest, err := wendy.NodeIDFromBytes([]byte(destination))
	if err != nil {
		panic(err)
	}

	// Marshall message contents
	data, err := bson.Marshal(contents)
	if err != nil {
		panic(err)
	}

	// Create Message object
	message := app.cluster.NewMessage(byte(messType), dest, data)

	// Send message out to destination
	err = app.cluster.Send(message)
	if err != nil {
		panic(err.Error())
	}
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
    leader.ContackBackup()
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
            fmt.Printf("[OnNewProposalDeliver] %t", agree)
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
            go initiateCracking(jobDetail.HashType, jobDetail.Hash, jobDetail.Pwdlength)
    }
}
func (app *WendyHandlers) OnDeliver(msg wendy.Message) {
	fmt.Println("[OnDeliver] Message purpose is: ", msg.Purpose)
	var nextNode *wendy.Node

	// Look at messageStruct.go for message details
	switch p := msg.Purpose; p {

	case NEW_JOB: // New Job
		mask := ""
		tmpList := &NewJobMessage{}
		err := bson.Unmarshal(msg.Value, tmpList)
		if err != nil {
			panic(err)
		}

		// If newjobmessage is back at origin and version is newest, send start
		if tmpList.Origin == selfnode.ID.String() {
			//fmt.Println("Handler: Received own job back.")
			// Check version number against waitingNewJob to decide to start or ignore
			if tmpList.Version == waitingNewJob.Version {
				//fmt.Println("Handler: Version check matches. Starting up HC.")
				newJobData := waitingNewJob
				waitingNewJob = nil
				// Start job locally and update jobList with verified
				for _, entry := range jobList {
					if entry.HashType == tmpList.HashType &&
						entry.HashValue == tmpList.HashValue &&
						entry.Origin == tmpList.Origin{

						entry.Verified = true
						break
					}
				}

				// Create mask for password length
				for i := 0; i < newJobData.Length; i++ {
					mask = mask + "?l"
				}

				isRunning := BeginCracking(newJobData.HashType, newJobData.HashValue, mask, newJobData.Start, newJobData.Limit)

				// Send start job message to ring
				// Craft start job message

				payload := StartJobMessage{HashType: newJobData.HashType, HashValue: newJobData.HashValue, Origin: selfnode.ID.String()}
				data, err := bson.Marshal(payload)
				if err != nil {
					panic(err)
					return
				}

				// Pass message around till origin
				nodes := cluster.GetListOfNodes()
				index := 0
				for i, nodeIterate := range nodes {
					if nodeIterate.ID.Equals(selfnode.ID) {
						index = i
						break
					}
				}

				// Loop back to start if the node is last in list
				if (index+1) == cluster.NumOfNodes(){
					index = -1
				}
				nextNode = nodes[index+1]
				// Don't send if origin
				if nextNode.ID.String() != tmpList.Origin {
					msg := cluster.NewMessage(START_JOB, nextNode.ID, data)
					err := cluster.Send(msg)
					if err != nil {
						fmt.Println("Handler: Error with sending START_JOB message.")
						panic(err)
					}
				}

				if isRunning {
					optiontoStop()

					// Wait for the running process to complete

					wg.Wait()
					fmt.Println("\n\nProcess completed\n\n")
					repeatJob(2)
				} else {
					fmt.Println("Sorry for that!")
					repeatJob(1)
				}

				return
			} else{ // Old job that can be ignored
				return
			}

		} else { // Record job data down in joblist
			// Check of duplicate job for updating joblist
			for index, entry := range jobList {
				if entry.HashValue == tmpList.HashValue &&
					entry.HashType == tmpList.HashType &&
					entry.Origin != tmpList.Origin {
					//fmt.Println("Handler: Updating new job")
					// Update entry with newest list
					jobList[index].Participants = createJobNodeList()
					return
				}
			}

			//fmt.Println("Handler: New job arrived. Registering.")
			// Add job to list and hold off cracking till startJob arrives
			jobList = append(jobList, job{tmpList.HashType, tmpList.HashValue, tmpList.Origin,
			createJobNodeList(), tmpList.Length, tmpList.Start, tmpList.Limit, false})

			//BeginCracking(tmpList.HashType, tmpList.HashValue, mask, tmpList.Start, tmpList.Limit)

			//Pass on the message to next node after updating the start value of keyspace
			nextStart := tmpList.Start + tmpList.Limit
			payload := NewJobMessage{HashType: tmpList.HashType, HashValue: tmpList.HashValue,
				Length: tmpList.Length, Start: nextStart, Limit: tmpList.Limit, Origin: tmpList.Origin, Version:tmpList.Version}
			data, err := bson.Marshal(payload)
			if err != nil {
				panic(err)
				return
			}

			nodes := cluster.GetListOfNodes()
			index := 0
			for i, nodeIterate := range nodes {
				if nodeIterate.ID.Equals(selfnode.ID) {
					index = i
					break
				}
			}

			// Loop back to start if the node is last in list
			if (index+1) == cluster.NumOfNodes(){
				index = -1
			}

			nextNode = nodes[index+1]

			//fmt.Println("Handler: Sending NEW_JOB to " + nextNode.ID.String())

			msg := cluster.NewMessage(NEW_JOB, nextNode.ID, data)
			err = cluster.Send(msg)
			if err != nil {
				fmt.Println("Handler: Error with sending NEW_JOB message.")
				panic(err)
			}
		}

	case START_JOB:
		mask := ""
		tmpList := &StartJobMessage{}
		err := bson.Unmarshal(msg.Value, tmpList)
		if err != nil {
			panic(err)
		}
		var isRunning bool

		// Find job info from joblist
		for _, entry := range jobList {
			if entry.HashValue == tmpList.HashValue && entry.HashType == tmpList.HashType && entry.Origin == tmpList.Origin {
				fmt.Println("Handler: Start job message. Starting.")
				entry.Verified = true

				// Create mask for password length
				for i := 0; i < entry.Length; i++ {
					mask = mask + "?l"
				}

				hashcatQueue = append(hashcatQueue, keyspaceBlock{entry.HashValue, entry.HashType,
					entry.Limit, entry.Start, true})
				// Queue up any dead node's keyspace if this node is predecessor, including multiple dead node cases
				for ind, part := range entry.Participants {
					if part.Node == selfnode.ID.String() {
						shifts := 0
						for {
							ind = ind + 1
							shifts = shifts + 1
							if ind == len(entry.Participants){
								ind = 0
							}
							if entry.Participants[ind].Alive {
								break
							} else {
								hashcatQueue = append(hashcatQueue, keyspaceBlock{entry.HashValue, entry.HashType,
														entry.Limit, entry.Start+(shifts*entry.Limit),false})
								fmt.Println("Handler: Adding abandoned keyspace to queue before start job.")
							}
						}
						break
					}
				}

				// Start job
				isRunning = BeginCracking(entry.HashType, entry.HashValue, mask, entry.Start, entry.Limit)
				break
			}
		}

		// Pass message around till origin
		nodes := cluster.GetListOfNodes()
		index := 0
		for i, nodeIterate := range nodes {
			if nodeIterate.ID.Equals(selfnode.ID) {
				index = i
			}
		}

		// Loop back to start if the node is last in list
		if (index+1) == cluster.NumOfNodes(){
			index = -1
		}
		nextNode = nodes[index+1]
		// Don't send if origin
		if nextNode.ID.String() != tmpList.Origin {
			msg := cluster.NewMessage(START_JOB, nextNode.ID, msg.Value)
			cluster.Send(msg)
		}

		if isRunning {
			optiontoStop()

			// Wait for the running process to complete
			wg.Wait()
			fmt.Println("\n\nProcess completed\n\n")
			repeatJob(2)
		} else {
			fmt.Println("Sorry for that!")
			repeatJob(1)
		}

		return

	case STOP_JOB: // Stop Job
		tmpList := &StopJobMessage{}
		err := bson.Unmarshal(msg.Value, tmpList)
		if err != nil {
			panic(err)
		}

		// Stop current job
		//killLocalProcess()

		// Now, pass stop message to your successor
		nodesCount := cluster.NumOfNodes()
		nodes := cluster.GetListOfNodes()

		data, err := bson.Marshal(tmpList)
		if err != nil {
			fmt.Println(err)
			return
		}

		// Send message on to next person if more than just self
		if nodesCount > 1 {
			// Find index of self in list
			index := 0
			for i, nodeIterate := range nodes {
				if nodeIterate.ID.Equals(selfnode.ID) {
					index = i
				}
			}

			// Loop back to start if the node is last in list
			if (index + 1) == cluster.NumOfNodes(){
				index = -1
			}
			nextNode = nodes[index+1]

			msg := cluster.NewMessage(STOP_JOB, nextNode.ID, data)
			cluster.Send(msg)
		}

	case FOUND_PASS: // Found Password
        unBlockMyCluster()
		tmpList := &FoundMessage{}
		err := bson.Unmarshal(msg.Value, tmpList)
		if err != nil {
			panic(err)
		}

		//first, verify if we are running a local process with these details
		if isRunningLocally(tmpList.HashType, tmpList.HashValue) {
			//Bson marhalling seems to be adding \r\n (or) \n to the password being marshalled
			var receivedPassword []string

			if runtime.GOOS == "windows" {
				receivedPassword = strings.Split(tmpList.Password, "\r\n")
			} else {
				receivedPassword = strings.Split(tmpList.Password, "\n")
			}



			//Verify password,hash pair before killing our own process
			if tmpList.HashType == 0 {
                fmt.Println("\n receivedPassword is :",receivedPassword[0])
                fmt.Println("\n current jobs hash value is :", currentJob.HashValue)

				ReceivedPasswordHash := md5.Sum([]byte(receivedPassword[0]))
                fmt.Println("\n ReceivedPasswordHash in hex is: ", ReceivedPasswordHash)
                fmt.Println("\n ReceivedPasswordHash after convert is: ", hex.EncodeToString(ReceivedPasswordHash[:]))

				if hex.EncodeToString(ReceivedPasswordHash[:]) == currentJob.HashValue {
					fmt.Println("Node \"",tmpList.Origin,"\" found the password! So, killing local process corresponding to this job")
					//fmt.Println("before triggering killlocal")
					// killLocalProcess()
					//fmt.Println("after triggering killlocal")
					//unBlockMyCluster()
					repeatJob(1)
				}
			} else if tmpList.HashType == 100 {
				ReceivedPasswordHash := sha1.Sum([]byte(receivedPassword[0]))
				if hex.EncodeToString(ReceivedPasswordHash[:]) == currentJob.HashValue {
					fmt.Println("Node \"",tmpList.Origin,"\" found the password! So, killing local process corresponding to this job")

					//	this need to be fixed: killLocalProcess()
				//	unBlockMyCluster()
					repeatJob(1)
				}
			} else if tmpList.HashType == 1400 {
				ReceivedPasswordHash := sha256.Sum256([]byte(receivedPassword[0]))
				if hex.EncodeToString(ReceivedPasswordHash[:]) == currentJob.HashValue {
					fmt.Println("Node \"",tmpList.Origin,"\" found the password! So, killing local process corresponding to this job")

					//this need to be fixed: killLocalProcess()
				//	unBlockMyCluster()
					repeatJob(1)
				}
			}

		} else {
			fmt.Println("Received a message saying that the password has been found though there is no such process running on your machine!")
			fmt.Println("Needs investigation: Might be a bogus node got access to network sending bogus messages to waste system resources or else some other reason")
		}
	}
}

// Unblock my cluster
func unBlockMyCluster() {
	unBlockMsg := cluster.NewMessage(UNBLOCK_CLUSTER, selfnode.ID, []byte{})
	cluster.Send(unBlockMsg)
}

// We will verfy if same hashtype and hash value cracking job is running locally
func isRunningLocally(hashType int, hashValue string) bool {
	if currentJob.HashValue == hashValue && currentJob.HashType == hashType {
		return true
	}
	return false
}

// Stop current job (HashcatJob located as global var in cracker.go)
func killLocalProcess() {
	// Stop current job (HashcatJob located as global var in cracker.go)
	fmt.Println("inside killlocal process")

	if HashcatJob != nil {
		err := HashcatJob.Process.Kill()
		if err != nil {
			fmt.Println("Failed to kill hashcat on stop message?")
			fmt.Println(HashcatJob.Process)
			panic(err.Error())
		} else {
			fmt.Println(err.Error())
			if err == nil {
				fmt.Print("am I nill?")
			}
			fmt.Println("I killed hashcat")
		}
		HashcatJob = nil
	} else { //for debug
		fmt.Print("Hashcat alrdy killed")
	}

	HashcatJob = nil
	doneJob()
	return
}

func (app *WendyHandlers) OnNodeJoin(node wendy.Node) {
	fmt.Println("Node joined: ", node.ID)
}

func (app *WendyHandlers) OnNodeExit(node wendy.Node) {
	fmt.Println("Node left: ", node.ID)

	// 1. If current job running, determine if node needs to take over
	if currentJob != nil {
		for index, entry := range currentJob.Participants {
			if entry.Node == node.ID.String(){
				entry.Alive = false // Mark node dead
				// Find index of first alive predecessor
				preindex := index
				precount := 1
				for currentJob.Participants[preindex].Alive == false {
					preindex = preindex - 1
					precount = precount + 1
					if preindex == -1 {
						preindex = len(currentJob.Participants) -1
					}
				}
				// If this node is owner...
				if currentJob.Participants[preindex].Node == selfnode.ID.String() {
					takeOverNum := 0
					// Calculate how many keyspace blocks to takeover
					for i := index; currentJob.Participants[i].Node != selfnode.ID.String(); {
						if currentJob.Participants[i].Alive == false {
							hashcatQueue = append(hashcatQueue, keyspaceBlock{currentJob.HashValue,
								currentJob.HashType, currentJob.Limit,
								(currentJob.Start+currentJob.Limit*precount)+currentJob.Limit*takeOverNum,false})
							fmt.Println("Handler: Adding abandoned keyspace to queue after Node Exit.")
							takeOverNum = takeOverNum + 1
						} else {
							break
						}

						// Increment to next node in chain
						i++
						if i == len(currentJob.Participants) {
							i = 0
						}
					}
				}
				break
			}
		}

	}

	// 2. Update other jobs in jobList with dead node status
	for _, entry1 := range jobList {
		for _, entry2 := range entry1.Participants {
			if node.ID.String() == entry2.Node {
				entry2.Alive = false
			}
		}
	}

	// 3. If currently sending new job out, resend and update own list
	if waitingNewJob != nil {
		fmt.Println("Handler: Sending updated New Job after Node Exit.")
		// Update version number and resend
		waitingNewJob.Version = waitingNewJob.Version + 1

		// Recalculate limits for hashcat based on new length
		mask := ""
		for i := 0; i < pwdlength; i++ {
			mask = mask + "?l"
		}
		keySpace := GetKeySpace(mask)
		nodesCount := cluster.NumOfNodes()
		limit := keySpace / nodesCount
		//fix if keyspace is exactly not divisable: no problem if we give additional keyspace as hashcat does only till the real keyspace and ignores the additional one
		limit++

		// Update joblist entry with new limit info and node list
		// Update waitingNewJob with new limit info
		waitingNewJob.Limit = limit
		for _, entry := range jobList {
			if entry.HashValue == waitingNewJob.HashValue && entry.HashType == waitingNewJob.HashType {
				entry.Participants = createJobNodeList()
				entry.Limit = limit
			}
		}

		// Update other nodes
		tmpEntry := NewJobMessage{waitingNewJob.HashType, waitingNewJob.HashValue, waitingNewJob.Length,
			limit, limit, selfnode.ID.String(), waitingNewJob.Version}

		data, err := bson.Marshal(tmpEntry)
		if err != nil {
			fmt.Println(err)
			return
		}
		nodes := cluster.GetListOfNodes()
		if cluster.NumOfNodes() > 1 {
			// Find index of self in list
			index := 0
			for i, nodeIterate := range nodes {
				if nodeIterate.ID.Equals(selfnode.ID) {
					index = i
				}
			}

			// Loop back to start if the node is last in list
			if (index + 1) == cluster.NumOfNodes(){
				index = -1
			}
			nextNode := nodes[index+1]

			msg := cluster.NewMessage(NEW_JOB, nextNode.ID, data)
			err := cluster.Send(msg)
			if err != nil {
				fmt.Println("Handler: Origin failed to send NEW_JOB message.")
				panic(err)
			}
		}
	}

}

func (app *WendyHandlers) OnError(err error) {
	fmt.Println("Error: ", err.Error())
}

//func (app *WendyHandlers) OnForward(msg *wendy.Message, next wendy.NodeID) bool {
//	//fmt.Printf("Forwarding message %s to Node %s.\n", msg.Key, next)
//	return true // return false if you don't want the message forwarded
//}

func (app *WendyHandlers) OnNewLeaves(leaves []*wendy.Node) {
	//fmt.Println("Leaf set changed: ", leaves)
}

func (app *WendyHandlers) OnHeartbeat(node wendy.Node) {
	//fmt.Println("Received heartbeat from ", node.ID)
}
