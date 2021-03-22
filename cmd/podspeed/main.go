package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/google/uuid"
	"github.com/markusthoemmes/podspeed/pkg/pod"
	statistics "github.com/montanaflynn/stats"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	var (
		ns         string
		typ        string
		podN       int
		skipDelete bool
	)
	flag.StringVar(&ns, "n", "default", "the namespace to create the pods in")
	flag.StringVar(&typ, "typ", "basic", "the type of pods to create, either 'basic', 'knative-head' or 'knative-v0.21'")
	flag.IntVar(&podN, "pods", 1, "the amount of pods to create")
	flag.BoolVar(&skipDelete, "skip-delete", false, "skip removing the pods after they're ready if true")
	flag.Parse()

	var podFn func(string, string) *corev1.Pod
	switch typ {
	case "basic":
		podFn = pod.Basic
	case "knative-head":
		podFn = pod.KnativeHead
	case "knative-v0.21":
		podFn = pod.Knative021
	default:
		log.Fatal("-typ must be either 'basic', 'knative-head' or 'knative-v0.21'")
	}
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

	watcher, err := kube.CoreV1().Pods(ns).Watch(ctx, metav1.ListOptions{})
	if err != nil {
		log.Fatal("Failed to setup watch for pods", err)
	}

	pods := make([]*corev1.Pod, 0, podN)
	stats := make(map[string]*pod.Stats, podN)

	for i := 0; i < podN; i++ {
		p := podFn(ns, typ+"-"+uuid.NewString())
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
					stats.ContainersStarted = pod.LastContainerStartedTime(p)
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

		if !skipDelete {
			var zero int64
			if err := kube.CoreV1().Pods(p.Namespace).Delete(ctx, p.Name, metav1.DeleteOptions{
				GracePeriodSeconds: &zero,
			}); err != nil {
				log.Fatal("Failed to delete pod", err)
			}

			waitForN(ctx, deletedCh, 1)
		}
	}

	timeToScheduled := make([]float64, 0, len(stats))
	timeToReady := make([]float64, 0, len(stats))
	for _, stat := range stats {
		timeToScheduled = append(timeToScheduled, float64(stat.TimeToScheduled()/time.Millisecond))
		timeToReady = append(timeToReady, float64(stat.TimeToReady()/time.Millisecond))
	}

	fmt.Printf("Created %d %s pods sequentially, results are in ms:\n", podN, typ)
	fmt.Println()
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.Debug)
	fmt.Fprintln(w, "metric\tmin\tmax\tmean\tp95\tp99")
	printStats(w, "Time to scheduled", timeToScheduled)
	printStats(w, "Time to ready", timeToReady)
	w.Flush()
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

func printStats(w io.Writer, label string, data []float64) {
	min, _ := statistics.Min(data)
	max, _ := statistics.Max(data)
	mean, _ := statistics.Mean(data)
	p95, _ := statistics.Percentile(data, 95)
	p99, _ := statistics.Percentile(data, 99)
	fmt.Fprintf(w, "%s\t%.0f\t%.0f\t%.0f\t%.0f\t%.0f\n", label, min, max, mean, p95, p99)
}
