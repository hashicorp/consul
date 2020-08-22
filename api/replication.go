package api

import "time"

type ReplicationInfo struct {
	Type         string
	Enabled      bool
	Running      bool
	Status       string
	Index        uint64     `json:",omitempty"`
	LastStatusAt *time.Time `json:",omitempty"`
	LastError    string     `json:",omitempty"`
}

type ReplicationInfoList struct {
	PrimaryDatacenter string
	Info              []ReplicationInfo
}
