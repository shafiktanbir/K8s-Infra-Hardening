package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"path/filepath"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// initclientset builds a Kubernetes clientset from in-cluster configuration first,
// falling back to the provided kubeconfig file when executed locally.
func initclientset(kubeconfigPath string) (*kubernetes.Clientset, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfigPath)
		if err != nil {
			return nil, fmt.Errorf("failed to build kubeconfig: %w", err)
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	return clientset, nil
}

func evaluatePodHealth(pd *corev1.Pod) (unhealthy bool, reason string) {
	if pd.Status.Reason == "Evicted" {
		return true, "Evicted"
	}
	if pd.Status.Phase == corev1.PodFailed {
		return true, "Failed"
	}

	for _, cs := range pd.Status.ContainerStatuses {
		if cs.State.Waiting != nil {
			r := cs.State.Waiting.Reason
			if r == "ImagePullBackOff" || r == "CrashLoopBackOff" || r == "ErrImagePull" {
				return true, r
			}
		}
	}

	return false, ""
}

func main() {
	var defaultKubeconfig string
	if home := homedir.HomeDir(); home != "" {
		defaultKubeconfig = filepath.Join(home, ".kube", "config")
	}

	kubeconfig := flag.String("kubeconfig", defaultKubeconfig, "path to the kubeconfig file")
	namespace := flag.String("namespace", "", "namespace to audit (empty scans all namespaces)")
	dryRun := flag.Bool("dry-run", true, "report unhealthy pods without deleting")
	flag.Parse()

	clientset, err := initclientset(*kubeconfig)
	if err != nil {
		log.Fatalf("initializing clientset failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	podList, err := clientset.CoreV1().Pods(*namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Fatalf("failed to list pods: %v", err)
	}

	for _, pod := range podList.Items {
		unhealthy, reason := evaluatePodHealth(&pod)
		if !unhealthy {
			fmt.Printf("Healthy pod: %s/%s\n", pod.Namespace, pod.Name)
			continue
		}

		fmt.Printf("Unhealthy pod: %s/%s (Reason: %s)\n", pod.Namespace, pod.Name, reason)
		if !*dryRun && (reason == "Evicted" || reason == "Failed") {
			if err := clientset.CoreV1().Pods(pod.Namespace).Delete(ctx, pod.Name, metav1.DeleteOptions{}); err != nil {
				log.Printf("failed to delete pod %s/%s: %v", pod.Namespace, pod.Name, err)
			} else {
				fmt.Printf("Deleted unhealthy pod: %s/%s\n", pod.Namespace, pod.Name)
			}
		}
	}
}
