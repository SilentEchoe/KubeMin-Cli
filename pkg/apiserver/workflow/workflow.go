package workflow

import (
	"fmt"
	"regexp"
	"strings"

	"k8s.io/klog/v2"

	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/model"
)

// LintWorkflow 验证工作流是否符合标准
func LintWorkflow(workflow *model.Workflow) error {
	workflow.Name = strings.ToLower(workflow.Name)
	if workflow.ProjectID == "" {
		err := fmt.Errorf("project should not be empty")
		klog.Errorf("%v", err)
		return err
	}
	// 判断工作流的名称是否符合正则表达式的规范
	match, err := regexp.MatchString(config.WorkflowRegx, workflow.Name)
	if err != nil {
		klog.Errorf("reg compile failed: %v", err)
		return err
	}
	if !match {
		errMsg := "workflow identifier supports uppercase and lowercase letters, digits, and hyphens"
		klog.Error(errMsg)
		return fmt.Errorf("%s", errMsg)
	}
	return nil
}
