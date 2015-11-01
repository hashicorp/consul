package audit

import (
"encoding/json"
"os"
"sync"
"fmt"
)

const (
	Complete = "complete"
	Failed = "failed"
)

//AuditItem is used for storing to output file
type AuditItem struct{

	//Name of the event
	Name string `json:name`

	// Node returns name of the node.
	Node string `json:node`

	// Datacetner returns name of datacenter
	Datacenter string `json:datacenter`

	// Time is execution of event
	Time string `json:time`

	// API returns 
	API  string `json:api`

	// Provide ACLToken if exist
	ACLToken string `json:acl_token`

	// IP provides IP-address of machine where
	// item was obtained 
	IP    string `json:ip`

	// Status can be Complete or Failed. In the case
	// if Status is failed, Errof ield is not empty. 
	Status   string `json:status`

	// Error returns error information of Status if failed.
	Error string `json:error`

	// Addition is optional field for providing
	// some information.
	Addition string `json:addition`
}



type Audit struct {

	//Outpath for auditting. 
	Outpath string
	//Enable returns true is audit is enable and false otherwise
	Enable bool
	//Descriptor of the file
	f *os.File
	mutex *sync.RWMutex
}

//NewAudit creates a new audit 
func NewAudit(outpath string)*Audit {
	audit := new(Audit)
	audit.Outpath = outpath
	if outpath != "" {
		audit.Enable = true
	}
	audit.mutex = &sync.RWMutex{}
	return audit
}

//Write information to the file
//If audit is disabled, do nothing.
// Outpuas in json format
func (audit *Audit) Write(item *AuditItem) error {
	if !audit.Enable {
		return fmt.Errorf("Audit is not enable")
	}

	audit.mutex.Lock()
	defer audit.mutex.Unlock()
	err := audit.open()
	if err != nil {
		return err
	}

	enc := json.NewEncoder(audit.f)
	return enc.Encode(item)
}

//open file for create or append information
func (audit *Audit) open() error{
	var err error
	audit.f, err = os.OpenFile(audit.Outpath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return err
	}

	return nil
}
