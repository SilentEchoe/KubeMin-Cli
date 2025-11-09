package kube

import (
	"context"
	"os"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/utils/pointer"
)

func withPodEnv(name, namespace string, fn func()) {
	origName := os.Getenv("POD_NAME")
	origNS := os.Getenv("POD_NAMESPACE")
	_ = os.Setenv("POD_NAME", name)
	_ = os.Setenv("POD_NAMESPACE", namespace)
	defer func() {
		_ = os.Setenv("POD_NAME", origName)
		_ = os.Setenv("POD_NAMESPACE", origNS)
	}()
	fn()
}

func TestDetectReplicaCountDeployment(t *testing.T) {
	withPodEnv("pod-0", "default", func() {
		dep := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "default"},
			Spec:       appsv1.DeploymentSpec{Replicas: pointer.Int32(3)},
		}
		rs := &appsv1.ReplicaSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "api-rs",
				Namespace: "default",
				OwnerReferences: []metav1.OwnerReference{{
					Kind: "Deployment",
					Name: "api",
				}},
			},
			Spec: appsv1.ReplicaSetSpec{Replicas: pointer.Int32(3)},
		}
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod-0",
				Namespace: "default",
				OwnerReferences: []metav1.OwnerReference{{
					Kind: "ReplicaSet",
					Name: "api-rs",
				}},
			},
		}
		client := fake.NewSimpleClientset(dep, rs, pod)
		if got := DetectReplicaCount(context.Background(), client); got != 3 {
			t.Fatalf("expected 3 replicas, got %d", got)
		}
	})
}

func TestDetectReplicaCountStatefulSet(t *testing.T) {
	withPodEnv("stateful-0", "default", func() {
		ss := &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: "default"},
			Spec:       appsv1.StatefulSetSpec{Replicas: pointer.Int32(5)},
		}
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "stateful-0",
				Namespace: "default",
				OwnerReferences: []metav1.OwnerReference{{
					Kind: "StatefulSet",
					Name: "db",
				}},
			},
		}
		client := fake.NewSimpleClientset(ss, pod)
		if got := DetectReplicaCount(context.Background(), client); got != 5 {
			t.Fatalf("expected 5 replicas, got %d", got)
		}
	})
}

func TestDetectReplicaCountDaemonSet(t *testing.T) {
	withPodEnv("daemon-0", "default", func() {
		ds := &appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{Name: "edge", Namespace: "default"},
			Status: appsv1.DaemonSetStatus{
				DesiredNumberScheduled: 4,
			},
		}
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "daemon-0",
				Namespace: "default",
				OwnerReferences: []metav1.OwnerReference{{
					Kind: "DaemonSet",
					Name: "edge",
				}},
			},
		}
		client := fake.NewSimpleClientset(ds, pod)
		if got := DetectReplicaCount(context.Background(), client); got != 4 {
			t.Fatalf("expected 4 replicas, got %d", got)
		}
	})
}
