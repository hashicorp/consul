package agent

import (
	"net/http"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/proto/pbcommon"
	"github.com/hashicorp/consul/proto/pbreplication"
)

func timePointer(t time.Time) *time.Time {
	return &t
}

func (s *HTTPServer) ReplicationInfoList(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	var args pbcommon.DCSpecificRequest
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	var out pbreplication.InfoList

	if err := s.agent.RPC("Replication.List", &args, &out); err != nil {
		return nil, err
	}

	reply := api.ReplicationInfoList{
		PrimaryDatacenter: out.PrimaryDatacenter,
		Info:              make([]api.ReplicationInfo, 0, len(out.Info)),
	}

	for _, info := range out.Info {
		apiInfo := api.ReplicationInfo{
			Type:      info.Type.String(),
			Enabled:   info.Enabled,
			Running:   info.Running,
			Status:    info.Status.String(),
			Index:     info.Index,
			LastError: info.LastError,
		}

		if info.LastStatusAt.GetSeconds() > 0 || info.LastStatusAt.GetNanos() > 0 {
			apiInfo.LastStatusAt = timePointer(time.Unix(info.LastStatusAt.GetSeconds(), int64(info.LastStatusAt.GetNanos())))
		}

		reply.Info = append(reply.Info, apiInfo)
	}

	return reply, nil
}
