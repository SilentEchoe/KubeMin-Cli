package template

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/KubeMin-Cli/KubeMin-Cli/pkg/apiserver/domain/model"
	"github.com/KubeMin-Cli/KubeMin-Cli/pkg/apiserver/utils"
)

// Engine 模板引擎接口
type Engine interface {
	// RenderTemplate 渲染完整模板
	RenderTemplate(template *model.Template, parameters map[string]interface{}) (*RenderResult, error)

	// RenderComponents 渲染组件模板
	RenderComponents(components model.TemplateComponents, parameters map[string]interface{}) ([]model.ComponentTemplate, error)

	// RenderWorkflow 渲染工作流模板
	RenderWorkflow(workflow model.TemplateWorkflow, parameters map[string]interface{}) (*model.TemplateWorkflow, error)

	// ValidateParameters 验证参数
	ValidateParameters(params model.TemplateParameters, values map[string]interface{}) error

	// ExtractVariables 提取模板中的变量
	ExtractVariables(data interface{}) []string
}

// RenderResult 模板渲染结果
type RenderResult struct {
	Name       string                      `json:"name"`
	Components []model.ComponentTemplate   `json:"components"`
	Workflow   *model.TemplateWorkflow     `json:"workflow,omitempty"`
	Parameters map[string]interface{}      `json:"parameters"`
	Metadata   map[string]interface{}      `json:"metadata,omitempty"`
}

// templateEngine 模板引擎实现
type templateEngine struct {
	variablePattern *regexp.Regexp
	funcPattern     *regexp.Regexp
	builtinFuncs    map[string]TemplateFunc
}

// TemplateFunc 模板函数类型
type TemplateFunc func(args ...interface{}) (interface{}, error)

// NewEngine 创建新的模板引擎
func NewEngine() Engine {
	engine := &templateEngine{
		variablePattern: regexp.MustCompile(`\{\{([^}]+)\}\}`),
		funcPattern:     regexp.MustCompile(`(\w+)\(([^)]*)\)`),
		builtinFuncs:    make(map[string]TemplateFunc),
	}

	// 注册内置函数
	engine.registerBuiltinFuncs()
	return engine
}

// registerBuiltinFuncs 注册内置函数
func (e *templateEngine) registerBuiltinFuncs() {
	// 字符串函数
	e.builtinFuncs["lower"] = func(args ...interface{}) (interface{}, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("lower函数需要1个参数，得到%d个", len(args))
		}
		return strings.ToLower(fmt.Sprint(args[0])), nil
	}

	e.builtinFuncs["upper"] = func(args ...interface{}) (interface{}, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("upper函数需要1个参数，得到%d个", len(args))
		}
		return strings.ToUpper(fmt.Sprint(args[0])), nil
	}

	e.builtinFuncs["trim"] = func(args ...interface{}) (interface{}, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("trim函数需要1个参数，得到%d个", len(args))
		}
		return strings.TrimSpace(fmt.Sprint(args[0])), nil
	}

	// 字符串替换
	e.builtinFuncs["replace"] = func(args ...interface{}) (interface{}, error) {
		if len(args) != 3 {
			return nil, fmt.Errorf("replace函数需要3个参数，得到%d个", len(args))
		}
		str := fmt.Sprint(args[0])
		old := fmt.Sprint(args[1])
		new := fmt.Sprint(args[2])
		return strings.ReplaceAll(str, old, new), nil
	}

	// 截取字符串
	e.builtinFuncs["substring"] = func(args ...interface{}) (interface{}, error) {
		if len(args) < 2 || len(args) > 3 {
			return nil, fmt.Errorf("substring函数需要2-3个参数，得到%d个", len(args))
		}
		str := fmt.Sprint(args[0])
		start, err := strconv.Atoi(fmt.Sprint(args[1]))
		if err != nil {
			return nil, fmt.Errorf("substring起始位置必须是整数: %v", err)
		}

		if len(args) == 2 {
			if start >= len(str) {
				return "", nil
			}
			return str[start:], nil
		}

		length, err := strconv.Atoi(fmt.Sprint(args[2]))
		if err != nil {
			return nil, fmt.Errorf("substring长度必须是整数: %v", err)
		}

		if start >= len(str) {
			return "", nil
		}

		end := start + length
		if end > len(str) {
			end = len(str)
		}
		return str[start:end], nil
	}

	// 数学函数
	e.builtinFuncs["add"] = func(args ...interface{}) (interface{}, error) {
		if len(args) != 2 {
			return nil, fmt.Errorf("add函数需要2个参数，得到%d个", len(args))
		}

		// 尝试整数加法
		if a, err := strconv.Atoi(fmt.Sprint(args[0])); err == nil {
			if b, err := strconv.Atoi(fmt.Sprint(args[1])); err == nil {
				return a + b, nil
			}
		}

		// 尝试浮点数加法
		if a, err := strconv.ParseFloat(fmt.Sprint(args[0]), 64); err == nil {
			if b, err := strconv.ParseFloat(fmt.Sprint(args[1]), 64); err == nil {
				return a + b, nil
			}
		}

		return nil, fmt.Errorf("add函数参数必须是数字")
	}

	e.builtinFuncs["sub"] = func(args ...interface{}) (interface{}, error) {
		if len(args) != 2 {
			return nil, fmt.Errorf("sub函数需要2个参数，得到%d个", len(args))
		}

		// 尝试整数减法
		if a, err := strconv.Atoi(fmt.Sprint(args[0])); err == nil {
			if b, err := strconv.Atoi(fmt.Sprint(args[1])); err == nil {
				return a - b, nil
			}
		}

		// 尝试浮点数减法
		if a, err := strconv.ParseFloat(fmt.Sprint(args[0]), 64); err == nil {
			if b, err := strconv.ParseFloat(fmt.Sprint(args[1]), 64); err == nil {
				return a - b, nil
			}
		}

		return nil, fmt.Errorf("sub函数参数必须是数字")
	}

	// 条件函数
	e.builtinFuncs["default"] = func(args ...interface{}) (interface{}, error) {
		if len(args) != 2 {
			return nil, fmt.Errorf("default函数需要2个参数，得到%d个", len(args))
		}

		// 如果第一个参数为空或零值，返回第二个参数
		if utils.IsZeroValue(args[0]) {
			return args[1], nil
		}
		return args[0], nil
	}

	// 随机数生成
	e.builtinFuncs["random"] = func(args ...interface{}) (interface{}, error) {
		length := 8 // 默认长度
		if len(args) > 0 {
			l, err := strconv.Atoi(fmt.Sprint(args[0]))
			if err != nil {
				return nil, fmt.Errorf("random长度必须是整数: %v", err)
			}
			length = l
		}

		if length <= 0 {
			return "", nil
		}

		return utils.GenerateRandomString(length), nil
	}

	// 时间函数
	e.builtinFuncs["timestamp"] = func(args ...interface{}) (interface{}, error) {
		return fmt.Sprintf("%d", time.Now().Unix()), nil
	}

	// 环境相关函数
	e.builtinFuncs["env"] = func(args ...interface{}) (interface{}, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("env函数需要1个参数，得到%d个", len(args))
		}
		key := fmt.Sprint(args[0])
		value := os.Getenv(key)
		if value == "" {
			return key + "_NOT_SET", nil // 返回一个标记值，便于调试
		}
		return value, nil
	}
}

