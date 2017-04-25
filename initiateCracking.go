package main

import (
	"fmt"
	"time"
	"os/exec"
	"strconv"
	"strings"
	"gopkg.in/mgo.v2/bson"
	"./wendy-modified"
	"runtime"
    "sync"

)

var waitingNewJob *NewJobMessage = nil // Holds newJob message while waiting for it to return through ring
var wg sync.WaitGroup

func initiateCracking(hashtype string, hash string, pwdlength int) {
	var hashType int
	var mask string
	//TODO: Later, change to int64 etc. see overflow range and change accordingly
	var start int
	var limit int
	var keySpace int

	if (strings.EqualFold(hashtype, "MD5")) == true {
		hashType = 0
	} else if (strings.EqualFold(hashtype, "SHA1")) == true {
		hashType = 100
	} else if (strings.EqualFold(hashtype, "SHA256")) == true {
		hashType = 1400
	}

	/*
	For sake of simplicity of demo, we currently support only for small characters. It is simple to extend it to alphanumeric etc. Just use the mask as below
		? | Charset
		===+=========
		l | abcdefghijklmnopqrstuvwxyz    <- we are using this
		u | ABCDEFGHIJKLMNOPQRSTUVWXYZ
		d | 0123456789
		h | 0123456789abcdef
		H | 0123456789ABCDEF
		s |  !"#$%&'()*+,-./:;<=>?@[\]^_`{|}~
  		a | ?l?u?d?s
  		b | 0x00 - 0xff
	*/

	for i := 0; i < pwdlength; i++ {
		mask = mask + "?l"
	}

	//Dividing keyspace
	//TODO: Implement capability based division (using --progress-only flag?) of hashcat (problem of uneven division for last node might still exist. Try to fix it!)
	keySpace = GetKeySpace(mask)

   	//clusterBlockHandler: set cluster to be blocked and multicast block message
	//XXX: clusterBlockManagement()
	nodesCount := cluster.NumOfNodes()
	fmt.Println("\t No. of nodes available within this network:",nodesCount,". Distributing total keyspace (", keySpace, ") among these nodes")

	start = 0
	limit = keySpace / nodesCount

	//fix if keyspace is exactly not divisable: no problem if we give additional keyspace as hashcat does only till the real keyspace and ignores the additional one
	limit++

	fmt.Println("\t Keyspace for this node is from ", start, " to ", limit)
	//Cracking process on this node
	//BeginCracking(hashType, hash, mask, start, limit)
	PassNewJobMessage(hashType, hash, pwdlength, limit, nodesCount)
}

func PassNewJobMessage(hashType int, hash string, pwdlength int, limit int, nodesCount int){
	var nextNode *wendy.Node

	// Send message on to successor node (to initiate cracking) if more than self node exists in the network
	nodes := cluster.GetListOfNodes()
	payload := NewJobMessage{HashType: hashType, HashValue: hash, Length: pwdlength,
		Start: limit, Limit: limit, Origin:selfnode.ID.String(), Version: 1}
	data, err := bson.Marshal(payload)
	if err != nil {
		fmt.Println(err)
		return
	}

	if nodesCount > 0 {
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

		msg := cluster.NewMessage(NEW_JOB, nextNode.ID, data)
		// Remember new job
		waitingNewJob = &NewJobMessage{HashType: hashType, HashValue: hash, Length: pwdlength,
			Start: 0, Limit: limit, Origin:selfnode.ID.String(), Version: 1}
		// Add to joblist
		jobList = append(jobList, job{hashType, hash, selfnode.ID.String(), createJobNodeList(),
		pwdlength, 0, limit, false})
		cluster.Send(msg)
	}
}

func GetKeySpace(mask string) int {
	var keySpace int
	var app string
	var parts []string

	if runtime.GOOS == "windows" {
		app = "./hashcat-3.5.0/hashcat"
	} else  {
		app = "hashcat"
	}

	arg0 := "--keyspace" // calcs keyspace for our cracking process
	arg1 := "-a"       // attack mode is brute force or also called as updated brute force i.e., mask attack
	arg2 := "3"
	arg3 := "--session"
	cmd := exec.Command(app, arg0, arg1, arg2,arg3, selfnode.ID.String(),mask)

	stdout, err := cmd.Output()
	if err != nil {
		fmt.Println(err.Error())
		return 10
	}

	// parsing output
	if runtime.GOOS == "windows" {
		parts = strings.Split(string(stdout), "\r\n")
	} else  {
		parts = strings.Split(string(stdout), "\n")
	}

	keySpace, err = strconv.Atoi(parts[0])
	if err != nil {
		fmt.Println(err)
		return 11
	}

	return keySpace
}

func BeginCracking(hashType int, hash string, mask string, start int, limit int) bool {
	// Record down job and set current job
	//TODO: Checks before overwriting current job
	//DO not run multiple hashcats on same machine as it kills the performance of the machine
	//TODO: What happens in case of fault tolerance?
	//TODO: Add queue for processing
	if currentJob != nil {
		fmt.Println("Job already running! So, this request is ignored")
		return false
	}

	currentJob = &job{hashType, hash, selfnode.ID.String(), createJobNodeList(), len(mask),
					  start, limit, true}

	// Let the cracking process begin!
	fmt.Println("\n\n \t Running hashcat. Please wait...! This might take long depending on the passwordlength provided by you")
	wg.Add(1)

	go HashCrack(hashType, hash, mask, start, limit, selfnode.ID.String())
	return true
}

//set cluster blocked and send block message to other nodes
func clusterBlockManagement() {
	cluster.SetBlocked()
	multicastBlockCluster()
	time.Sleep(time.Duration(4) * time.Second)
}

//multicast the block message
func multicastBlockCluster() {
	nodes := cluster.GetListOfNodes()
	nodesCount := cluster.NumOfNodes()
	payload := BlockClusterMessage{Origin: selfnode.ID.String()}
	data, err := bson.Marshal(payload)
	if err != nil {
		fmt.Println(err)
		return
	}

	//send only if there are anyone else within the network
	if nodesCount > 1 {
		for _, nodeIterate := range nodes {
			//send to everyone else except self node as it already knows it
			if !nodeIterate.ID.Equals(selfnode.ID) {
				msg := cluster.NewMessage(BLOCK_CLUSTER, nodeIterate.ID, data)
				cluster.Send(msg)
			}
		}
	}
}
