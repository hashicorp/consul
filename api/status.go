package api

// Status can be used to query the Status endpoints
type Status struct {
	c *Client
}

// Status returns a handle to the status endpoints
func (c *Client) Status() *Status {
	return &Status{c}
}

// Leader is used to query for a known leader
func (s *Status) Leader(q *QueryOptions) (string, error) {
	r := s.c.newRequest("GET", "/v1/status/leader")
	r.setQueryOptions(q)
	_, resp, err := requireOK(s.c.doRequest(r))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var leader string
	if err := decodeBody(resp, &leader); err != nil {
		return "", err
	}
	return leader, nil
}

// Peers is used to query for a known raft peers
func (s *Status) Peers(q *QueryOptions) ([]string, error) {
	r := s.c.newRequest("GET", "/v1/status/peers")
	r.setQueryOptions(q)
	_, resp, err := requireOK(s.c.doRequest(r))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var peers []string
	if err := decodeBody(resp, &peers); err != nil {
		return nil, err
	}
	return peers, nil
}