// RenderTemplate 渲染完整模板
func (e *templateEngine) RenderTemplate(template *model.Template, parameters map[string]interface{}) (*RenderResult, error) {
	if template == nil {
		return nil, fmt.Errorf("模板不能为空")
	}

	// 1. 验证参数
	if err := e.ValidateParameters(template.Parameters, parameters); err != nil {
		return nil, fmt.Errorf("参数验证失败: %v", err)
	}

	// 2. 应用默认值
	enrichedParams := e.applyDefaults(template.Parameters, parameters)

	// 3. 渲染组件
	components, err := e.RenderComponents(template.Components, enrichedParams)
	if err != nil {
		return nil, fmt.Errorf("渲染组件失败: %v", err)
	}

	// 4. 渲染工作流
	workflow := &template.Workflow
	if len(template.Workflow.Steps) > 0 {
		renderedWorkflow, err := e.RenderWorkflow(template.Workflow, enrichedParams)
		if err != nil {
			return nil, fmt.Errorf("渲染工作流失败: %v", err)
		}
		workflow = renderedWorkflow
	}

	// 5. 生成应用名称
	appName := e.generateAppName(template, enrichedParams)

	// 6. 处理组件间引用
	e.resolveComponentReferences(components, enrichedParams)

	return &RenderResult{
		Name:       appName,
		Components: components,
		Workflow:   workflow,
		Parameters: enrichedParams,
		Metadata: map[string]interface{}{
			"template_id":   template.ID,
			"template_name": template.Name,
			"version":       template.Version,
			"rendered_at":   time.Now().Format(time.RFC3339),
		},
	}, nil
}

// RenderComponents 渲染组件模板
func (e *templateEngine) RenderComponents(components model.TemplateComponents, parameters map[string]interface{}) ([]model.ComponentTemplate, error) {
	var renderedComponents []model.ComponentTemplate

	for i, compTemplate := range components.Components {
		// 检查条件是否满足
		if !e.evaluateConditions(compTemplate.Conditions, parameters) {
			continue
		}

		// 渲染组件名称
		renderedName, err := e.renderString(compTemplate.Name, parameters)
		if err != nil {
			return nil, fmt.Errorf("渲染组件[%d]名称失败: %v", i, err)
		}

		// 渲染属性
		renderedProps, err := e.renderInterface(compTemplate.Properties, parameters)
		if err != nil {
			return nil, fmt.Errorf("渲染组件[%d]属性失败: %v", i, err)
		}

		// 渲染Traits
		var renderedTraits map[string]interface{}
		if len(compTemplate.Traits) > 0 {
			renderedTraits, err = e.renderInterface(compTemplate.Traits, parameters)
			if err != nil {
				return nil, fmt.Errorf("渲染组件[%d]Traits失败: %v", i, err)
			}
		}

		// 应用命名规则
		if components.NamingRules != nil {
			renderedName = e.applyNamingRules(renderedName, components.NamingRules)
		}

		renderedComp := model.ComponentTemplate{
			Name:          renderedName.(string),
			Type:          compTemplate.Type,
			Description:   compTemplate.Description,
			Properties:    renderedProps.(map[string]interface{}),
			Traits:        renderedTraits,
			Conditions:    compTemplate.Conditions,
			Validation:    compTemplate.Validation,
			Documentation: compTemplate.Documentation,
		}

		renderedComponents = append(renderedComponents, renderedComp)
	}

	return renderedComponents, nil
}

