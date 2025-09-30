package kube

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func DeletePvcWithName(namespace, name string, clientset kubernetes.Interface) error {
	deletePolicy := metav1.DeletePropagationForeground
	return clientset.CoreV1().PersistentVolumeClaims(namespace).Delete(
		context.TODO(), name,
		metav1.DeleteOptions{
			PropagationPolicy: &deletePolicy,
		},
	)
}

func CreatePvc(namespace string, pvc *corev1.PersistentVolumeClaim, clientset kubernetes.Interface) error {
	_, err := clientset.CoreV1().PersistentVolumeClaims(namespace).Create(context.TODO(), pvc,
		metav1.CreateOptions{})
	return err
}

func UpdatePvc(namespace string, pvc *corev1.PersistentVolumeClaim, clientset kubernetes.Interface) error {
	_, err := clientset.CoreV1().PersistentVolumeClaims(namespace).Update(context.TODO(), pvc,
		metav1.UpdateOptions{})
	return err
}
