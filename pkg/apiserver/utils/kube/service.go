package kube

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// ServiceExists 判断指定命名空间下的 Service 是否存在
func ServiceExists(ctx context.Context, client *kubernetes.Clientset, name, namespace string) (*corev1.Service, bool, error) {
	svc, err := client.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return svc, true, nil
}

func ParseInt64(i int64) *int64 {
	return &i
}
