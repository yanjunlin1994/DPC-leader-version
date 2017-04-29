package main
/* job class */
import (
	"errors"
	"fmt"
    "io/ioutil"
    "strconv"
    "runtime"
    "os"
	"os/exec"
    "strings"
    "gopkg.in/mgo.v2/bson"
)

const (
	Inactive = -1
    Active = 0
    Found = 1
    WaitForNew = 2
)


// job for each hashcat instance
type Job struct {
    self        *wendy.Node
    cluster     *wendy.Cluster
    leaderComm  *LeaderComm
	HashType    int // Type of hash (int)
	HashValue   string // Hash value
	Length      int
	Start       int
	Limit       int
    Status      int
    Outfile     string
    HashcatJob  *exec.Cmd
}
// String returns a string representation of a message.
func (j *Job) String() string {
	return "[JOB]" + strconv.Itoa(j.Start) + " += " + strconv.Itoa(j.Limit)
}

func (j *Job) NewJob(self *wendy.Node, cluster *wendy.Cluster,
                    leaderComm *LeaderComm,
                    Hasht int, Hashv string, Len int, St int, Lm int) *Job {
	return &Job{
        self:          self,
        cluster:       cluster,
        HashType:      Hasht,
    	HashValue:     Hashv,
    	Length:        Len,
    	Start:         St,
    	Limit:         Lm,
        Status:        Inactive,
        Outfile:       self.ID.String()+ ".txt",
        HashcatJob:    nil,

	}
}
//spin a new process to deal with it
func (j *Job) StartJob() {
    j.SetStatus(Active)
    result := j.startCrack()
    if (result == 1) {
        j.SetStatus(Found)
        foundPassword := j.getFoundedPassword()
        j.announceFound(foundPassword)
    } else {
        j.SetStatus(WaitForNew)
        j.askLeaderANewPiece()
    }
}
func (j *Job) SetStatus(s int) {
    fmt.Println("[JOB] SetStatus" + strconv.Itoa(s))
    j.Status = s
}
func (j *Job) startCrack() int{
    app := "./hashcat-3.5.0/hashcat"
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
    mask := ""
    for i := 0; i < j.Length; i++ {
		mask = mask + "?l"

    unique := self.ID.String()

    if runtime.GOOS == "windows" {
		j.HashcatJob = exec.Command(app, arg0, arg1, arg2, strconv.Itoa(j.HashType),
			arg3, strconv.Itoa(j.Start), arg4, strconv.Itoa(j.Limit), j.HashValue,
            mask, arg5, arg8, unique,arg9,arg10, j.Outfile)
	} else  {
		j.HashcatJob = exec.Command(app, arg0, arg1, arg2, strconv.Itoa(j.HashType),
            arg3, strconv.Itoa(j.Start), arg4, strconv.Itoa(j.Limit), j.HashValue,
            mask, arg5, arg6, arg7, arg8,unique,arg9,arg10,j.Outfile )
	}
    fmt.Println("[JOB] Running hashcat. Key space is " + strconv.Itoa(j.Start)
                + " to " + strconv.Itoa(j.Start + j.Limit))

    stdout, err := HashcatJob.Output()
	HashcatJob = nil

    if err != nil {
		switch err.Error() {
			//common errors
			case "exit status 1" :
				fmt.Println("[JOB] Keyspace exhausted on this node")
			case "exit status -2":
				fmt.Println("[JOB] Issue with either GPU/ Temperature limit of Hashcat")
			//uncommon errors
			case "exit status 2":
				fmt.Println("[JOB] Hashcat aborted with 'q' key during run (quit) !")
			case "exit status 3":
				fmt.Println("[JOB] Hashcat aborted with 'c' key during run (checkpoint)!")
			case "exit status 4":
				fmt.Println("[JOB] Hashcat aborted by runtime limit (--runtime)!")
			case "exit status -1":
				fmt.Println("[JOB] Hashcat error (with arguments, inputs, inputfiles etc.)!")
			default :
				fmt.Println("[JOB] Error running hashcat")
		}
		return -1
	}
    result := string(stdout)
	foundFlag := strings.Index(result, "Cracked")

	if foundFlag > 0 {
		fmt.Println("[JOB] I found the password for the hash! Please memorize
                 it as I am deleting it from my side for security reasons")
		return 1
	} else {
		fmt.Println("[JOB] Code can never reach here. DANGEROUS!")
	}
}
func (j *Job) getFoundedPassword() string{
    outfileContents, err := ioutil.ReadFile(j.Outfile)
    if err != nil {
        fmt.Print(err)
    }
    passwordsplit := strings.SplitN(string(outfileContents), ":", 2)
    foundPassword := passwordsplit[1]
    fmt.Println("[JOB] Found password:" + passwordsplit[1])
    err = os.Remove(j.Outfile)
    if err != nil {
        fmt.Print(err)
        return ""
    }
    return foundPassword
}
func (j *Job) announceFound(foundPassword string) {
	nodes := j.cluster.GetListOfNodes()
	nodesCount := len(nodes)
    if nodesCount < 1 {
        fmt.Println("[JOB] No one to notify")
        return
    }
    fmt.Println("[JOB] Notifying everyone that I have found the password")

	payload := FoundMessage{HashType: hashType, HashValue: hash, Password: foundPassword, Origin: j.self.ID.String()}
	data, err := bson.Marshal(payload)
	if err != nil {
		fmt.Println(err)
		return
	}
    for _, nodeIterate := range nodes {
        if !nodeIterate.ID.Equals(self.ID) {
            msg := j.cluster.NewMessage(FOUND_PASS, nodeIterate.ID, data)
            j.cluster.Send(msg) //no need to check error
        }
    }
}
func (j *Job) askLeaderANewPiece() {
    err = os.Remove(j.Outfile)
    if err != nil {
        fmt.Print(err)
    }
    fmt.Println("[JOB] Ask leader for a new piece")
    l.leaderComm.AskLeaderANewPiece()
}
func (j *Job) Stop() {
    err := j.killHashCat()
    if err != nil {
        fmt.Print(err)
    }
    j.SetStatus(Inactive)
    j.notifyLeaderStop()
}
func (j *Job) killHashCat() error {
    fmt.Println("[JOB] killHashCat")
    if (j.Status != 0) || (j.HashcatJob == nil){
        fmt.Println("[JOB] no hashcat running")
        return errors.New("ERROR: no hashcat running, can't kill")
    }
    err := j.HashcatJob.Process.Kill()
    if err != nil {
        fmt.Println("[JOB] Failed to kill hashcat on stop message?")
        panic(err)
    }
    j.HashcatJob = nil
}
func (j *Job) notifyLeaderStop() {
    l.leaderComm.NotifyLeaderIStop()
}
