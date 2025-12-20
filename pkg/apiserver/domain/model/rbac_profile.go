package model

import spec "KubeMin-Cli/pkg/apiserver/domain/spec"

func init() {
	RegisterModel(&RBACProfile{})
}

// RBACProfile stores reusable RBAC definitions in the datastore so they can be shared globally.
// It can hold multiple RBAC policies, each of which will materialize ServiceAccount/Role/Binding resources.
type RBACProfile struct {
	ID          string                `json:"id" gorm:"primaryKey;type:varchar(24)"`
	Name        string                `json:"name" gorm:"type:varchar(128);uniqueIndex;not null"`
	Description string                `json:"description,omitempty" gorm:"type:text"`
	Policies    []spec.RBACPolicySpec `json:"policies" gorm:"serializer:json"`
	BaseModel
}

func (r *RBACProfile) PrimaryKey() string {
	return r.ID
}

func (r *RBACProfile) TableName() string {
	return tableNamePrefix + "rbac_profiles"
}

func (r *RBACProfile) ShortTableName() string {
	return "rbac_profile"
}

func (r *RBACProfile) Index() map[string]interface{} {
	index := make(map[string]interface{})
	if r.ID != "" {
		index["id"] = r.ID
	}
	if r.Name != "" {
		index["name"] = r.Name
	}
	return index
}
