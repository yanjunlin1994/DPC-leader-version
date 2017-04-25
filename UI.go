package main

import (
	"fmt"
	"time"
	"os"
	"strings"
	"os/signal"
)

var hashType string
var hash string
var pwdlength int
var role string
var count1 int = 0
var count2 int = 0
var count3 int = 0
var count4 int = 0

func main() {
	var userChoice int

	fmt.Println("\n\n\n\n\t\t Welcome to Distributed Password Cracker")
	fmt.Println("\n\t\t     What do you want to do today?")
	fmt.Println("\n\t\t     1. Setup a cluster")
	fmt.Println("\n\t\t     2. Join a cluster")
	// TODO: option for viewing status
	fmt.Scanf("%d\n", &userChoice)
	count1++

	//buggy: needs fix (sleep?)(ctrl+c when in UI)
	exitfix := make(chan os.Signal, 1)
	signal.Notify(exitfix, os.Interrupt)
	go func(){
		for _ = range exitfix {
			fmt.Println("\n\t\t Uh Oh...Interrupt received! Exiting program!\n")
			close(exitfix)
			os.Exit(0)
		}
	} ()

	if userChoice == 1 {
		ClusterManagement(userChoice)
		// wait for cluster to setup before taking user i/p
		time.Sleep(time.Millisecond * 300)
		fmt.Println("\n\t\t     Cluster setup done!")
		nodesJob()
		// to keep goroutine running
		select {}
	} else if userChoice == 2 {
		ClusterManagement(userChoice)
		time.Sleep(time.Millisecond * 300)
		fmt.Println("\n\t\t     Joining cluster done!")
		nodesJob()
		select {}
	} else {
		fmt.Println("\n\t  Hmm...I did not provide you that option. Please try again! (",3-count1,") attempts left")
		if count1 < 3 {
			main()
		} else {
			fmt.Println("\n\t\t	   You entered wrong value 3 times. Exiting!")
			os.Exit(0);
		}
	}
}

//I was not able to come up with a better name than this :(
func nodesJob() {
	count2++
	fmt.Println("\n Do you want crack a hash (type 'crack') or just help other nodes in cracking their hash (type 'help')?")
	fmt.Scanf("%s\n", &role)

	if role == "crack" {
        allowed := proposeNewCrackingJob()
        if (allowed) {
            fmt.Println("you can crack")
        } else {
            fmt.Println("you can't crack")
        }
		getHashDetails()
        leaderComm.SendCrackJobDetailsToLeader(hashType, hash, pwdlength)
		// initiateCracking(hashType, hash, pwdlength)
	} else if role == "help" {
		fmt.Println("\n   Please keep waiting and you will be assigned a task when someone else within this distributed network initiates a hash cracking task")
		fmt.Println("\n\t    Listening for any jobs from nodes in the network......")
	} else {
		fmt.Println("\n\t\t    Hmm...I did not provide you that option. Please try again!(",3-count1,")attempts left")
		if count2 < 3 {
			nodesJob()
		} else {
			fmt.Println("\n\t\t	  You entered wrong value 3 times. Exiting!")
			os.Exit(0);
		}
	}
}
func proposeNewCrackingJob() bool{
    return leaderComm.ProposeNewJob()
}
func getHashDetails() {
	fmt.Println("\n Sure but please provide me some details of the hash before I start the cracking process")
	fmt.Println("\n\t\t 1. What type of hash do you want to crack (MD5/SHA1/SHA256)?")
	fmt.Scanf("%s\n", &hashType)
	count3++

	if ( strings.EqualFold(hashType, "MD5") || strings.EqualFold(hashType, "SHA1") || strings.EqualFold(hashType, "SHA256") ) {
		fmt.Println("\n\t\t 2. Please give me the hash")
		fmt.Scanf("%s\n", &hash)
		if (strings.EqualFold(hashType, "MD5")) {
			if 32 == len(hash) {
			} else {
				fmt.Println("MD5 hash should be of 32 characters and not any less or more. Please try again!")
				getHashDetails()
			}
		} else if (strings.EqualFold(hashType, "SHA1")) {
			if 40 == len(hash) {
			} else {
				fmt.Println("SHA1 hash should be of 40 characters and not any less or more. Please try again!")
				getHashDetails()
			}
		} else if (strings.EqualFold(hashType, "SHA256")) {
			if 64 == len(hash) {
			} else {
				fmt.Println("SHA256 hash should be of 64 characters and not any less or more. Please try again!")
				getHashDetails()
			}
		}
		fmt.Println("\n\t\t 3. How many characters do you think is the password? \n")
		fmt.Scanf("%d\n", &pwdlength)
	} else {
		fmt.Println("\n\t\t Hmm...I only support MD5/SHA1/SHA256 as of now! Please select one among them (",3-count1,")attempts left")
		if count3 < 3 {
			getHashDetails()
		} else {
			fmt.Println("\n\t\t	   You entered wrong value 3 times. Exiting!")
			os.Exit(0);
		}

	}

	//to be enabled before demo
	// if pwdlength < 8 {
	// 	fmt.Println("Well, password less than 8 characters does not really need a distributed hascat and Prof. Nace will not be happy with the demo then. So, please choose a different one")
	// }
}

//If new task has to be stated after the current process is done
func repeatJob(option int) {
	var repeat string
	var repeatRole string

	time.Sleep(time.Millisecond * 400)

	if option == 2 && count4==3{
		fmt.Println("\n\n Thank you!")
	}

	fmt.Println("\n\n\t\t Do you want to repeat the cracking/helping process (type 'yes') or do you want to quit? ( type 'no')")
	fmt.Scanf("%s\n", &repeat)
	count4++

	if repeat == "yes" {
		fmt.Println("\n\t\t\t Cracking (type 'crack') or Helping (type 'help') ?")
		fmt.Scanf("%s\n", &repeatRole)
		if repeatRole == "crack" {
			getHashDetails()
			initiateCracking(hashType, hash, pwdlength)
		} else if repeatRole == "help" {
			fmt.Println("\n   Please keep waiting and you will be assigned a task when someone else within this distributed network initiates a hash cracking task")
			fmt.Println("\n\t    Listening for any jobs from nodes in the network......")
		} else {
			fmt.Println("\n\t I did not provide you that option. Defaulting to 'help' mode")
		}
	} else if repeat == "no" {
		fmt.Println("\n\n\t   Thank you for using our DPC! Have a good one. Good Bye!\n\n\n")
		os.Exit(0)
	} else {
		fmt.Println("\n\t\t    Hmm...I did not provide you that option. Please try again!(",3-count4,")attempts left")
		if count2 < 3 {
			repeatJob(option)
		} else {
			fmt.Println("\n\t\t	  You entered wrong value 3 times. Exiting!")
			os.Exit(0);
		}
	}
}

//TODO: to be fixed without mandatory input
func optiontoStop() {
	var toKill string
	fmt.Println(" \n\n\t  At any point if you want to kill the running process, type 'quit' \n\n ")
	fmt.Scanf("%s\n", &toKill)
	if toKill == "quit" {
		stopJob()
	} else {
		//Ignore
	}
}
