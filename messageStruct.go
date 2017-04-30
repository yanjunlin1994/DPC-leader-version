package main
// import (
//     "./wendy-modified"
// )
// Message numbers
type MessageType byte

const (
	NEW_JOB = 20
	STOP_JOB = 21
	FOUND_PASS = 22
	START_JOB = 23
)
const (
    BLOCK_CLUSTER = 30
	UNBLOCK_CLUSTER = 31
)
const (
    LEADER_VIC = 60
    YOU_ARE_LEADER = 61
    WANT_TO_CRACK = 62
    LEADER_JUDGE = 63
    CRACK_DETAIL = 64
)
const (
    FIRST_JOB = 70
    ASK_ANOTHER_PIECE = 71
    I_STOP = 72
)
const (
    INIT_BACKUP = 90
)
// Struct for new job message


// Struct for start latest job message
type StartJobMessage struct {
	HashType int
	HashValue string
	Origin string
}

// Struct for stop job message
type StopJobMessage struct {
	HashValue string
	HashType  int
	Origin string
}







type NewJobMessage struct {
	HashType      int
	HashValue     string
	Pwdlength     int
	Start         int
	End         int
}
// Struct for found password message
type FoundMessage struct {
	HashValue string
	HashType  int
	Password  string
}

type CrackJobDetailsMessage struct {
    HashType string
    Hash string
    Pwdlength int
}
type InitializeBackUpMessage struct {
    ChosenProposerID   string       `json:"cid,omitempty"`
    BackUps            string       `json:"bu,omitempty"`
    JobMap             []JobEntry  `json:"jm,omitempty"`
}


// Struct for block cluster message
// type BlockClusterMessage struct {
// 	Origin string
// }
//
// type unBlockClusterMessage struct {
// 	Origin string
// }
