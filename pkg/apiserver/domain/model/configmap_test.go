package model

import (
	"testing"
)

func TestConfigMapFromMap_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *ConfigMapFromMap
		wantErr bool
	}{
		{
			name: "valid config",
			config: &ConfigMapFromMap{
				Name: "test-config",
				Data: map[string]string{
					"key1": "value1",
					"key2": "value2",
				},
			},
			wantErr: false,
		},
		{
			name: "missing name",
			config: &ConfigMapFromMap{
				Data: map[string]string{
					"key1": "value1",
				},
			},
			wantErr: true,
		},
		{
			name: "empty data",
			config: &ConfigMapFromMap{
				Name: "test-config",
				Data: map[string]string{},
			},
			wantErr: true,
		},
		{
			name: "empty key",
			config: &ConfigMapFromMap{
				Name: "test-config",
				Data: map[string]string{
					"": "value1",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jobInfo := &ConfigMapJobInfo{
				Type:    "map",
				FromMap: tt.config,
			}
			err := jobInfo.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("ConfigMapFromMap.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConfigMapFromURL_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *ConfigMapFromURL
		wantErr bool
	}{
		{
			name: "valid config",
			config: &ConfigMapFromURL{
				Name: "test-config",
				URL:  "https://example.com/config.txt",
			},
			wantErr: false,
		},
		{
			name: "missing name",
			config: &ConfigMapFromURL{
				URL: "https://example.com/config.txt",
			},
			wantErr: true,
		},
		{
			name: "missing URL",
			config: &ConfigMapFromURL{
				Name: "test-config",
			},
			wantErr: true,
		},
		{
			name: "invalid URL",
			config: &ConfigMapFromURL{
				Name: "test-config",
				URL:  "not-a-url",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jobInfo := &ConfigMapJobInfo{
				Type:    "url",
				FromURL: tt.config,
			}
			err := jobInfo.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("ConfigMapFromURL.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConfigMapJobInfo_CreateConfigMap_Map(t *testing.T) {
	jobInfo := &ConfigMapJobInfo{
		Type: "map",
		FromMap: &ConfigMapFromMap{
			Name:      "test-config",
			Namespace: "default",
			Labels: map[string]string{
				"app": "test",
			},
			Data: map[string]string{
				"config.yaml": "apiVersion: v1\nkind: ConfigMap",
				"env.txt":     "DEBUG=true",
			},
		},
	}

	configMap, err := jobInfo.CreateConfigMap()
	if err != nil {
		t.Fatalf("CreateConfigMap() error = %v", err)
	}

	if configMap.Name != "test-config" {
		t.Errorf("expected name 'test-config', got '%s'", configMap.Name)
	}

	if configMap.Namespace != "default" {
		t.Errorf("expected namespace 'default', got '%s'", configMap.Namespace)
	}

	if configMap.Labels["app"] != "test" {
		t.Errorf("expected label 'app=test', got 'app=%s'", configMap.Labels["app"])
	}

	if len(configMap.Data) != 2 {
		t.Errorf("expected 2 data entries, got %d", len(configMap.Data))
	}

	if configMap.Data["config.yaml"] != "apiVersion: v1\nkind: ConfigMap" {
		t.Errorf("unexpected config.yaml content")
	}
}

func TestConfigMapJobInfo_ExtractFileNameFromURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "simple filename",
			url:      "https://example.com/config.txt",
			expected: "config.txt",
		},
		{
			name:     "filename with query params",
			url:      "https://example.com/config.yaml?version=1.0",
			expected: "config.yaml",
		},
		{
			name:     "filename with path",
			url:      "https://example.com/path/to/config.json",
			expected: "config.json",
		},
		{
			name:     "empty path",
			url:      "https://example.com/",
			expected: "config",
		},
		{
			name:     "root domain",
			url:      "https://example.com",
			expected: "config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jobInfo := &ConfigMapJobInfo{
				Type: "url",
				FromURL: &ConfigMapFromURL{
					URL: tt.url,
				},
			}

			filename := jobInfo.extractFileNameFromURL()
			if filename != tt.expected {
				t.Errorf("extractFileNameFromURL() = %v, want %v", filename, tt.expected)
			}
		})
	}
}

func TestConfigMapJobInfo_DataSizeValidation(t *testing.T) {
	// 创建一个超过1MB的数据
	largeData := make(map[string]string)
	largeString := ""
	for i := 0; i < ConfigMapMaxSize+1024; i++ {
		largeString += "a"
	}
	largeData["large_file"] = largeString

	jobInfo := &ConfigMapJobInfo{
		Type: "map",
		FromMap: &ConfigMapFromMap{
			Name: "test-config",
			Data: largeData,
		},
	}

	err := jobInfo.Validate()
	if err == nil {
		t.Error("expected error for data exceeding 1MB limit")
	}

	if err.Error() == "" {
		t.Error("expected error message for data size limit")
	}
}

func TestConfigMapJobInfo_JSONSerialization(t *testing.T) {
	jobInfo := &ConfigMapJobInfo{
		Type: "map",
		FromMap: &ConfigMapFromMap{
			Name: "test-config",
			Data: map[string]string{
				"key": "value",
			},
		},
	}

	// 测试序列化
	jsonStr, err := jobInfo.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON() error = %v", err)
	}

	if jsonStr == "" {
		t.Error("expected non-empty JSON string")
	}

	// 测试反序列化
	newJobInfo := &ConfigMapJobInfo{}
	err = newJobInfo.FromJSON(jsonStr)
	if err != nil {
		t.Fatalf("FromJSON() error = %v", err)
	}

	if newJobInfo.Type != jobInfo.Type {
		t.Errorf("expected type '%s', got '%s'", jobInfo.Type, newJobInfo.Type)
	}

	if newJobInfo.FromMap.Name != jobInfo.FromMap.Name {
		t.Errorf("expected name '%s', got '%s'", jobInfo.FromMap.Name, newJobInfo.FromMap.Name)
	}
}