// RenderWorkflow 渲染工作流模板
func (e *templateEngine) RenderWorkflow(workflow model.TemplateWorkflow, parameters map[string]interface{}) (*model.TemplateWorkflow, error) {
	var renderedSteps []model.WorkflowStepTemplate

	for i, step := range workflow.Steps {
		// 检查步骤条件
		if !e.evaluateConditions(step.Conditions, parameters) {
			continue
		}

		// 渲染步骤名称
		renderedName, err := e.renderString(step.Name, parameters)
		if err != nil {
			return nil, fmt.Errorf("渲染工作流步骤[%d]名称失败: %v", i, err)
		}

		// 渲染步骤属性
		renderedProps, err := e.renderInterface(step.Properties, parameters)
		if err != nil {
			return nil, fmt.Errorf("渲染工作流步骤[%d]属性失败: %v", i, err)
		}

		// 渲染组件引用
		var renderedComponents []string
		for _, compRef := range step.Components {
			renderedRef, err := e.renderString(compRef, parameters)
			if err != nil {
				return nil, fmt.Errorf("渲染工作流步骤[%d]组件引用失败: %v", i, err)
			}
			renderedComponents = append(renderedComponents, renderedRef.(string))
		}

		renderedStep := model.WorkflowStepTemplate{
			Name:       renderedName.(string),
			Type:       step.Type,
			Components: renderedComponents,
			Properties: renderedProps.(map[string]interface{}),
			Conditions: step.Conditions,
			Timeout:    step.Timeout,
			Retry:      step.Retry,
		}

		renderedSteps = append(renderedSteps, renderedStep)
	}

	return &model.TemplateWorkflow{
		Steps:            renderedSteps,
		Strategy:         workflow.Strategy,
		Parameterization: workflow.Parameterization,
	}, nil
}

// ValidateParameters 验证参数
func (e *templateEngine) ValidateParameters(params model.TemplateParameters, values map[string]interface{}) error {
	// 1. 检查必需参数
	for _, def := range params.Definitions {
		if def.Required {
			if _, exists := values[def.Name]; !exists {
				if def.Default == nil {
					return fmt.Errorf("缺少必需参数: %s", def.Name)
				}
			}
		}
	}

	// 2. 验证每个参数值
	for key, value := range values {
		def := e.findParameterDefinition(params.Definitions, key)
		if def == nil {
			// 允许额外的参数，但给出警告
			continue
		}

		if err := e.validateParameterValue(def, value); err != nil {
			return fmt.Errorf("参数 %s 验证失败: %v", key, err)
		}
	}

	// 3. 检查参数依赖
	for _, def := range params.Definitions {
		if len(def.DependsOn) > 0 {
			if _, exists := values[def.Name]; exists {
				for _, dep := range def.DependsOn {
					if _, depExists := values[dep]; !depExists {
						return fmt.Errorf("参数 %s 依赖于 %s，但后者未提供", def.Name, dep)
					}
				}
			}
		}
	}

	// 4. 全局验证规则
	if params.Validation != nil {
		if err := e.validateGlobalRules(params.Validation, values); err != nil {
			return err
		}
	}

	return nil
}

// ExtractVariables 提取模板中的变量
func (e *templateEngine) ExtractVariables(data interface{}) []string {
	variableMap := make(map[string]bool)
	e.extractVariablesRecursive(data, variableMap)

	var variables []string
	for v := range variableMap {
		variables = append(variables, v)
	}
	return variables
}

// 辅助方法

// renderString 渲染字符串模板
func (e *templateEngine) renderString(template string, parameters map[string]interface{}) (interface{}, error) {
	result := e.variablePattern.ReplaceAllStringFunc(template, func(match string) string {
		// 提取变量名或函数调用
		expr := strings.TrimSpace(match[2 : len(match)-2])

		// 尝试作为函数调用处理
		if funcMatch := e.funcPattern.FindStringSubmatch(expr); funcMatch != nil {
			funcName := funcMatch[1]
			funcArgs := strings.Split(funcMatch[2], ",")

			for i, arg := range funcArgs {
				funcArgs[i] = strings.TrimSpace(arg)
			}

			if fn, exists := e.builtinFuncs[funcName]; exists {
				args := make([]interface{}, len(funcArgs))
				for i, arg := range funcArgs {
					// 解析参数（可能是变量引用或字面值）
					args[i] = e.parseArgument(arg, parameters)
				}

				result, err := fn(args...)
				if err != nil {
					return fmt.Sprintf("[ERROR:%v]", err)
				}
				return fmt.Sprint(result)
			}
		}

		// 作为变量处理
		if value, exists := parameters[expr]; exists {
			return fmt.Sprint(value)
		}

		return fmt.Sprintf("[MISSING:%s]", expr)
	})

	return result, nil
}

// renderInterface 渲染任意接口
func (e *templateEngine) renderInterface(data interface{}, parameters map[string]interface{}) (interface{}, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("JSON编码失败: %v", err)
	}

	renderedStr, err := e.renderString(string(jsonData), parameters)
	if err != nil {
		return nil, err
	}

	var result interface{}
	if err := json.Unmarshal([]byte(renderedStr.(string)), &result); err != nil {
		return nil, fmt.Errorf("JSON解码失败: %v", err)
	}

	return result, nil
}

// 其他辅助方法将在后续实现中完成
func (e *templateEngine) applyDefaults(params model.TemplateParameters, values map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	// 复制已有值
	for k, v := range values {
		result[k] = v
	}

	// 应用默认值
	for _, def := range params.Definitions {
		if _, exists := result[def.Name]; !exists {
			if def.Default != nil {
				result[def.Name] = def.Default
			}
		}
	}

	return result
}

