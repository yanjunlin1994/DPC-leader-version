package main

import (
	"fmt"
	"time"
	"os"
	"strings"
	"os/signal"
)

func main() {
    exitfix := make(chan os.Signal, 1)
	signal.Notify(exitfix, os.Interrupt)
	go func(){
		for _ = range exitfix {
			fmt.Println("CTRL+C Interrupt")
			close(exitfix)
			os.Exit(0)
		}
	} ()
    userChoice := SetUpOrJoinEntrance()
    ClusterManagementEntrance(userChoice)
    myRole := AskForWhatToDo()
    WhatToDoEntrance(myRole)
    select{}
    //WAHT IS NEXT STEP? MAYBE STATUS
}
func SetUpOrJoinEntrance() int{
    userChoice := -1
    for (userChoice != 1) && (userChoice != 2) {
        userChoice = SetUpOrJoin()
        if (userChoice != 1) && (userChoice != 2) {
            fmt.Println("wrong input, try again")
        }
    }
    return userChoice
}
func SetUpOrJoin() int {
    var userChoice int
    fmt.Println("Welcome to Distributed Password Cracker")
	fmt.Println("What do you want to do today?")
	fmt.Println("1. Setup a cluster")
	fmt.Println("2. Join a cluster")
	fmt.Scanf("%d\n", &userChoice)
    return userChoice
}
func ClusterManagementEntrance(userChoice int) {
    if userChoice == 1 {
		ClusterManagement(userChoice)
		time.Sleep(time.Millisecond * 300)
		fmt.Println("Cluster setup done")
	} else if userChoice == 2 {
		ClusterManagement(userChoice)
		time.Sleep(time.Millisecond * 300)
		fmt.Println("Join cluster done")
	} else {
		fmt.Println("DANGEROUS")
	}
}
func AskForWhatToDo() string{
    role := ""
    for (role != "crack") && (role != "help") {
        fmt.Println("Do you want crack a hash (type 'crack') or just help other
                    nodes in cracking their hash (type 'help')?")
    	fmt.Scanf("%s\n", &role)
        if (role != "crack") && (role != "help") {
            fmt.Println("wrong input, try again")
        }
    }
    return role
}
func WhatToDoEntrance(myrole string) {
    if myrole == "crack" {
        crackJobEntrance()
    } else if myrole == "help" {
        fmt.Println("Please keep waiting and you will be assigned a task")
    } else {
        fmt.Println("DANGEROUS")
    }
}
func crackJobEntrance() {
    allowed := proposeMyCrackJob()
    if (allowed) {
        fmt.Println("You can crack")
        details := getHashDetails()
        detailArray := strings.Split(details, ",")
        fmt.Println("Hash details" + detailArray[0] + "  "+ detailArray[1] + "  " + detailArray[2])
        leaderComm.SendCrackJobDetailsToLeader(detailArray[0], detailArray[1], detailArray[2])
    } else {
        fmt.Println("You can't crack, someone has already initiated a job")
    }
}
func proposeMyCrackJob() bool{
    allowed := proposeNewCrackingJob()
    return allowed
}
func proposeNewCrackingJob() bool {
    return leaderComm.ProposeNewJob()
}

func getHashDetails() string{
    var hashType, hash string
    var pwdlength int
    for () {
        fmt.Println("1. What type of hash do you want to crack (MD5/SHA1/SHA256)?")
    	fmt.Scanf("%s\n", &hashType)
        if (strings.EqualFold(hashType, "MD5") || strings.EqualFold(hashType, "SHA1") || strings.EqualFold(hashType, "SHA256") ) {
    		fmt.Println("2. Please give me the hash")
    		fmt.Scanf("%s\n", &hash)
    		if (strings.EqualFold(hashType, "MD5")) {
    			if 32 != len(hash) {
                    fmt.Println("MD5 hash should be of 32 characters and not any less or more. Please try again!")
                    continue
    			}
    		} else if (strings.EqualFold(hashType, "SHA1")) {
    			if 40 == len(hash) {
                    fmt.Println("SHA1 hash should be of 40 characters and not any less or more. Please try again!")
    				continue
                }
    		} else if (strings.EqualFold(hashType, "SHA256")) {
    			if 64 == len(hash) {
                    fmt.Println("SHA256 hash should be of 64 characters and not any less or more. Please try again!")
    				continue
                }
    		}
    		fmt.Println("3. How many characters do you think is the password? \n")
    		fmt.Scanf("%d\n", &pwdlength)
            if pwdlength < 1 {
                fmt.Println("wrong input, try again")
                continue
            }
            break
    	} else {
    		fmt.Println("wrong input, try again")
            continue
    	}
    }
    details := hashType + "," + hash + "," + strconv.Itoa(pwdlength)
    return details
}
