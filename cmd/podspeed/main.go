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
				if pod.IsConditionTrue(p, corev1.PodScheduled) && stats.Scheduled.IsZero() {
					stats.Scheduled = now
					eventCh <- pod.Event{Name: p.Name, Time: now, Type: pod.Scheduled}
				}
				if pod.IsConditionTrue(p, corev1.PodInitialized) && stats.Initialized.IsZero() {
					stats.Initialized = now
					eventCh <- pod.Event{Name: p.Name, Time: now, Type: pod.Initialized}
				}
				if pod.IsConditionTrue(p, corev1.ContainersReady) && stats.ContainersReady.IsZero() {
					stats.ContainersReady = now
					eventCh <- pod.Event{Name: p.Name, Time: now, Type: pod.ContainersReady}
				}
				if pod.IsConditionTrue(p, corev1.PodReady) && stats.Ready.IsZero() {
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

	p := pod.Basic("default", "basic-"+uuid.NewString())
	if _, err := kube.CoreV1().Pods(p.Namespace).Create(ctx, p, metav1.CreateOptions{}); err != nil {
		log.Fatal("Failed to create pod", err)
	}

	if err := waitForReady(ctx, eventCh); err != nil {
		log.Fatal("Failed to wait for pod becoming ready", err)
	}

	log.Printf("Timings: Scheduled: %v, Initialized %v, Ready: %v\n",
		stats.TimeToScheduled(), stats.TimeToInitialized(), stats.TimeToReady())

	if err := kube.CoreV1().Pods(p.Namespace).Delete(ctx, p.Name, metav1.DeleteOptions{}); err != nil {
		log.Fatal("Failed to delete pod", err)
	}

	// The channel will be closed the pod is gone.
	for {
		select {
		case _, ok := <-eventCh:
			if !ok {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

func waitForReady(ctx context.Context, eventCh chan pod.Event) error {
	for {
		select {
		case event := <-eventCh:
			if event.Type == pod.Ready {
				return nil
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
