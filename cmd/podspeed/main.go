package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/markusthoemmes/podspeed/pkg/pod"
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

	stats := &pod.Stats{}
	eventCh := make(chan pod.Event, 10)
	go func() {
		for event := range watcher.ResultChan() {
			now := time.Now()
			switch event.Type {
			case watch.Added:
				p := event.Object.(*corev1.Pod)
				stats.Created = now
				eventCh <- pod.Event{Name: p.Name, Time: now, Type: pod.Created}
			case watch.Modified:
				p := event.Object.(*corev1.Pod)
				if isPodCondTrue(p, corev1.PodScheduled) && stats.Scheduled.IsZero() {
					stats.Scheduled = now
					eventCh <- pod.Event{Name: p.Name, Time: now, Type: pod.Scheduled}
				}
				if isPodCondTrue(p, corev1.PodInitialized) && stats.Initialized.IsZero() {
					stats.Initialized = now
					eventCh <- pod.Event{Name: p.Name, Time: now, Type: pod.Initialized}
				}
				if isPodCondTrue(p, corev1.ContainersReady) && stats.ContainersReady.IsZero() {
					stats.ContainersReady = now
					eventCh <- pod.Event{Name: p.Name, Time: now, Type: pod.ContainersReady}
				}
				if isPodCondTrue(p, corev1.PodReady) && stats.Ready.IsZero() {
					stats.Ready = now
					eventCh <- pod.Event{Name: p.Name, Time: now, Type: pod.Ready}
				}
			case watch.Deleted:
				p := event.Object.(*corev1.Pod)
				eventCh <- pod.Event{Name: p.Name, Time: now, Type: pod.Deleted}
				close(eventCh)
			}
		}
	}()

	p := basicPod("default", "basic-"+uuid.NewString())
	if _, err := kube.CoreV1().Pods(p.Namespace).Create(ctx, p, metav1.CreateOptions{}); err != nil {
		log.Fatal("Failed to create pod", err)
	}

	for event := range eventCh {
		// Wait for the pod to become ready before we delete it.
		if event.Type == pod.Ready {
			break
		}
	}

	log.Printf("Timings: Scheduled: %v, Initialized %v, Ready: %v\n",
		stats.TimeToScheduled(), stats.TimeToInitialized(), stats.TimeToReady())

	if err := kube.CoreV1().Pods(p.Namespace).Delete(ctx, p.Name, metav1.DeleteOptions{}); err != nil {
		log.Fatal("Failed to delete pod", err)
	}

	// The channel will be closed the pod is gone.
	for range eventCh {
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

func isPodCondTrue(p *corev1.Pod, condType corev1.PodConditionType) bool {
	for _, cond := range p.Status.Conditions {
		if cond.Type == condType && cond.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}
