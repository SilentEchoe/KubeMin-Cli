package model

import spec "kubemin-cli/pkg/apiserver/domain/spec"

func init() {
	RegisterModel(&NodeSelectorProfile{})
}

// NodeSelectorProfile stores reusable node selection rules in the datastore so they can be shared globally.
type NodeSelectorProfile struct {
	ID          string                 `json:"id" gorm:"primaryKey;type:varchar(24)"`
	Name        string                 `json:"name" gorm:"type:varchar(128);uniqueIndex;not null"`
	Description string                 `json:"description,omitempty" gorm:"type:text"`
	Selection   spec.NodeSelectionSpec `json:"selection" gorm:"serializer:json"`
	BaseModel
}

func (n *NodeSelectorProfile) PrimaryKey() string {
	return n.ID
}

func (n *NodeSelectorProfile) TableName() string {
	return tableNamePrefix + "node_selector_profiles"
}

func (n *NodeSelectorProfile) ShortTableName() string {
	return "node_selector_profile"
}

func (n *NodeSelectorProfile) Index() map[string]interface{} {
	index := make(map[string]interface{})
	if n.ID != "" {
		index["id"] = n.ID
	}
	if n.Name != "" {
		index["name"] = n.Name
	}
	return index
}
