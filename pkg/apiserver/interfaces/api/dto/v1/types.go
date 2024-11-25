package v1

import "time"

// ApplicationBase application base model
type ApplicationBase struct {
	Name        string            `json:"name"`
	Alias       string            `json:"alias"`
	Project     *ProjectBase      `json:"project"`
	Description string            `json:"description"`
	CreateTime  time.Time         `json:"createTime"`
	UpdateTime  time.Time         `json:"updateTime"`
	Icon        string            `json:"icon"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	ReadOnly    bool              `json:"readOnly,omitempty"`
}

// ProjectBase project base model
type ProjectBase struct {
	Name        string    `json:"name"`
	Alias       string    `json:"alias"`
	Description string    `json:"description"`
	CreateTime  time.Time `json:"createTime"`
	UpdateTime  time.Time `json:"updateTime"`
	Owner       NameAlias `json:"owner,omitempty"`
	Namespace   string    `json:"namespace"`
}

// NameAlias name and alias
type NameAlias struct {
	Name  string `json:"name"`
	Alias string `json:"alias"`
}

// ListApplicationResponse list applications by query params
type ListApplicationResponse struct {
	Applications []*ApplicationBase `json:"applications"`
}
