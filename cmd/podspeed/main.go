package main

import (
	"flag"
	"log"
	"strings"

	podtypes "github.com/psschwei/podspeed/pkg/pod/types"
	"github.com/psschwei/podspeed/pkg/podspeed"
	corev1 "k8s.io/api/core/v1"
)

func main() {

	var (
		ns         string
		typ        string
		template   string
		podN       int
		skipDelete bool
		prepull    bool
		probe      bool
		details    bool
	)

	supportedTypes, err := podtypes.Names()
	if err != nil {
		log.Fatalln("failed to built in types: ", err)
	}

	flag.StringVar(&ns, "n", "default", "the namespace to create the pods in")
	flag.StringVar(&typ, "typ", "basic", "the type of pods to create, supported values: "+strings.Join(supportedTypes, ", "))
	flag.StringVar(&template, "template", "", "a YAML template to create pods from, can be exported from Kubernetes directly via 'kubectl get pods -oyaml', reads stdin if '-'")
	flag.IntVar(&podN, "pods", 1, "the amount of pods to create")
	flag.BoolVar(&skipDelete, "skip-delete", false, "skip removing the pods after they're ready if true")
	flag.BoolVar(&prepull, "prepull", false, "prepull all used images to all Kubernetes nodes")
	flag.BoolVar(&probe, "probe", false, "probe the pods as soon as they have an IP address and capture latency of that as well")
	flag.BoolVar(&details, "details", false, "print detailed timing information for each pod")
	flag.Parse()

	// won't be passed from CLI
	podObj := &corev1.Pod{}

	podspeed.Run(ns, podObj, typ, template, podN, skipDelete, prepull, probe, details)
}
