package model

import (
	"fmt"
	"strings"

	"KubeMin-Cli/pkg/apiserver/utils"
)

// ConfigMapData 定义ConfigMap的数据结构
type ConfigMapData struct {
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	Data        map[string]string `json:"data"`
}

// SecretInput : 与 ConfigMapInput 类似，支持 Data 或 URL（URL 下载后作为单文件注入）。
// 注意：Secret 的值需为字节；通过 StringData 便捷传入。
type SecretInput struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Labels    map[string]string `json:"labels,omitempty"`
	Type      string            `json:"type,omitempty"`
	Data      map[string]string `json:"data,omitempty"`
	URL       string            `json:"url,omitempty"`
	FileName  string            `json:"fileName,omitempty"`
}

func ExtractFileNameFromURLForSecret(url string) string {
	if idx := strings.Index(url, "?"); idx != -1 {
		url = url[:idx]
	}
	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		fn := parts[len(parts)-1]
		if fn != "" && fn != "http:" && fn != "https:" {
			if len(parts) <= 3 && (strings.Contains(fn, ".") && !strings.Contains(fn, "/")) {
				return "secret"
			}
			return fn
		}
	}
	return "secret"
}

// ConfigMapInput : 简化的声明，仅支持 Data 或 URL 二选一
type ConfigMapInput struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Labels    map[string]string `json:"labels,omitempty"`
	Data      map[string]string `json:"data,omitempty"`
	URL       string            `json:"url,omitempty"`
	FileName  string            `json:"fileName,omitempty"`
}

// GenerateConf 根据 Data 或 URL 生成标准 ConfigMapData
func (s *ConfigMapInput) GenerateConf() (*ConfigMapData, error) {
	if s.Name == "" {
		return nil, fmt.Errorf("ConfigMap name is required")
	}
	if len(s.Data) == 0 && s.URL == "" {
		return nil, fmt.Errorf("either data or url must be provided")
	}
	if len(s.Data) > 0 && s.URL != "" {
		return nil, fmt.Errorf("data and url are mutually exclusive")
	}

	if len(s.Data) > 0 {
		totalSize := 0
		for k, v := range s.Data {
			if k == "" {
				return nil, fmt.Errorf("ConfigMap key cannot be empty")
			}
			totalSize += len(k) + len(v)
		}
		if totalSize > utils.ConfigMapMaxSize {
			return nil, fmt.Errorf("total ConfigMap data size %d bytes exceeds maximum size %d bytes", totalSize, utils.ConfigMapMaxSize)
		}
		return &ConfigMapData{
			Name:      s.Name,
			Namespace: s.Namespace,
			Labels:    s.Labels,
			Data:      s.Data,
		}, nil
	}

	// URL 路径
	if !strings.HasPrefix(s.URL, "http://") && !strings.HasPrefix(s.URL, "https://") {
		return nil, fmt.Errorf("invalid URL format: must start with http:// or https://")
	}
	body, err := utils.ReadFileFromURLSimple(s.URL)
	if err != nil {
		return nil, err
	}
	if len(body) > utils.ConfigMapMaxSize {
		return nil, fmt.Errorf("file size %d bytes exceeds ConfigMap maximum size %d bytes", len(body), utils.ConfigMapMaxSize)
	}
	fileName := s.FileName
	if fileName == "" {
		fileName = extractFileNameFromURLSimple(s.URL)
	}
	return &ConfigMapData{
		Name:      s.Name,
		Namespace: s.Namespace,
		Labels:    s.Labels,
		Data:      map[string]string{fileName: string(body)},
	}, nil
}

func extractFileNameFromURLSimple(url string) string {
	if idx := strings.Index(url, "?"); idx != -1 {
		url = url[:idx]
	}
	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		fn := parts[len(parts)-1]
		if fn != "" && fn != "http:" && fn != "https:" {
			if len(parts) <= 3 && (strings.Contains(fn, ".") && !strings.Contains(fn, "/")) {
				return "config"
			}
			return fn
		}
	}
	return "config"
}
