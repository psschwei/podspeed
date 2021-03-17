package main

import (
	"context"
	"log"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const basicApplicationImage = "docker.io/markusthoemmes/basic-500716b931f14b4a09df1ec4b4c5550d@sha256:f053b2bfef7f5bd32c678d64b0d4cd785004ed93b3ba512cdd8500212cabbb74"

func main() {
	// Load kubernetes config.
	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(), nil).ClientConfig()
	if err != nil {
		log.Fatal("Failed to load config", err)
	}

	// Create kubernetes client.
	kube, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatal("Failed to create Kubernetes client", err)
	}

	log.Println("Setting up watch")
	watch, err := kube.CoreV1().Pods("default").Watch(context.TODO(), metav1.ListOptions{
		LabelSelector: labels.Set{"podspeed/type": "basic"}.String(),
	})
	if err != nil {
		log.Fatal("Failed to setup watch for pods", err)
	}

	go func() {
		for event := range watch.ResultChan() {
			log.Println("event", event)
		}
	}()

	log.Println("Creating pod")
	pod := basicPod("default", "test")
	if _, err := kube.CoreV1().Pods(pod.Namespace).Create(context.TODO(), pod, metav1.CreateOptions{}); err != nil {
		log.Fatal("Failed to create pod", err)
	}
	log.Println("Pod created successfully")

	log.Println("Deleting pod")
	if err := kube.CoreV1().Pods(pod.Namespace).Delete(context.TODO(), pod.Name, metav1.DeleteOptions{}); err != nil {
		log.Fatal("Failed to delete pod", err)
	}
	log.Println("Pod deleted successfully")
}

func basicPod(ns, name string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
			Labels: map[string]string{
				"podspeed/type": "basic",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "test",
				Image: basicApplicationImage,
			}},
		},
	}
}
