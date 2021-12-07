//go:build !consulent
// +build !consulent

package authmethod

import (
	"sync"

	"github.com/hashicorp/consul/agent/structs"
)

type syncCache struct {
	lock  sync.RWMutex
	cache authMethodCache
}

func NewCache() Cache {
	c := &syncCache{}
	c.cache.init()
	return c
}

func (c *syncCache) GetValidator(method *structs.ACLAuthMethod) (uint64, Validator, bool) {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.cache.GetValidator(method)
}

func (c *syncCache) PutValidatorIfNewer(method *structs.ACLAuthMethod, validator Validator, idx uint64) Validator {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.cache.PutValidatorIfNewer(method, validator, idx)
}

func (c *syncCache) Purge() {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.cache.Purge()
}
