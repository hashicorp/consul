package consul

import "fmt"

func checkNumPeers(s *Server, count int) func() error {
	return func() error {
		n, _ := s.numPeers()
		return checkNum("peers", n, count)
	}
}

func checkLANMembers(s *Server, count int) func() error {
	return func() error {
		return checkNum("LAN members", len(s.LANMembers()), count)
	}
}

func checkWANMembers(s *Server, count int) func() error {
	return func() error {
		return checkNum("WAN members", len(s.WANMembers()), count)
	}
}

func checkNum(name string, got, want int) error {
	if got != want {
		return fmt.Errorf("got %d %s want %d", got, name, want)
	}
	return nil
}
