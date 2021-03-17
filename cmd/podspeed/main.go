package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/markusthoemmes/podspeed/pkg/pod"
	statistics "github.com/montanaflynn/stats"
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

	readyCh := make(chan struct{}, podN)
	deletedCh := make(chan struct{}, podN)
	go func() {
		for event := range watcher.ResultChan() {
			now := time.Now()
			switch event.Type {
			case watch.Added:
				p := event.Object.(*corev1.Pod)
				stats := stats[p.Name]
				stats.Created = now
			case watch.Modified:
				p := event.Object.(*corev1.Pod)
				stats := stats[p.Name]
				if pod.IsConditionTrue(p, corev1.PodScheduled) && stats.Scheduled.IsZero() {
					stats.Scheduled = now
				}
				if pod.IsConditionTrue(p, corev1.PodInitialized) && stats.Initialized.IsZero() {
					stats.Initialized = now
				}
				if pod.IsConditionTrue(p, corev1.ContainersReady) && stats.ContainersReady.IsZero() {
					stats.ContainersReady = now
				}
				if pod.IsConditionTrue(p, corev1.PodReady) && stats.Ready.IsZero() {
					stats.Ready = now
					readyCh <- struct{}{}
				}
			case watch.Deleted:
				deletedCh <- struct{}{}
			}
		}
	}()

	for _, p := range pods {
		if _, err := kube.CoreV1().Pods(p.Namespace).Create(ctx, p, metav1.CreateOptions{}); err != nil {
			log.Fatal("Failed to create pod", err)
		}

		// Wait for all pods to become ready.
		if err := waitForN(ctx, readyCh, 1); err != nil {
			log.Fatal("Failed to wait for pod becoming ready", err)
		}

		var zero int64
		if err := kube.CoreV1().Pods(p.Namespace).Delete(ctx, p.Name, metav1.DeleteOptions{
			GracePeriodSeconds: &zero,
		}); err != nil {
			log.Fatal("Failed to delete pod", err)
		}

		waitForN(ctx, deletedCh, 1)
	}

	timeToReady := make([]float64, 0, len(stats))
	for _, stat := range stats {
		timeToReady = append(timeToReady, float64(stat.TimeToReady()/time.Millisecond))
	}

	min, _ := statistics.Min(timeToReady)
	max, _ := statistics.Max(timeToReady)
	mean, _ := statistics.Mean(timeToReady)
	p95, _ := statistics.Percentile(timeToReady, 95)
	p99, _ := statistics.Percentile(timeToReady, 99)
	fmt.Printf("Created %d pods sequentially, results are in ms:\n", podN)
	fmt.Printf("min: %.0f, max: %.0f, mean: %.0f, p95: %.0f, p99: %.0f\n", min, max, mean, p95, p99)
}

func waitForN(ctx context.Context, ch chan struct{}, n int) error {
	var seen int
	for {
		select {
		case <-ch:
			seen++
			if seen == n {
				return nil
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
