package consul

import (
	"fmt"

	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
)

type caBackend struct {
	*Server
}

func (c *caBackend) ServersSupportMultiDCConnectCA() (bool, bool) {
	return ServersInDCMeetMinimumVersion(c.Server, c.Server.config.PrimaryDatacenter, minMultiDCConnectVersion)
}

func (c *caBackend) State() *state.Store {
	return c.Server.fsm.State()
}

func (c *caBackend) ApplyCARequest(req *structs.CARequest) (interface{}, error) {
	return c.Server.raftApplyMsgpack(structs.ConnectCARequestType, req)
}

func (c *caBackend) ApplyCALeafRequest() (uint64, error) {
	// TODO(banks): when we implement IssuedCerts table we can use the insert to
	// that as the raft index to return in response.
	//
	// UPDATE(mkeeler): The original implementation relied on updating the CAConfig
	// and using its index as the ModifyIndex for certs. This was buggy. The long
	// term goal is still to insert some metadata into raft about the certificates
	// and use that raft index for the ModifyIndex. This is a partial step in that
	// direction except that we only are setting an index and not storing the
	// metadata.
	req := structs.CALeafRequest{
		Op:         structs.CALeafOpIncrementIndex,
		Datacenter: c.Server.config.Datacenter,
	}
	resp, err := c.Server.raftApplyMsgpack(structs.ConnectCALeafRequestType|structs.IgnoreUnknownTypeFlag, &req)
	if err != nil {
		return 0, err
	}

	modIdx, ok := resp.(uint64)
	if !ok {
		return 0, fmt.Errorf("Invalid response from updating the leaf cert index")
	}
	return modIdx, err
}

func (c *caBackend) PrimaryRoots(args structs.DCSpecificRequest, roots *structs.IndexedCARoots) error {
	return c.Server.forwardDC("ConnectCA.Roots", c.Server.config.PrimaryDatacenter, &args, roots)
}

func (c *caBackend) SignIntermediate(csr string) (string, error) {
	req := &structs.CASignRequest{
		Datacenter:   c.Server.config.PrimaryDatacenter,
		CSR:          csr,
		WriteRequest: structs.WriteRequest{Token: c.Server.tokens.ReplicationToken()},
	}
	var pem string
	err := c.Server.forwardDC("ConnectCA.SignIntermediate", c.Server.config.PrimaryDatacenter, req, &pem)
	return pem, err
}