func (e *templateEngine) generateAppName(template *model.Template, parameters map[string]interface{}) string {
	// 生成应用名称的逻辑
	if name, exists := parameters["app_name"]; exists {
		if nameStr, ok := name.(string); ok {
			return nameStr
		}
	}

	// 使用模板名称 + 时间戳
	return fmt.Sprintf("%s-%d", template.Name, time.Now().Unix())
}

func (e *templateEngine) evaluateConditions(conditions []string, parameters map[string]interface{}) bool {
	if len(conditions) == 0 {
		return true
	}

	// 简化的条件评估逻辑
	for _, condition := range conditions {
		// 这里可以实现更复杂的条件表达式解析
		if strings.Contains(condition, "==") {
			parts := strings.Split(condition, "==")
			if len(parts) == 2 {
				param := strings.TrimSpace(parts[0])
				expected := strings.TrimSpace(parts[1])
				if val, exists := parameters[param]; exists {
					if fmt.Sprint(val) != expected {
						return false
					}
				} else {
					return false
				}
			}
		}
	}

	return true
}

func (e *templateEngine) resolveComponentReferences(components []model.ComponentTemplate, parameters map[string]interface{}) {
	// 解析组件间引用的逻辑
	// 例如处理 depends_on、connects_to 等关系
}

func (e *templateEngine) applyNamingRules(name string, rules *model.NamingRules) string {
	// 应用命名规则的逻辑
	result := name

	if rules.Prefix != "" {
		result = rules.Prefix + rules.Separator + result
	}

	if rules.Suffix != "" {
		result = result + rules.Separator + rules.Suffix
	}

	// 应用转换规则
	switch rules.Transform {
	case model.NamingTransformLowercase:
		result = strings.ToLower(result)
	case model.NamingTransformUppercase:
		result = strings.ToUpper(result)
	case model.NamingTransformKebabCase:
		result = strings.ReplaceAll(strings.ToLower(result), "_", "-")
	case model.NamingTransformSnakeCase:
		result = strings.ReplaceAll(strings.ToLower(result), "-", "_")
	}

	return result
}

func (e *templateEngine) findParameterDefinition(definitions []model.ParameterDefinition, name string) *model.ParameterDefinition {
	for i := range definitions {
		if definitions[i].Name == name {
			return &definitions[i]
		}
	}
	return nil
}

func (e *templateEngine) validateParameterValue(def *model.ParameterDefinition, value interface{}) error {
	if def.Validation == nil {
		return nil
	}

	// 类型验证
	switch def.Type {
	case model.ParameterTypeString:
		str := fmt.Sprint(value)

		if def.Validation.MinLength != nil && len(str) < *def.Validation.MinLength {
			return fmt.Errorf("字符串长度不能小于%d", *def.Validation.MinLength)
		}

		if def.Validation.MaxLength != nil && len(str) > *def.Validation.MaxLength {
			return fmt.Errorf("字符串长度不能大于%d", *def.Validation.MaxLength)
		}

		if def.Validation.Pattern != "" {
			matched, err := regexp.MatchString(def.Validation.Pattern, str)
			if err != nil {
				return fmt.Errorf("正则表达式验证失败: %v", err)
			}
			if !matched {
				return fmt.Errorf("格式不符合要求")
			}
		}

	case model.ParameterTypeInt:
		num, err := strconv.Atoi(fmt.Sprint(value))
		if err != nil {
			return fmt.Errorf("必须是整数")
		}

		if def.Validation.MinValue != nil && num < *def.Validation.MinValue {
			return fmt.Errorf("数值不能小于%d", *def.Validation.MinValue)
		}

		if def.Validation.MaxValue != nil && num > *def.Validation.MaxValue {
			return fmt.Errorf("数值不能大于%d", *def.Validation.MaxValue)
		}
	}

	return nil
}

func (e *templateEngine) validateGlobalRules(validation *model.ParameterValidation, values map[string]interface{}) error {
	// 实现全局验证逻辑
	return nil
}

func (e *templateEngine) extractVariablesRecursive(data interface{}, variableMap map[string]bool) {
	switch v := data.(type) {
	case string:
		matches := e.variablePattern.FindAllStringSubmatch(v, -1)
		for _, match := range matches {
			if len(match) > 1 {
				variableMap[strings.TrimSpace(match[1])] = true
			}
		}
	case map[string]interface{}:
		for _, value := range v {
			e.extractVariablesRecursive(value, variableMap)
		}
	case []interface{}:
		for _, item := range v {
			e.extractVariablesRecursive(item, variableMap)
		}
	}
}

func (e *templateEngine) parseArgument(arg string, parameters map[string]interface{}) interface{} {
	arg = strings.TrimSpace(arg)

	// 如果是字符串字面值（被引号包围）
	if (strings.HasPrefix(arg, "'") && strings.HasSuffix(arg, "'")) ||
	   (strings.HasPrefix(arg, "\"") && strings.HasSuffix(arg, "\"")) {
		return arg[1 : len(arg)-1]
	}

	// 如果是数字字面值
	if num, err := strconv.Atoi(arg); err == nil {
		return num
	}

	// 如果是浮点数
	if float, err := strconv.ParseFloat(arg, 64); err == nil {
		return float
	}

	// 如果是布尔值
	if arg == "true" {
		return true
	}
	if arg == "false" {
		return false
	}

	// 作为变量引用
	if val, exists := parameters[arg]; exists {
		return val
	}

	// 返回原字符串（可能是未定义的变量）
	return arg
}

