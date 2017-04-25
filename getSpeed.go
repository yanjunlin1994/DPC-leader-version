package main

import (
	"runtime"
	"os/exec"
	"fmt"
	//"strings"
	"strconv"
)

// to be also used for capability based division

var speedtestJob *exec.Cmd

func getSpeed(hashType int, mask string) int {
	var app string
	var arg3 string

	if runtime.GOOS == "windows" {
		app = "./hashcat-3.5.0/hashcat"
	} else  {
		app = "hashcat"
	}

	arg0 := "-a"
	arg1 := "3"
	arg2 := "--speed-only"

	if hashType == 0 {
		arg3 = "5d41402abc4b2a76b9719d911017c592"
	} else if hashType == 100 {
		arg3 = "aaf4c61ddcc5e8a2dabede0f3b482cd9aea9434d"
	} else if hashType == 1400 {
		arg3 = "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	}

	arg5 := "--force"
	arg6 := "-D"
	arg7 := "1"
	arg8 := "--session"

	if runtime.GOOS == "windows" {
		speedtestJob = exec.Command(app, arg0, arg1, arg2, arg3, mask, arg5, arg8, selfnode.ID.String())
	} else  {
		speedtestJob = exec.Command(app, arg0, arg1, arg2, arg3, mask, arg5, arg6, arg7, arg8, selfnode.ID.String())
	}

	stdout, err := speedtestJob.Output()

	if err!=nil {
		fmt.Println("Issue running hashcat in speedtest")
	}

	results := string(stdout)
	tempspeed := results[294:297]
	speed,err  := strconv.Atoi(tempspeed)
	if err != nil {
		fmt.Println(err.Error())
	}

	fmt.Println("Speed is", speed)
	return speed
}
