package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"gopkg.in/mgo.v2/bson"
	"strconv"
	"strings"
	"runtime"
	"time"
	//"sync"
)

var HashcatJob *exec.Cmd // Global to be able to access hashcat exec, cleared when hashcat is done/killed
var outfile string

var hashcatQueue []keyspaceBlock

type keyspaceBlock struct {
	HashValue string
	HashType  int
	Limit     int
	Start     int
	Started   bool
}

func HashCrack(hashType int, hash string, mask string, start int, limit int, unique string) {
	defer wg.Done()

	var app string

	if runtime.GOOS == "windows" {
		app = "./hashcat-3.5.0/hashcat"
	} else  {
		app = "hashcat"
	}

	arg0 := "-a"
	arg1 := "3"
	arg2 := "-m"
	arg3 := "-s"
	arg4 := "-l"
	arg5 := "--force"
	arg6 := "-D"
	arg7 := "1"
	arg8 := "--session"
	arg9 := "--potfile-disable"
	arg10:= "-o"
	outfile = selfnode.ID.String()+ ".txt"
	//additional randomity than node name with help of random function

	if runtime.GOOS == "windows" {
		HashcatJob = exec.Command(app, arg0, arg1, arg2, strconv.Itoa(hashType),
			arg3, strconv.Itoa(start), arg4, strconv.Itoa(limit), hash, mask, arg5, arg8, unique,arg9,arg10,outfile)
	} else  {
		HashcatJob = exec.Command(app, arg0, arg1, arg2, strconv.Itoa(hashType), arg3,
			strconv.Itoa(start), arg4, strconv.Itoa(limit), hash, mask, arg5, arg6, arg7, arg8,unique,arg9,arg10,outfile )
	}

	/*
	speed := getSpeed(hashType,mask)
	size:= limit-start
	time := speed/size
*/
	fmt.Println("DPC is running to crack the hash - \"",hash,"\" with keyspace range", start,"-",start+limit,". (Total keyspace is:",GetKeySpace(mask),")")
//	fmt.Println("\n\n\n This machine is cracking", speed,"million hashes/second. So, it would roughly take", time, "seconds for this cracking process")
//	fmt.Println("\n\n This estimate might vary basing on your other system activities and many other activities.")

	stdout, err := HashcatJob.Output()
	HashcatJob = nil

	// Spin off go routine for next HashcatJob if needed
	for _, entry := range hashcatQueue {
		if entry.Started == false {
			entry.Started = true
			//go HashCrack(hashType, hash, mask, entry.Start, limit, unique)
			fmt.Println("Starting another Hashcat for abandoned segment.")
			break
		}
	}

	if err != nil {
		//unblock cluster even if hashcat was not able to find password/ has some errors as we are already done with the process
		// cluster.UnBlock()
		switch err.Error() {
			//common errors
			case "exit status 1" :
				fmt.Println("Keyspace exhausted on this node :( I hope that some other node will find the password \n")
			case "exit status -2":
				fmt.Println("Issue with either GPU/ Temperature limit of Hashcat")
			//uncommon errors
			case "exit status 2":
				fmt.Println("Hashcat aborted with 'q' key during run (quit) !")
			case "exit status 3":
				fmt.Println("Hashcat aborted with 'c' key during run (checkpoint)!")
			case "exit status 4":
				fmt.Println("Hashcat aborted by runtime limit (--runtime)!")
			case "exit status -1":
				fmt.Println("Hashcat error (with arguments, inputs, inputfiles etc.)!")
			default :
				fmt.Println("Error running hashcat")
		}
		return
	}

	result := string(stdout)
	foundFlag := strings.Index(result, "Cracked")

	if foundFlag > 0 {
		// cluster.UnBlock() // no need of multicasting as it is done along with FOUND_PASS msg receive
		fmt.Print("\n\n I found the password for the hash! Please memorize it as I am deleting it from my side for security reasons\n ")
		afterPasswordFound(hashType, hash)
	} else {
		fmt.Println("Code can never reach here")
	}

	//workaround for blocking issue at optiontoStop
	fmt.Println("\n\n Thanks for using our DPC. If you have some time, please provide your rating to our project on a scale of 1-10")
	fmt.Println("\n\t\t Only if you want to, or else you can just ignore this!")
	time.Sleep(time.Millisecond * 200)
}

func afterPasswordFound(hashType int, hash string) {
    // cluster.UnBlock() // no need of multicasting as it is done along with FOUND_PASS msg receive
    	unBlockMsg := cluster.NewMessage(UNBLOCK_CLUSTER, selfnode.ID, []byte{})
	cluster.Send(unBlockMsg)
	//Displaying password
	outfileContents, err := ioutil.ReadFile(outfile)
	if err != nil {
		fmt.Print(err)
	}

	passwordsplit := strings.SplitN(string(outfileContents), ":", 2)
	foundPassword := passwordsplit[1]
	fmt.Print("\n\n\n")
	fmt.Println("\t\t    Password: ", passwordsplit[1])
	err = os.Remove(outfile)
	if err != nil {
		fmt.Print(err)
		return
	}

	//Notifying everyone (Multicast)
	nodes := cluster.GetListOfNodes()
	nodesCount := cluster.NumOfNodes()

	if nodesCount > 1 {
		fmt.Println("\n\t\t   Notifying everyone that I found the PASSWORD")
	} else {
		fmt.Println("\n \t\t No one else to notify")
	}

	payload := FoundMessage{HashType: hashType, HashValue: hash, Password: foundPassword, Origin: selfnode.ID.String()}
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
				msg 	:= cluster.NewMessage(FOUND_PASS, nodeIterate.ID, data)
				cluster.Send(msg)
			}
		}
	}

	if nodesCount > 1 {
		fmt.Println("\n\t\t   Notified everyone that I found the PASSWORD")
	}

	// Clean up job data from currentJob and jobList
	doneJob()
}