// Helper function to check if value is zero
func isZeroValue(v interface{}) bool {
	if v == nil {
		return true
	}

	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.String:
		return rv.String() == ""
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return rv.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return rv.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return rv.Float() == 0
	case reflect.Bool:
		return !rv.Bool()
	case reflect.Slice, reflect.Map:
		return rv.Len() == 0
	}

	return false
}

// Import required packages at the top
import (
	"os"
	"time"
	"math/rand"
)

// GenerateRandomString 生成随机字符串
func GenerateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

// Import time package for timestamp function
import (
	"time"
)

// Update timestamp function to use current time
func init() {
	// 在builtinFuncs注册中添加timestamp函数的实现
	// 已在registerBuiltinFuncs函数中实现
}

// TemplateEngineOption 模板引擎选项
type TemplateEngineOption func(*templateEngine)

// WithCustomFunc 添加自定义函数
func WithCustomFunc(name string, fn TemplateFunc) TemplateEngineOption {
	return func(e *templateEngine) {
		e.builtinFuncs[name] = fn
	}
}

// WithVariablePattern 设置变量模式
func WithVariablePattern(pattern string) TemplateEngineOption {
	return func(e *templateEngine) {
		e.variablePattern = regexp.MustCompile(pattern)
	}
}

// NewEngineWithOptions 使用选项创建模板引擎
func NewEngineWithOptions(opts ...TemplateEngineOption) Engine {
	engine := NewEngine().(*templateEngine)
	for _, opt := range opts {
		opt(engine)
	}
	return engine
}

// TemplateContext 模板上下文
type TemplateContext struct {
	Parameters map[string]interface{}
	Metadata   map[string]interface{}
	Functions  map[string]TemplateFunc
}

// NewTemplateContext 创建模板上下文
func NewTemplateContext() *TemplateContext {
	return &TemplateContext{
		Parameters: make(map[string]interface{}),
		Metadata:   make(map[string]interface{}),
		Functions:  make(map[string]TemplateFunc),
	}
}

// AddParameter 添加参数
func (c *TemplateContext) AddParameter(key string, value interface{}) {
	c.Parameters[key] = value
}

// AddMetadata 添加元数据
func (c *TemplateContext) AddMetadata(key string, value interface{}) {
	c.Metadata[key] = value
}

// AddFunction 添加自定义函数
func (c *TemplateContext) AddFunction(name string, fn TemplateFunc) {
	c.Functions[name] = fn
}

// TemplateRenderer 模板渲染器
type TemplateRenderer interface {
	Render(ctx *TemplateContext) (interface{}, error)
}

// StringTemplate 字符串模板
type StringTemplate struct {
	Template string
}

// Render 渲染字符串模板
func (t *StringTemplate) Render(ctx *TemplateContext) (interface{}, error) {
	engine := NewEngineWithOptions(
		WithCustomFunc("metadata", func(args ...interface{}) (interface{}, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("metadata函数需要1个参数")
			}
			key := fmt.Sprint(args[0])
			if val, exists := ctx.Metadata[key]; exists {
				return val, nil
			}
			return nil, fmt.Errorf("metadata键 %s 不存在", key)
		}),
	)

	// 合并参数和元数据
	allParams := make(map[string]interface{})
	for k, v := range ctx.Parameters {
		allParams[k] = v
	}
	for k, v := range ctx.Metadata {
		allParams["meta."+k] = v
	}

	return engine.renderString(t.Template, allParams)
}

// Error definitions
type TemplateError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
}

func (e TemplateError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Type, e.Message)
}

// Common template errors
var (
	ErrTemplateNotFound    = &TemplateError{Type: "TEMPLATE_NOT_FOUND", Message: "模板不存在"}
	ErrParameterRequired   = &TemplateError{Type: "PARAMETER_REQUIRED", Message: "缺少必需参数"}
	ErrParameterInvalid    = &TemplateError{Type: "PARAMETER_INVALID", Message: "参数值无效"}
	ErrRenderFailed        = &TemplateError{Type: "RENDER_FAILED", Message: "模板渲染失败"}
	ErrValidationFailed    = &TemplateError{Type: "VALIDATION_FAILED", Message: "验证失败"}
	ErrCircularReference   = &TemplateError{Type: "CIRCULAR_REFERENCE", Message: "循环引用"}
	ErrFunctionNotFound    = &TemplateError{Type: "FUNCTION_NOT_FOUND", Message: "函数不存在"}
	ErrFunctionInvalidArgs = &TemplateError{Type: "FUNCTION_INVALID_ARGS", Message: "函数参数无效"}
)

// TemplateValidator 模板验证器
type TemplateValidator interface {
	Validate(template *model.Template) error
	ValidateParameters(params model.TemplateParameters, values map[string]interface{}) error
}

// DefaultTemplateValidator 默认模板验证器
type DefaultTemplateValidator struct{}

// Validate 验证模板
func (v *DefaultTemplateValidator) Validate(template *model.Template) error {
	if template == nil {
		return fmt.Errorf("模板不能为空")
	}

	if template.Name == "" {
		return fmt.Errorf("模板名称不能为空")
	}

	if len(template.Components.Components) == 0 {
		return fmt.Errorf("模板至少需要定义一个组件")
	}

	// 验证组件定义
	for i, comp := range template.Components.Components {
		if comp.Name == "" {
			return fmt.Errorf("组件[%d]名称不能为空", i)
		}
		if comp.Type == "" {
			return fmt.Errorf("组件[%d]类型不能为空", i)
		}
	}

	// 验证参数定义
	for i, param := range template.Parameters.Definitions {
		if param.Name == "" {
			return fmt.Errorf("参数[%d]名称不能为空", i)
		}
		if param.Type == "" {
			return fmt.Errorf("参数[%d]类型不能为空", i)
		}
	}

	return nil
}

