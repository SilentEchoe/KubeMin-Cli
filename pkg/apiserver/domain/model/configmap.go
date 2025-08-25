package model

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const (
	// ConfigMapMaxSize 1MB in bytes
	ConfigMapMaxSize = 1024 * 1024
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
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	Type        string            `json:"type,omitempty"` // 默认为 Opaque
	Data        map[string]string `json:"data,omitempty"` // 将映射到 StringData
	URL         string            `json:"url,omitempty"`
	FileName    string            `json:"fileName,omitempty"`
}

// Helpers for Secret URL handling (reusing existing logic style)
func ReadFileFromURLForSecret(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP request failed with status: %d", resp.StatusCode)
	}
	rd := io.LimitReader(resp.Body, ConfigMapMaxSize+1024)
	data, err := io.ReadAll(rd)
	if err != nil {
		return nil, err
	}
	return data, nil
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
		if totalSize > ConfigMapMaxSize {
			return nil, fmt.Errorf("total ConfigMap data size %d bytes exceeds maximum size %d bytes", totalSize, ConfigMapMaxSize)
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
	body, err := readFileFromURLSimple(s.URL)
	if err != nil {
		return nil, err
	}
	if len(body) > ConfigMapMaxSize {
		return nil, fmt.Errorf("file size %d bytes exceeds ConfigMap maximum size %d bytes", len(body), ConfigMapMaxSize)
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

// ConfigMapFromMap 从Map创建ConfigMap的配置
type ConfigMapFromMap struct {
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	Data        map[string]string `json:"data" validate:"required"`
}

// ConfigMapFromURL 从URL文件创建ConfigMap的配置
type ConfigMapFromURL struct {
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	URL         string            `json:"url" validate:"required,url"`
	FileName    string            `json:"fileName,omitempty"` // 可选的文件名，如果不提供则从URL中提取
}

// ConfigMapJobInfo ConfigMap Job的信息
type ConfigMapJobInfo struct {
	Type    string            `json:"type"` // "map" 或 "url"
	FromMap *ConfigMapFromMap `json:"fromMap,omitempty"`
	FromURL *ConfigMapFromURL `json:"fromURL,omitempty"`
}

// CreateConfigMap 创建ConfigMap数据
func (c *ConfigMapJobInfo) CreateConfigMap() (*ConfigMapData, error) {
	switch c.Type {
	case "map":
		if c.FromMap == nil {
			return nil, fmt.Errorf("fromMap configuration is missing")
		}
		return &ConfigMapData{
			Name:        c.FromMap.Name,
			Namespace:   c.FromMap.Namespace,
			Labels:      c.FromMap.Labels,
			Annotations: c.FromMap.Annotations,
			Data:        c.FromMap.Data,
		}, nil

	case "url":
		if c.FromURL == nil {
			return nil, fmt.Errorf("fromURL configuration is missing")
		}

		// 从URL读取文件内容
		data, err := c.readFileFromURL()
		if err != nil {
			return nil, fmt.Errorf("failed to read file from URL: %w", err)
		}

		// 检查文件大小
		if len(data) > ConfigMapMaxSize {
			return nil, fmt.Errorf("file size %d bytes exceeds ConfigMap maximum size %d bytes", len(data), ConfigMapMaxSize)
		}

		// 确定文件名
		fileName := c.FromURL.FileName
		if fileName == "" {
			fileName = c.extractFileNameFromURL()
		}

		return &ConfigMapData{
			Name:        c.FromURL.Name,
			Namespace:   c.FromURL.Namespace,
			Labels:      c.FromURL.Labels,
			Annotations: c.FromURL.Annotations,
			Data: map[string]string{
				fileName: string(data),
			},
		}, nil

	default:
		return nil, fmt.Errorf("unknown ConfigMap type: %s", c.Type)
	}
}

// readFileFromURL 从URL读取文件内容
func (c *ConfigMapJobInfo) readFileFromURL() ([]byte, error) {
	resp, err := http.Get(c.FromURL.URL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP request failed with status: %d", resp.StatusCode)
	}

	// 限制读取大小，防止内存溢出
	limitedReader := io.LimitReader(resp.Body, ConfigMapMaxSize+1024) // 多读1KB用于检查
	data, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// extractFileNameFromURL 从URL中提取文件名
func (c *ConfigMapJobInfo) extractFileNameFromURL() string {
	url := c.FromURL.URL
	// 移除查询参数
	if idx := strings.Index(url, "?"); idx != -1 {
		url = url[:idx]
	}

	// 获取路径的最后一部分作为文件名
	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		fileName := parts[len(parts)-1]
		// 检查是否是有效的文件名（不是协议、空字符串或域名）
		if fileName != "" && fileName != "http:" && fileName != "https:" {
			// 检查是否是域名（没有路径的情况）
			if len(parts) <= 3 && (strings.Contains(fileName, ".") && !strings.Contains(fileName, "/")) {
				// 这是域名，不是文件名
				return "config"
			}
			return fileName
		}
	}

	// 如果无法提取文件名，使用默认名称
	return "config"
}

// Validate 验证ConfigMap配置
func (c *ConfigMapJobInfo) Validate() error {
	switch c.Type {
	case "map":
		if c.FromMap == nil {
			return fmt.Errorf("fromMap configuration is missing")
		}
		if c.FromMap.Name == "" {
			return fmt.Errorf("ConfigMap name is required")
		}
		if len(c.FromMap.Data) == 0 {
			return fmt.Errorf("ConfigMap data cannot be empty")
		}

		// 检查数据大小
		totalSize := 0
		for key, value := range c.FromMap.Data {
			if key == "" {
				return fmt.Errorf("ConfigMap key cannot be empty")
			}
			totalSize += len(key) + len(value)
		}

		if totalSize > ConfigMapMaxSize {
			return fmt.Errorf("total ConfigMap data size %d bytes exceeds maximum size %d bytes", totalSize, ConfigMapMaxSize)
		}

	case "url":
		if c.FromURL == nil {
			return fmt.Errorf("fromURL configuration is missing")
		}
		if c.FromURL.Name == "" {
			return fmt.Errorf("ConfigMap name is required")
		}
		if c.FromURL.URL == "" {
			return fmt.Errorf("URL is required")
		}

		// 验证URL格式
		if !strings.HasPrefix(c.FromURL.URL, "http://") && !strings.HasPrefix(c.FromURL.URL, "https://") {
			return fmt.Errorf("invalid URL format: must start with http:// or https://")
		}

	default:
		return fmt.Errorf("unknown ConfigMap type: %s", c.Type)
	}

	return nil
}

// ToJSON 转换为JSON字符串
func (c *ConfigMapJobInfo) ToJSON() (string, error) {
	data, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// FromJSON 从JSON字符串解析
func (c *ConfigMapJobInfo) FromJSON(jsonStr string) error {
	return json.Unmarshal([]byte(jsonStr), c)
}

// 工具：简化版给 ConfigMapSpec 复用
func readFileFromURLSimple(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP request failed with status: %d", resp.StatusCode)
	}
	rd := io.LimitReader(resp.Body, ConfigMapMaxSize+1024)
	data, err := io.ReadAll(rd)
	if err != nil {
		return nil, err
	}
	return data, nil
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
