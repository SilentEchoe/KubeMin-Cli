/*
Copyright 2021 The KubeVela Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package model

import (
	"encoding/json"
	"fmt"

	"time"

	"k8s.io/klog/v2"
)

var registeredModels = map[string]Interface{}

// Interface model interface
type Interface interface {
	TableName() string
	ShortTableName() string
}

// RegisterModel register model
func RegisterModel(models ...Interface) {
	for _, model := range models {
		if _, exist := registeredModels[model.TableName()]; exist {
			panic(fmt.Errorf("model table name %s conflict", model.TableName()))
		}
		registeredModels[model.TableName()] = model
	}
}

// GetRegisterModels will return the register models
func GetRegisterModels() map[string]Interface {
	return registeredModels
}

// JSONStruct json struct, same with runtime.RawExtension
type JSONStruct map[string]interface{}

// JSON Encoded as a JSON string
func (j *JSONStruct) JSON() string {
	b, err := json.Marshal(j)
	if err != nil {
		klog.Errorf("json marshal failure %s", err.Error())
	}
	return string(b)
}

// BaseModel common model
type BaseModel struct {
	CreateTime time.Time `json:"createTime"`
	UpdateTime time.Time `json:"updateTime"`
}

// SetCreateTime set create time
func (m *BaseModel) SetCreateTime(time time.Time) {
	m.CreateTime = time
}

// SetUpdateTime set update time
func (m *BaseModel) SetUpdateTime(time time.Time) {
	m.UpdateTime = time
}
