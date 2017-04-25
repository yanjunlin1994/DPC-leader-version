package main

import (
	"bytes"
	"fmt"
	"net/http"
	"net"
	"os"
	"math/rand"
	"time"

	"./wendy-modified"
	"github.com/twinj/uuid"
	"gopkg.in/mgo.v2/bson"
	"io/ioutil"
)

var cluster *wendy.Cluster
var selfnode *wendy.Node
var leaderComm *LeaderComm
var leader *Leader
var jobList []job // List of all jobs (currently just appends to end, but doesn't remove)
var currentJob *job // Current job that is cleared on password verify or stop job

// Struct for node entry for job entry
type jobNode struct {
	Node string
	Alive bool
}

// Struct for a job entry
type job struct {
	HashType  int // Type of hash
	HashValue string // Hash value
	Origin    string // Original creator of job
	Participants []jobNode // Updated list of nodes participating
	Length    int
	Start     int
	Limit     int
	Verified  bool
}

// Returns a list of currently connected nodes from Wendy
func createJobNodeList() []jobNode{
	var participants []jobNode
	for _, entry := range cluster.GetListOfNodes(){
		tmp := jobNode{entry.ID.String(), true}
		participants = append(participants, tmp)
	}
	return participants
}

// Remove current job in joblist and clear currentJob
func doneJob() {
	for index, entry := range jobList {
		if entry.HashValue == currentJob.HashValue && entry.HashType == currentJob.HashType && entry.Origin == currentJob.Origin{
			jobList = append(jobList[:index], jobList[index+1:]...)
			break
		}
	}
	hashcatQueue = nil
	currentJob = nil
}

// Called by origin to stop hashcat and job
func stopJob() {
	var nextNode *wendy.Node

	if currentJob == nil {
		fmt.Println("No current job to stop. Your process might have been already killed")
		return
	}

	// Stop hashcat if its still running
	if HashcatJob != nil {
		err := HashcatJob.Process.Kill()
		if err != nil {
			fmt.Println("Failed to kill hashcat on stop message?")
			panic(err)
		}
		HashcatJob = nil
	}

	// Now, send a message to your successor to stop the process
	nodesCount := cluster.NumOfNodes()
	nodes := cluster.GetListOfNodes()

	payload := StopJobMessage{currentJob.HashValue,currentJob.HashType, currentJob.Origin}

	data, err := bson.Marshal(payload)
	if err != nil {
		fmt.Println("err in clusterManagement:marshall")
		panic(err)
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

	doneJob()
}

func ClusterManagement(option int) {
	var wendyPort int = 18818

	if option == 2 {
		// randomly generate port number
		wendyPort = Random(11000, 19000)
	}

	var region string = "home"

	fileContents, err := ioutil.ReadFile("clusterkey.txt")
	if err != nil {
		fmt.Print(err)
	}

	var credentials wendy.Passphrase = wendy.Passphrase(fileContents)

	var heartbeatFreq int = 30
	var entryNodeIP string

	//var entryNodeIPs []string

	if option == 2 {
        entryNodeIP = "127.0.0.1"
	//var entryNames = []string{"yanjunlin.hopto.org", "yanjunlin1.hopto.org", "yanjunlin2.hopto.org"}
        // entryNodeIPs = addrLookup(entryNames)
    	}
	var entryNodePort int = 18818

	// Generate UUID and use for Wendy Node ID
	u4 := uuid.NewV4()
	id, err := wendy.NodeIDFromBytes(u4)
	if err != nil {
		panic(err.Error())
	}
	fmt.Println("[ClusterManagement] ID: ", id)

	// Get IP addresses
	 //var externalAddr string = getExternalIP()
	var externalAddr string = "127.0.0.1"

	fmt.Printf("[ClusterManagement] IP: %s : %d \n",externalAddr, wendyPort)
	selfnode = wendy.NewNode(id, externalAddr, externalAddr, region, wendyPort)

	cluster = wendy.NewCluster(selfnode, credentials)
	cluster.SetHeartbeatFrequency(heartbeatFreq)
	cluster.SetLogLevel(wendy.LogLevelDebug)
	// cluster.SetLogLevel(wendy.LogLevelError)

    leaderComm = NewLeaderComm(selfnode, cluster)
    leader = NewLeader(selfnode, cluster)
	// Start go routine to concurrently start listening for messages
	go func() {
		defer cluster.Stop()
		err := cluster.Listen()
		if err != nil {
			panic(err.Error())
		}
	}()

	wendyHandlers := &WendyHandlers{selfnode, cluster}
	cluster.RegisterCallback(wendyHandlers)
	// Join initial node's cluster
	if option == 2 {
		//bootstrapping [comment this out to disable bootstrapping]
		//  err := joinCluster(entryNodeIPs, entryNodePort)
        // 	 if err != nil {
        // 	 fmt.Println("[ClusterManagement] Not able to join cluster via any available entry nodes")
		// 	 panic(err)
		//  }
		//no bootstrapping
		cluster.Join(entryNodeIP, entryNodePort)
	}

	fmt.Println("[ClusterManagement] Cluster configuration done.")

}

//Join the cluster until find a reachable ip address
func joinCluster(entryNodeIPs []string, entryNodePort int) error{
    var err error
    for i := 0; i < len(entryNodeIPs); i++ {
        err = cluster.Join(entryNodeIPs[i], entryNodePort)
        if (err == nil) {
            fmt.Println("[ClusterManagement] Join cluster successes! ")
            return nil
        }
        fmt.Printf("[ClusterManagement] Joining cluster failed via ip %s. Trying next one... \n", entryNodeIPs[i])
    }
    return err
}

// Get external IPv4 address of this machine
// Source: http://myexternalip.com/#golang
func getExternalIP() string {
	resp, err := http.Get("http://ipv4.myexternalip.com/raw")
	if err != nil {
		os.Stderr.WriteString(err.Error())
		os.Stderr.WriteString("\n")
		os.Exit(1)
	}
	defer resp.Body.Close()
	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)

	return buf.String()[:len(buf.String())-1]
}
//generate a random number
func Random(min, max int) int {
	rand.Seed(time.Now().Unix())
	return rand.Intn(max - min) + min
}
//DNS lookup for given domain names
func addrLookup(domainName []string) []string {
	var entryIPs []string
	for i := 0; i < len(domainName); i++ {
		addresses, err := net.LookupHost(domainName[i])
		if (err != nil) {
			fmt.Println("[addrLookup] err: " + err.Error() + " continuing lookup ...")
			continue
		}
		entryIPs = append(entryIPs, addresses[0])
	}
	return entryIPs
}
