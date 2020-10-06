package api

import "time"

type ServiceIntentionsConfigEntry struct {
	Kind      string
	Name      string
	Namespace string `json:",omitempty"`

	Sources []*SourceIntention

	Meta map[string]string `json:",omitempty"`

	CreateIndex uint64
	ModifyIndex uint64
}

type SourceIntention struct {
	Name        string
	Namespace   string `json:",omitempty"`
	Action      IntentionAction
	Precedence  int
	Type        IntentionSourceType
	Description string `json:",omitempty"`

	LegacyID         string            `json:",omitempty" alias:"legacy_id"`
	LegacyMeta       map[string]string `json:",omitempty" alias:"legacy_meta"`
	LegacyCreateTime *time.Time        `json:",omitempty" alias:"legacy_create_time"`
	LegacyUpdateTime *time.Time        `json:",omitempty" alias:"legacy_update_time"`
}

func (e *ServiceIntentionsConfigEntry) GetKind() string {
	return e.Kind
}

func (e *ServiceIntentionsConfigEntry) GetName() string {
	return e.Name
}

func (e *ServiceIntentionsConfigEntry) GetNamespace() string {
	return e.Namespace
}

func (e *ServiceIntentionsConfigEntry) GetMeta() map[string]string {
	return e.Meta
}

func (e *ServiceIntentionsConfigEntry) GetCreateIndex() uint64 {
	return e.CreateIndex
}

func (e *ServiceIntentionsConfigEntry) GetModifyIndex() uint64 {
	return e.ModifyIndex
}