// ValidateParameters 验证参数
func (v *DefaultTemplateValidator) ValidateParameters(params model.TemplateParameters, values map[string]interface{}) error {
	engine := NewEngine()
	return engine.ValidateParameters(params, values)
}

// TemplateManager 模板管理器
type TemplateManager interface {
	CreateTemplate(template *model.Template) error
	UpdateTemplate(template *model.Template) error
	DeleteTemplate(id string) error
	GetTemplate(id string) (*model.Template, error)
	ListTemplates(filter TemplateFilter) ([]*model.Template, error)
	InstantiateTemplate(templateID string, parameters map[string]interface{}) (*RenderResult, error)
}

// TemplateFilter 模板过滤条件
type TemplateFilter struct {
	Category   string
	IsPublic   *bool
	IsSystem   *bool
	Status     string
	Tags       []string
	Author     string
	SearchText string
	Limit      int
	Offset     int
}

// TemplateStatistics 模板统计信息
type TemplateStatistics struct {
	TotalCount       int64  `json:"total_count"`
	ActiveCount      int64  `json:"active_count"`
	PublicCount      int64  `json:"public_count"`
	SystemCount      int64  `json:"system_count"`
	PopularTags      []TagCount `json:"popular_tags"`
	MostUsedTemplate string     `json:"most_used_template"`
}

// TagCount 标签统计
type TagCount struct {
	Tag   string `json:"tag"`
	Count int64  `json:"count"`
}

// TemplateRecommendations 模板推荐
type TemplateRecommendations struct {
	ByCategory   map[string][]*model.Template `json:"by_category"`
	ByUsage      []*model.Template           `json:"by_usage"`
	ByPopularity []*model.Template           `json:"by_popularity"`
}

// TemplateSharing 模板分享
type TemplateSharing struct {
	TemplateID    string   `json:"template_id"`
	SharedBy      string   `json:"shared_by"`
	SharedWith    []string `json:"shared_with"`
	Permissions   []string `json:"permissions"` // read, write, execute
	ExpiryTime    *time.Time `json:"expiry_time,omitempty"`
	SharedAt      time.Time `json:"shared_at"`
}

// TemplateImportExport 模板导入导出
type TemplateImportExport struct {
	Format    string           `json:"format"` // json, yaml
	Templates []*model.Template `json:"templates"`
	Metadata  map[string]interface{} `json:"metadata"`
}

// TemplateBackup 模板备份
type TemplateBackup struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Templates  []string  `json:"templates"`
	BackupTime time.Time `json:"backup_time"`
	Size       int64     `json:"size"`
	Checksum   string    `json:"checksum"`
}

// TemplateSearchResult 模板搜索结果
type TemplateSearchResult struct {
	Templates    []*model.Template `json:"templates"`
	TotalCount   int64             `json:"total_count"`
	SearchTime   int64             `json:"search_time_ms"`
	Suggestions  []string          `json:"suggestions,omitempty"`
	RelatedTags  []string          `json:"related_tags,omitempty"`
}

// TemplateUsageAnalytics 模板使用分析
type TemplateUsageAnalytics struct {
	TemplateID      string                 `json:"template_id"`
	UsageCount      int64                  `json:"usage_count"`
	SuccessRate     float64                `json:"success_rate"`
	AverageTime     int64                  `json:"average_time_ms"`
	PopularParams   map[string]interface{} `json:"popular_params"`
	CommonErrors    []string               `json:"common_errors"`
	UserFeedback    *UserFeedbackStats     `json:"user_feedback"`
}

// UserFeedbackStats 用户反馈统计
type UserFeedbackStats struct {
	TotalRatings  int64   `json:"total_ratings"`
	AverageRating float64 `json:"average_rating"`
	Comments      []string `json:"comments"`
}

// TemplateComparison 模板对比
type TemplateComparison struct {
	TemplateA *model.Template        `json:"template_a"`
	TemplateB *model.Template        `json:"template_b"`
	Differences []TemplateDifference `json:"differences"`
	Similarity  float64              `json:"similarity"`
}

// TemplateDifference 模板差异
type TemplateDifference struct {
	Field      string      `json:"field"`
	Type       string      `json:"type"` // added, removed, modified
	OldValue   interface{} `json:"old_value,omitempty"`
	NewValue   interface{} `json:"new_value,omitempty"`
	Path       string      `json:"path,omitempty"`
}

// TemplateCloning 模板克隆
type TemplateCloning struct {
	SourceTemplateID string                 `json:"source_template_id"`
	TargetName       string                 `json:"target_name"`
	Modifications    map[string]interface{} `json:"modifications"`
	PreserveHistory  bool                   `json:"preserve_history"`
	NewVersion       string                 `json:"new_version,omitempty"`
}

// TemplateMigration 模板迁移
type TemplateMigration struct {
	ID               string    `json:"id"`
	SourceVersion    string    `json:"source_version"`
	TargetVersion    string    `json:"target_version"`
	MigrationSteps   []MigrationStep `json:"migration_steps"`
	RollbackSteps    []MigrationStep `json:"rollback_steps"`
	Compatibility    CompatibilityInfo `json:"compatibility"`
	CreatedAt        time.Time `json:"created_at"`
}

