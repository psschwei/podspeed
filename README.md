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
  -n string
    	the namespace to create the pods in (default "default")
  -pods int
    	the amount of pods to create (default 1)
  -typ string
    	the type of pods to create, either 'basic' or 'knative' (default "basic")
```

## "Roadmap"

- Parallel creation of pods
