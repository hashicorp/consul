package core

import "log"

// Restart restarts CoreDNS forcefully using newCorefile,
// or, if nil, the current/existing Corefile is reused.
func Restart(newCorefile Input) error {
	log.Println("[INFO] Restarting")

	if newCorefile == nil {
		corefileMu.Lock()
		newCorefile = corefile
		corefileMu.Unlock()
	}

	wg.Add(1) // barrier so Wait() doesn't unblock

	err := Stop()
	if err != nil {
		return err
	}

	err = Start(newCorefile)
	if err != nil {
		return err
	}

	wg.Done() // take down our barrier

	return nil
}
