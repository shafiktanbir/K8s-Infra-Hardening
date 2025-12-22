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
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// initclientset takes the path to kubeconfig, builds the config, and instantiates the clientset.
// This function doesn't know or care about command-line flags. It just takes a path.
func initclientset(kubeconfigPath string) (*kubernetes.Clientset, error) {
	// Use clientcmd.BuildConfigFromFlags to build the configuration.
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to build kubeconfig: %w", err)
	}

	// Use kubernetes.NewForConfig to create the clientset using the config.
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
	if pd.Status.Phase == "Failed" {
		return true, "Failed"
	}

	// if pd.Status.ContainerStatuses
	for _, cs := range pd.Status.ContainerStatuses {
		// Check if container is in a Waiting failure state
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
	var kubeconfigPath string

	// Resolve the default kubeconfig path (~/.kube/config) if the home directory is found.
	if home := homedir.HomeDir(); home != "" {
		kubeconfigPath = filepath.Join(home, ".kube", "config")
	}

	// Register the CLI flag. Use short variable declaration (:=) so that
	// the compiler automatically infers that kubeconfig is a *string.
	kubeconfig := flag.String("kubeconfig", kubeconfigPath, "path to the kubeconfig file")

	// Parse the command-line flags.
	flag.Parse()

	// Call initclientset. Since kubeconfig is a *string (pointer),
	// we must dereference it using *kubeconfig to pass the actual string value.
	clientset, err := initclientset(*kubeconfig)
	if err != nil {
		log.Fatalf("initializing clientset failed: %v", err)
	}

	// fmt.Printf("clientset initialized: %v\n", clientset)

	//checking container status
	// evaluatePodHealth()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 2. Fetch all pods across all namespaces ("")
	podList, err := clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Fatalf("failed to list pods: %v", err)
	}

	for _, pod := range podList.Items {
		unhealthy, reason := evaluatePodHealth(&pod)
		if unhealthy {
			fmt.Printf("Unhealthy pod: %s/%s, Reason: %s\n", pod.Namespace, pod.Name, reason)
		} else {
			fmt.Printf("Healthy pod: %s/%s\n", pod.Namespace, pod.Name)
		}

	}

}
