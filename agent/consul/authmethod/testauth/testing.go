package testauth

import (
	"fmt"
	"sync"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/authmethod"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/go-uuid"
)

func init() {
	authmethod.Register("testing", newValidator)
}

var (
	tokenDatabaseMu sync.Mutex
	tokenDatabase   map[string]map[string]map[string]string // session => token => fieldmap
)

func StartSession() string {
	sessionID, err := uuid.GenerateUUID()
	if err != nil {
		panic(err)
	}
	return sessionID
}

func ResetSession(sessionID string) {
	tokenDatabaseMu.Lock()
	defer tokenDatabaseMu.Unlock()
	if tokenDatabase != nil {
		delete(tokenDatabase, sessionID)
	}
}

func InstallSessionToken(sessionID string, token string, namespace, name, uid string) {
	fields := map[string]string{
		serviceAccountNamespaceField: namespace,
		serviceAccountNameField:      name,
		serviceAccountUIDField:       uid,
	}

	tokenDatabaseMu.Lock()
	defer tokenDatabaseMu.Unlock()
	if tokenDatabase == nil {
		tokenDatabase = make(map[string]map[string]map[string]string)
	}
	sdb, ok := tokenDatabase[sessionID]
	if !ok {
		sdb = make(map[string]map[string]string)
		tokenDatabase[sessionID] = sdb
	}
	sdb[token] = fields
}

func GetSessionToken(sessionID string, token string) (map[string]string, bool) {
	tokenDatabaseMu.Lock()
	defer tokenDatabaseMu.Unlock()
	if tokenDatabase == nil {
		return nil, false
	}
	sdb, ok := tokenDatabase[sessionID]
	if !ok {
		return nil, false
	}
	fields, ok := sdb[token]
	if !ok {
		return nil, false
	}

	fmCopy := make(map[string]string)
	for k, v := range fields {
		fmCopy[k] = v
	}

	return fmCopy, true
}

type Config struct {
	SessionID string // unique identifier for this set of tokens in the database
}

func newValidator(method *structs.ACLAuthMethod) (authmethod.Validator, error) {
	if method.Type != "testing" {
		return nil, fmt.Errorf("%q is not a testing auth method", method.Name)
	}

	var config Config
	if err := authmethod.ParseConfig(method.Config, &config); err != nil {
		return nil, err
	}

	if config.SessionID == "" {
		// If you don't explicitly create one, we create a random one but you
		// won't have access to it. Useful if you are testing everything EXCEPT
		// ValidateToken().
		config.SessionID = StartSession()
	}

	return &Validator{
		name:   method.Name,
		config: &config,
	}, nil
}

type Validator struct {
	name   string
	config *Config
}

func (v *Validator) Name() string { return v.name }

// ValidateLogin takes raw user-provided auth method metadata and ensures it is
// sane, provably correct, and currently valid. Relevant identifying data is
// extracted and returned for immediate use by the role binding process.
//
// Depending upon the method, it may make sense to use these calls to continue
// to extend the life of the underlying token.
//
// Returns auth method specific metadata suitable for the Role Binding process.
func (v *Validator) ValidateLogin(loginToken string) (map[string]string, error) {
	fields, valid := GetSessionToken(v.config.SessionID, loginToken)
	if !valid {
		return nil, acl.ErrNotFound
	}

	return fields, nil
}

func (v *Validator) AvailableFields() []string { return availableFields }

const (
	serviceAccountNamespaceField = "serviceaccount.namespace"
	serviceAccountNameField      = "serviceaccount.name"
	serviceAccountUIDField       = "serviceaccount.uid"
)

var availableFields = []string{
	serviceAccountNamespaceField,
	serviceAccountNameField,
	serviceAccountUIDField,
}

// MakeFieldMapSelectable converts a field map as returned by ValidateLogin
// into a structure suitable for selection with a binding rule.
func (v *Validator) MakeFieldMapSelectable(fieldMap map[string]string) interface{} {
	return &selectableVars{
		ServiceAccount: selectableServiceAccount{
			Namespace: fieldMap[serviceAccountNamespaceField],
			Name:      fieldMap[serviceAccountNameField],
			UID:       fieldMap[serviceAccountUIDField],
		},
	}
}

type selectableVars struct {
	ServiceAccount selectableServiceAccount `bexpr:"serviceaccount"`
}

type selectableServiceAccount struct {
	Namespace string `bexpr:"namespace"`
	Name      string `bexpr:"name"`
	UID       string `bexpr:"uid"`
}