// MigrationStep 迁移步骤
type MigrationStep struct {
	Type        string      `json:"type"` // transform, validate, convert
	Description string      `json:"description"`
	Operation   string      `json:"operation"`
	Parameters  interface{} `json:"parameters,omitempty"`
	Rollback    string      `json:"rollback,omitempty"`
}

// CompatibilityInfo 兼容性信息
type CompatibilityInfo struct {
	BreakingChanges []string `json:"breaking_changes"`
	NewFeatures     []string `json:"new_features"`
	Deprecated      []string `json:"deprecated"`
	MigrationNotes  string   `json:"migration_notes"`
}

// TemplateDeployment 模板部署
type TemplateDeployment struct {
	TemplateID      string                 `json:"template_id"`
	Parameters      map[string]interface{} `json:"parameters"`
	TargetNamespace string                 `json:"target_namespace"`
	DryRun          bool                   `json:"dry_run"`
	PreviewOnly     bool                   `json:"preview_only"`
}

// TemplateDeploymentResult 模板部署结果
type TemplateDeploymentResult struct {
	Success        bool                   `json:"success"`
	ApplicationID  string                 `json:"application_id,omitempty"`
	Errors         []string               `json:"errors,omitempty"`
	Warnings       []string               `json:"warnings,omitempty"`
	RenderedOutput *RenderResult           `json:"rendered_output,omitempty"`
	Preview        map[string]interface{} `json:"preview,omitempty"`
}

// TemplateFieldMapping 模板字段映射
type TemplateFieldMapping struct {
	SourceField string `json:"source_field"`
	TargetField string `json:"target_field"`
	Transform   string `json:"transform,omitempty"`
	Default     interface{} `json:"default,omitempty"`
	Required    bool   `json:"required"`
}

// TemplateFieldValidation 模板字段验证
type TemplateFieldValidation struct {
	Field      string         `json:"field"`
	Rules      []ValidationRule `json:"rules"`
	Messages   map[string]string `json:"messages,omitempty"`
}

// ValidationRule 验证规则
type ValidationRule struct {
	Type     string      `json:"type"` // required, pattern, range, custom
	Value    interface{} `json:"value,omitempty"`
	Message  string      `json:"message,omitempty"`
}

// TemplateRenderingOptions 模板渲染选项
type TemplateRenderingOptions struct {
	StrictMode        bool                   `json:"strict_mode"`
	PreserveUnknown   bool                   `json:"preserve_unknown"`
	CustomFunctions   map[string]TemplateFunc `json:"custom_functions,omitempty"`
	ValidationMode    string                 `json:"validation_mode"` // strict, warning, none
	NamingStrategy    string                 `json:"naming_strategy"` // kebab-case, snake_case, camelCase
	GenerateIDs       bool                   `json:"generate_ids"`
	TimestampFormat   string                 `json:"timestamp_format"`
}

// DefaultRenderingOptions 默认渲染选项
func DefaultRenderingOptions() *TemplateRenderingOptions {
	return &TemplateRenderingOptions{
		StrictMode:      false,
		PreserveUnknown: true,
		ValidationMode:  "warning",
		NamingStrategy:  "kebab-case",
		GenerateIDs:     false,
		TimestampFormat: time.RFC3339,
	}
}

// TemplateParameterBuilder 模板参数构建器
type TemplateParameterBuilder struct {
	definitions []model.ParameterDefinition
	groups      []model.ParameterGroup
}

// NewTemplateParameterBuilder 创建参数构建器
func NewTemplateParameterBuilder() *TemplateParameterBuilder {
	return &TemplateParameterBuilder{
		definitions: make([]model.ParameterDefinition, 0),
		groups:      make([]model.ParameterGroup, 0),
	}
}

// AddParameter 添加参数
func (b *TemplateParameterBuilder) AddParameter(def model.ParameterDefinition) *TemplateParameterBuilder {
	b.definitions = append(b.definitions, def)
	return b
}

// AddGroup 添加参数组
func (b *TemplateParameterBuilder) AddGroup(group model.ParameterGroup) *TemplateParameterBuilder {
	b.groups = append(b.groups, group)
	return b
}

// Build 构建参数定义
func (b *TemplateParameterBuilder) Build() model.TemplateParameters {
	return model.TemplateParameters{
		Definitions: b.definitions,
		Groups:      b.groups,
	}
}

// TemplateBuilder 模板构建器
type TemplateBuilder struct {
	template *model.Template
}

