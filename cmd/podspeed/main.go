package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const basicApplicationImage = "docker.io/markusthoemmes/basic-500716b931f14b4a09df1ec4b4c5550d@sha256:f053b2bfef7f5bd32c678d64b0d4cd785004ed93b3ba512cdd8500212cabbb74"

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

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

	watcher, err := kube.CoreV1().Pods("default").Watch(ctx, metav1.ListOptions{
		LabelSelector: labels.Set{"podspeed/type": "basic"}.String(),
	})
	if err != nil {
		log.Fatal("Failed to setup watch for pods", err)
	}

	stats := &podTimes{}
	eventCh := make(chan podEvent, 10)
	go func() {
		for event := range watcher.ResultChan() {
			now := time.Now()
			switch event.Type {
			case watch.Added:
				stats.created = now
				eventCh <- podEvent{name: "created", time: now}
			case watch.Modified:
				pod := event.Object.(*corev1.Pod)
				if isPodCondTrue(pod, corev1.PodScheduled) && stats.scheduled.IsZero() {
					stats.scheduled = now
					eventCh <- podEvent{name: "scheduled", time: now}
				}
				if isPodCondTrue(pod, corev1.PodInitialized) && stats.initialized.IsZero() {
					stats.initialized = now
					eventCh <- podEvent{name: "initialized", time: now}
				}
				if isPodCondTrue(pod, corev1.ContainersReady) && stats.containersReady.IsZero() {
					stats.containersReady = now
					eventCh <- podEvent{name: "containerready", time: now}
				}
				if isPodCondTrue(pod, corev1.PodReady) && stats.ready.IsZero() {
					stats.ready = now
					eventCh <- podEvent{name: "ready", time: now}
				}
			case watch.Deleted:
				eventCh <- podEvent{name: "deleted", time: now}
				close(eventCh)
			}
		}
	}()

	pod := basicPod("default", "test")
	if _, err := kube.CoreV1().Pods(pod.Namespace).Create(ctx, pod, metav1.CreateOptions{}); err != nil {
		log.Fatal("Failed to create pod", err)
	}

	for event := range eventCh {
		log.Println(event.name, event.time)
		if event.name == "ready" {
			break
		}
	}
	log.Println("Until Scheduled took", stats.scheduled.Sub(stats.created))
	log.Println("Until Initialized took", stats.initialized.Sub(stats.created))
	log.Println("Until Ready took", stats.ready.Sub(stats.created))

	if err := kube.CoreV1().Pods(pod.Namespace).Delete(ctx, pod.Name, metav1.DeleteOptions{}); err != nil {
		log.Fatal("Failed to delete pod", err)
	}
	for event := range eventCh {
		log.Println(event.name, event.time)
	}
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

type podTimes struct {
	created         time.Time
	scheduled       time.Time
	initialized     time.Time
	containersReady time.Time
	ready           time.Time
}

type podEvent struct {
	name string
	time time.Time
}

func isPodCondTrue(p *corev1.Pod, condType corev1.PodConditionType) bool {
	for _, cond := range p.Status.Conditions {
		if cond.Type == condType && cond.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}
