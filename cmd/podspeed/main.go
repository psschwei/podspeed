package main

import (
	"context"
	"flag"
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
	var (
		ns   string
		podN int
	)
	flag.StringVar(&ns, "n", "default", "the namespace to create the pods in")
	flag.IntVar(&podN, "pods", 1, "the amount of pods to create")
	flag.Parse()

	if podN < 1 {
		log.Fatal("-pods must not be smaller than 1")
	}

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

	watcher, err := kube.CoreV1().Pods(ns).Watch(ctx, metav1.ListOptions{
		LabelSelector: labels.Set{"podspeed/type": "basic"}.String(),
	})
	if err != nil {
		log.Fatal("Failed to setup watch for pods", err)
	}

	pods := make([]*corev1.Pod, 0, podN)
	stats := make(map[string]*pod.Stats, podN)

	for i := 0; i < podN; i++ {
		p := pod.Basic(ns, "basic-"+uuid.NewString())
		pods = append(pods, p)
		stats[p.Name] = &pod.Stats{}
	}

	eventCh := make(chan pod.Event, 10)
	go func() {
		for event := range watcher.ResultChan() {
			now := time.Now()
			switch event.Type {
			case watch.Added:
				p := event.Object.(*corev1.Pod)
				stats := stats[p.Name]
				stats.Created = now
				eventCh <- pod.Event{Name: p.Name, Time: now, Type: pod.Created}
			case watch.Modified:
				p := event.Object.(*corev1.Pod)
				stats := stats[p.Name]
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
			}
		}
	}()

	for _, p := range pods {
		if _, err := kube.CoreV1().Pods(p.Namespace).Create(ctx, p, metav1.CreateOptions{}); err != nil {
			log.Fatal("Failed to create pod", err)
		}
	}

	// Wait for all pods to become ready.
	if err := waitForEventN(ctx, eventCh, pod.Ready, podN); err != nil {
		log.Fatal("Failed to wait for pod becoming ready", err)
	}

	for _, stat := range stats {
		log.Printf("Timings: Scheduled: %v, Initialized %v, Ready: %v\n",
			stat.TimeToScheduled(), stat.TimeToInitialized(), stat.TimeToReady())
	}

	for _, p := range pods {
		if err := kube.CoreV1().Pods(p.Namespace).Delete(ctx, p.Name, metav1.DeleteOptions{}); err != nil {
			log.Fatal("Failed to delete pod", err)
		}
	}

	// Wait for all pods to have been deleted.
	waitForEventN(ctx, eventCh, pod.Deleted, podN)
}

func waitForEventN(ctx context.Context, eventCh chan pod.Event, eventType pod.EventType, podN int) error {
	var seen int
	for {
		select {
		case event := <-eventCh:
			if event.Type == eventType {
				seen++
				if seen == podN {
					return nil
				}
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
