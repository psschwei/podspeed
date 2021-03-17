package pod

import (
	corev1 "k8s.io/api/core/v1"
)

func IsConditionTrue(p *corev1.Pod, condType corev1.PodConditionType) bool {
	for _, cond := range p.Status.Conditions {
		if cond.Type == condType && cond.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}
