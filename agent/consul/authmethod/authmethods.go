package authmethod

import (
	"fmt"
	"sort"
	"sync"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/mitchellh/mapstructure"
)

type Cache interface {
	// GetValidator retrieves the Validator from the cache.
	// It returns the modify index of struct that the validator was created from,
	// the validator and a boolean indicating whether the value was found
	GetValidator(method *structs.ACLAuthMethod) (uint64, Validator, bool)

	// PutValidatorIfNewer inserts a new validator into the cache if the index is greater
	// than the modify index of any existing entry in the cache. This method will return
	// the newest validator which may or may not be the one from the method parameter
	PutValidatorIfNewer(method *structs.ACLAuthMethod, validator Validator, idx uint64) Validator

	// Purge removes all cached validators
	Purge()
}

type ValidatorFactory func(method *structs.ACLAuthMethod) (Validator, error)

type Validator interface {
	// Name returns the name of the auth method backing this validator.
	Name() string

	// ValidateLogin takes raw user-provided auth method metadata and ensures
	// it is sane, provably correct, and currently valid. Relevant identifying
	// data is extracted and returned for immediate use by the role binding
	// process.
	//
	// Depending upon the method, it may make sense to use these calls to
	// continue to extend the life of the underlying token.
	//
	// Returns auth method specific metadata suitable for the Role Binding
	// process as well as the desired enterprise meta for the token to be
	// created.
	ValidateLogin(loginToken string) (map[string]string, *structs.EnterpriseMeta, error)

	// AvailableFields returns a slice of all fields that are returned as a
	// result of ValidateLogin. These are valid fields for use in any
	// BindingRule tied to this auth method.
	AvailableFields() []string

	// MakeFieldMapSelectable converts a field map as returned by ValidateLogin
	// into a structure suitable for selection with a binding rule.
	MakeFieldMapSelectable(fieldMap map[string]string) interface{}
}

var (
	typesMu sync.RWMutex
	types   = make(map[string]ValidatorFactory)
)

// Register makes an auth method with the given type available for use. If
// Register is called twice with the same name or if validator is nil, it
// panics.
func Register(name string, factory ValidatorFactory) {
	typesMu.Lock()
	defer typesMu.Unlock()
	if factory == nil {
		panic("authmethod: Register factory is nil for type " + name)
	}
	if _, dup := types[name]; dup {
		panic("authmethod: Register called twice for type " + name)
	}
	types[name] = factory
}

func IsRegisteredType(typeName string) bool {
	typesMu.RLock()
	_, ok := types[typeName]
	typesMu.RUnlock()
	return ok
}

type authMethodValidatorEntry struct {
	Validator   Validator
	ModifyIndex uint64
}

// authMethodCache is an non-thread-safe cache that maps ACLAuthMethods to their Validators
type authMethodCache struct {
	entries map[string]*authMethodValidatorEntry
}

func newCache() Cache {
	c := &authMethodCache{}
	c.init()
	return c
}

func (c *authMethodCache) init() {
	c.Purge()
}

func (c *authMethodCache) GetValidator(method *structs.ACLAuthMethod) (uint64, Validator, bool) {
	entry, ok := c.entries[method.Name]
	if ok {
		return entry.ModifyIndex, entry.Validator, true
	}

	return 0, nil, false
}

func (c *authMethodCache) PutValidatorIfNewer(method *structs.ACLAuthMethod, validator Validator, idx uint64) Validator {
	prev, ok := c.entries[method.Name]
	if ok {
		if prev.ModifyIndex >= idx {
			return prev.Validator
		}
	}

	c.entries[method.Name] = &authMethodValidatorEntry{
		Validator:   validator,
		ModifyIndex: idx,
	}
	return validator
}

func (c *authMethodCache) Purge() {
	c.entries = make(map[string]*authMethodValidatorEntry)
}

// NewValidator instantiates a new Validator for the given auth method
// configuration. If no auth method is registered with the provided type an
// error is returned.
func NewValidator(method *structs.ACLAuthMethod) (Validator, error) {
	typesMu.RLock()
	factory, ok := types[method.Type]
	typesMu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("no auth method registered with type: %s", method.Type)
	}

	return factory(method)
}

// Types returns a sorted list of the names of the registered types.
func Types() []string {
	typesMu.RLock()
	defer typesMu.RUnlock()
	var list []string
	for name := range types {
		list = append(list, name)
	}
	sort.Strings(list)
	return list
}

// ParseConfig parses the config block for a auth method.
func ParseConfig(rawConfig map[string]interface{}, out interface{}) error {
	decodeConf := &mapstructure.DecoderConfig{
		Result:           out,
		WeaklyTypedInput: true,
		ErrorUnused:      true,
	}

	decoder, err := mapstructure.NewDecoder(decodeConf)
	if err != nil {
		return err
	}

	if err := decoder.Decode(rawConfig); err != nil {
		return fmt.Errorf("error decoding config: %s", err)
	}

	return nil
}
