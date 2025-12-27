package job

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"kubemin-cli/pkg/apiserver/config"
	"kubemin-cli/pkg/apiserver/domain/model"
)

func TestDeployCleanupDeletesCreatedDeployment(t *testing.T) {
	client := fake.NewSimpleClientset()
	jobTask := &model.JobTask{Name: "web", Namespace: "default"}
	ctl := &DeployJobCtl{job: jobTask, client: client}

	ctx := WithCleanupTracker(context.Background())
	MarkResourceCreated(ctx, config.ResourceDeployment, "default", "web")

	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "web"}},
			Template: corePodTemplate("web"),
		},
	}
	if _, err := client.AppsV1().Deployments("default").Create(context.Background(), deploy, metav1.CreateOptions{}); err != nil {
		t.Fatalf("create deployment: %v", err)
	}

	ctl.Clean(ctx)

	if _, err := client.AppsV1().Deployments("default").Get(context.Background(), "web", metav1.GetOptions{}); err == nil {
		t.Fatalf("expected deployment to be deleted during cleanup")
	}
}

func corePodTemplate(name string) corev1.PodTemplateSpec {
	return corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": name}},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: name, Image: "nginx", Ports: []corev1.ContainerPort{{ContainerPort: 80}}}}},
	}
}
