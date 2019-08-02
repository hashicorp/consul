package api

import (
	"io/ioutil"
	"strings"
	"time"
)

type License struct {
	// The unique identifier of the license
	LicenseID string `json:"license_id" jsonapi:"primary,license"`

	// The customer ID associated with the license
	CustomerID string `json:"customer_id" jsonapi:"attr,customer_id"`

	// If set, an identifier that should be used to lock the license to a
	// particular site, cluster, etc.
	InstallationID string `json:"installation_id" jsonapi:"attr,installation_id"`

	// The time at which the license was issued
	IssueTime time.Time `json:"issue_time" jsonapi:"attr,issue_time,iso8601"`

	// The time at which the license starts being valid
	StartTime time.Time `json:"start_time" jsonapi:"attr,issue_time,iso8601"`

	// The time after which the license expires
	ExpirationTime time.Time `json:"expiration_time" jsonapi:"attr,expiration_time,iso8601"`

	// The time at which the license ceases to function and can
	// no longer be used in any capacity
	TerminationTime time.Time `json:"termination_time" jsonapi:"attr,termination_time,iso8601"`

	// The product the license is valid for
	Product string `json:"product" jsonapi:"attr,product"`

	// License Specific Flags
	Flags map[string]interface{} `json:"flags"`

	// List of features enabled by the license
	Features []string `json:"features"`
}

type LicenseReply struct {
	Valid    bool
	License  *License
	Warnings []string
}

func (op *Operator) LicenseGet(q *QueryOptions) (*LicenseReply, error) {
	var reply LicenseReply
	if _, err := op.c.query("/v1/operator/license", &reply, q); err != nil {
		return nil, err
	} else {
		return &reply, nil
	}
}

func (op *Operator) LicenseGetSigned(q *QueryOptions) (string, error) {
	r := op.c.newRequest("GET", "/v1/operator/license")
	r.params.Set("signed", "1")
	r.setQueryOptions(q)
	_, resp, err := requireOK(op.c.doRequest(r))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

func (op *Operator) LicensePut(license string, opts *WriteOptions) (*LicenseReply, error) {
	var reply LicenseReply
	r := op.c.newRequest("PUT", "/v1/operator/license")
	r.setWriteOptions(opts)
	r.body = strings.NewReader(license)
	_, resp, err := requireOK(op.c.doRequest(r))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := decodeBody(resp, &reply); err != nil {
		return nil, err
	}

	return &reply, nil
}
