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
    ANOTHER_PIECE = 70
    I_STOP = 71
)
const (
    INIT_BACKUP = 90
)
// Struct for new job message
type NewJobMessage struct {
	HashType  int
	HashValue string
	Length    int
	Start     int
	Limit     int
	Origin    string
	Version	  int
}

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

// Struct for found password message
type FoundMessage struct {
	HashValue string
	HashType  int
	Password  string
}

// Struct for block cluster message
// type BlockClusterMessage struct {
// 	Origin string
// }
//
// type unBlockClusterMessage struct {
// 	Origin string
// }
type CrackJobDetailsMessage struct {
    HashType string
    Hash string
    Pwdlength int
}
type InitializeBackUpMessage struct {
    chosenProposerID   string
    BackUps            string
    JobMap             map[string]string
}