// NewTemplateBuilder 创建模板构建器
func NewTemplateBuilder(name string) *TemplateBuilder {
	return &TemplateBuilder{
		template: &model.Template{
			ID:          utils.GenerateRandomString(24),
			Name:        name,
			Parameters:  model.TemplateParameters{},
			Components:  model.TemplateComponents{},
			Workflow:    model.TemplateWorkflow{},
			IsPublic:    false,
			IsSystem:    false,
			Status:      model.TemplateStatusActive,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}
}

// SetDisplayName 设置显示名称
func (b *TemplateBuilder) SetDisplayName(name string) *TemplateBuilder {
	b.template.DisplayName = name
	return b
}

// SetDescription 设置描述
func (b *TemplateBuilder) SetDescription(desc string) *TemplateBuilder {
	b.template.Description = desc
	return b
}

// SetCategory 设置分类
func (b *TemplateBuilder) SetCategory(category string) *TemplateBuilder {
	b.template.Category = category
	return b
}

// SetParameters 设置参数
func (b *TemplateBuilder) SetParameters(params model.TemplateParameters) *TemplateBuilder {
	b.template.Parameters = params
	return b
}

// SetComponents 设置组件
func (b *TemplateBuilder) SetComponents(components model.TemplateComponents) *TemplateBuilder {
	b.template.Components = components
	return b
}

// SetWorkflow 设置工作流
func (b *TemplateBuilder) SetWorkflow(workflow model.TemplateWorkflow) *TemplateBuilder {
	b.template.Workflow = workflow
	return b
}

// Build 构建模板
func (b *TemplateBuilder) Build() *model.Template {
	return b.template
}

// TemplateCache 模板缓存接口
type TemplateCache interface {
	Get(key string) (*model.Template, bool)
	Set(key string, template *model.Template, ttl time.Duration)
	Delete(key string)
	Clear()
}

// InMemoryTemplateCache 内存模板缓存
type InMemoryTemplateCache struct {
	cache map[string]*cacheEntry
	mu    sync.RWMutex
}

type cacheEntry struct {
	template  *model.Template
	expireAt  time.Time
}

// NewInMemoryTemplateCache 创建内存缓存
func NewInMemoryTemplateCache() TemplateCache {
	return &InMemoryTemplateCache{
		cache: make(map[string]*cacheEntry),
	}
}

// Get 获取缓存
func (c *InMemoryTemplateCache) Get(key string) (*model.Template, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.cache[key]
	if !exists || time.Now().After(entry.expireAt) {
		return nil, false
	}

	return entry.template, true
}

// Set 设置缓存
func (c *InMemoryTemplateCache) Set(key string, template *model.Template, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache[key] = &cacheEntry{
		template: template,
		expireAt: time.Now().Add(ttl),
	}
}

// Delete 删除缓存
func (c *InMemoryTemplateCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.cache, key)
}

// Clear 清空缓存
func (c *InMemoryTemplateCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache = make(map[string]*cacheEntry)
}

// Import sync package
import "sync"

// TemplateMetrics 模板指标
type TemplateMetrics struct {
	RenderCount       int64   `json:"render_count"`
	RenderSuccessRate float64 `json:"render_success_rate"`
	AverageRenderTime int64   `json:"average_render_time_ms"`
	ErrorCount        int64   `json:"error_count"`
	CacheHitRate      float64 `json:"cache_hit_rate"`
}

// TemplateAnalyticsCollector 模板分析收集器
type TemplateAnalyticsCollector interface {
	RecordRender(templateID string, success bool, duration int64, error error)
	RecordCacheHit(templateID string, hit bool)
	GetMetrics(templateID string) *TemplateMetrics
	GetGlobalMetrics() *TemplateMetrics
}

// DefaultTemplateAnalyticsCollector 默认分析收集器
type DefaultTemplateAnalyticsCollector struct {
	mu        sync.RWMutex
	metrics   map[string]*TemplateMetrics
	globalMetrics *TemplateMetrics
}

// NewDefaultTemplateAnalyticsCollector 创建默认收集器
func NewDefaultTemplateAnalyticsCollector() TemplateAnalyticsCollector {
	return &DefaultTemplateAnalyticsCollector{
		metrics: make(map[string]*TemplateMetrics),
		globalMetrics: &TemplateMetrics{},
	}
}

// RecordRender 记录渲染
func (c *DefaultTemplateAnalyticsCollector) RecordRender(templateID string, success bool, duration int64, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.metrics[templateID]; !exists {
		c.metrics[templateID] = &TemplateMetrics{}
	}

	metric := c.metrics[templateID]
	metric.RenderCount++

	if success {
		successCount := int64(float64(metric.RenderCount) * metric.RenderSuccessRate)
		successCount++
		metric.RenderSuccessRate = float64(successCount) / float64(metric.RenderCount)

		// 更新平均渲染时间
		if metric.AverageRenderTime == 0 {
			metric.AverageRenderTime = duration
		} else {
			metric.AverageRenderTime = (metric.AverageRenderTime + duration) / 2
		}
	} else {
		metric.ErrorCount++
	}

	// 更新全局指标
	c.globalMetrics.RenderCount++
	if success {
		successCount := int64(float64(c.globalMetrics.RenderCount) * c.globalMetrics.RenderSuccessRate)
		successCount++
		c.globalMetrics.RenderSuccessRate = float64(successCount) / float64(c.globalMetrics.RenderCount)
	} else {
		c.globalMetrics.ErrorCount++
	}
}

// RecordCacheHit 记录缓存命中
func (c *DefaultTemplateAnalyticsCollector) RecordCacheHit(templateID string, hit bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.metrics[templateID]; !exists {
		c.metrics[templateID] = &TemplateMetrics{}
	}

	// 简化的缓存命中率计算
	if hit {
		c.metrics[templateID].CacheHitRate = 1.0
	}
}

// GetMetrics 获取模板指标
func (c *DefaultTemplateAnalyticsCollector) GetMetrics(templateID string) *TemplateMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if metric, exists := c.metrics[templateID]; exists {
		return metric
	}
	return &TemplateMetrics{}
}

// GetGlobalMetrics 获取全局指标
func (c *DefaultTemplateAnalyticsCollector) GetGlobalMetrics() *TemplateMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.globalMetrics
}