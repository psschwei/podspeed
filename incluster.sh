#!/bin/bash

ko apply -f job.yaml > /dev/null 2>&1
kubectl wait --for=condition=complete job/podspeed > /dev/null 2>&1
kubectl logs job/podspeed -f
kubectl delete job podspeed > /dev/null 2>&1