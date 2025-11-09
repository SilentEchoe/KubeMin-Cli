package kube

import (
	"context"
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

// DetectReplicaCount attempts to determine the desired replica count of the controller managing this Pod.
// It relies on POD_NAME and POD_NAMESPACE env vars; if unavailable or any error occurs, returns 1.
func DetectReplicaCount(ctx context.Context, client kubernetes.Interface) int {
	podName := os.Getenv("POD_NAME")
	ns := os.Getenv("POD_NAMESPACE")
	if podName == "" || ns == "" {
		klog.V(3).Infof("POD_NAME or POD_NAMESPACE not set; default replica count = 1")
		return 1
	}
	pod, err := client.CoreV1().Pods(ns).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		klog.V(3).Infof("get pod failed: %v; default replica count = 1", err)
		return 1
	}
	// find top-level owner
	for _, or := range pod.OwnerReferences {
		switch or.Kind {
		case "ReplicaSet":
			rs, err := client.AppsV1().ReplicaSets(ns).Get(ctx, or.Name, metav1.GetOptions{})
			if err != nil {
				klog.V(3).Infof("get replicaset failed: %v; default replica count = 1", err)
				return 1
			}
			// if owned by Deployment
			for _, rsOwner := range rs.OwnerReferences {
				if rsOwner.Kind == "Deployment" {
					dep, err := client.AppsV1().Deployments(ns).Get(ctx, rsOwner.Name, metav1.GetOptions{})
					if err != nil {
						klog.V(3).Infof("get deployment failed: %v; default replica count = 1", err)
						return 1
					}
					return int(orInt32(dep.Spec.Replicas, 1))
				}
			}
			// fall back to replicaset replicas
			return int(orInt32(rs.Spec.Replicas, 1))
		case "StatefulSet":
			ss, err := client.AppsV1().StatefulSets(ns).Get(ctx, or.Name, metav1.GetOptions{})
			if err != nil {
				klog.V(3).Infof("get statefulset failed: %v; default replica count = 1", err)
				return 1
			}
			return int(orInt32(ss.Spec.Replicas, 1))
		case "DaemonSet":
			ds, err := client.AppsV1().DaemonSets(ns).Get(ctx, or.Name, metav1.GetOptions{})
			if err != nil {
				klog.V(3).Infof("get daemonset failed: %v; default replica count = 1", err)
				return 1
			}
			// desired scheduled as a proxy for count
			return int(ds.Status.DesiredNumberScheduled)
		}
	}
	return 1
}

func orInt32(p *int32, d int32) int32 {
	if p == nil {
		return d
	}
	return *p
}
