# Podspeed

`podspeed` is a tool to benchmark pod-startup-time on Kubernetes clusters. 

**Warning:** All timing related data is currently harvested by watching the created pods
and storing the wall-clock-time, when an event happens and the pod goes into a certain
state (i.e. the `Ready` condition becomes true). This might not be a super accurate
measure, but it should be good enough to at least a rough idea on pod-startup-time
across different use-cases and clusters.

## Usage

```
$ podspeed -h
  -details
    	print detailed timing information for each pod
  -n string
    	the namespace to create the pods in (default "default")
  -pods int
    	the amount of pods to create (default 1)
  -prepull
    	prepull all used images to all Kubernetes nodes
  -probe
    	probe the pods as soon as they have an IP address and capture latency of that as well
  -skip-delete
    	skip removing the pods after they're ready if true
  -template string
    	a YAML template to create pods from, can be exported from Kubernetes directly via 'kubectl get pods -oyaml', reads stdin if '-'
  -typ string
    	the type of pods to create, supported values: basic, basic-no-volume, knative-head (default "basic")
```

## "Roadmap"

- Parallel creation of pods
