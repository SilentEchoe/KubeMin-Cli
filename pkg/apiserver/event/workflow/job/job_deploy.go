package job

import (
	"context"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
)

type DeployJobCtl struct {
	job       *JobTask
	namespace string
	informer  informers.SharedInformerFactory
	clientSet *kubernetes.Clientset
	ack       func()
}

func NewDeployJobCtl(job *JobTask, ack func()) *DeployJobCtl {
	return &DeployJobCtl{
		job: job,
		ack: ack,
	}
}

func (c *DeployJobCtl) Clean(ctx context.Context) {}

// SaveInfo  创建Job的详情信息
func (c *DeployJobCtl) SaveInfo(ctx context.Context) error {
	return nil
}

func (c *DeployJobCtl) Run(ctx context.Context) {

}
