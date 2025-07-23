package workflow

import (
	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"fmt"
	"k8s.io/klog/v2"
	"regexp"
)

// LintWorkflow 验证工作流是否符合标准
func LintWorkflow(workflow *model.Workflow) error {
	if workflow.ProjectId == "" {
		err := fmt.Errorf("project should not be empty")
		klog.Errorf(err.Error())
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
		klog.Errorf(errMsg)
		return fmt.Errorf(errMsg)
	}
	return nil
}
