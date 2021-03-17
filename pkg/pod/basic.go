package pod

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const basicApplicationImage = "docker.io/markusthoemmes/basic-500716b931f14b4a09df1ec4b4c5550d@sha256:f053b2bfef7f5bd32c678d64b0d4cd785004ed93b3ba512cdd8500212cabbb74"

func Basic(ns, name string) *corev1.Pod {
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
