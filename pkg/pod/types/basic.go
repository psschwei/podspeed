package types

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func init() {
	addConstructor("basic", Basic)
}

const basicApplicationImage = "docker.io/markusthoemmes/basic-500716b931f14b4a09df1ec4b4c5550d@sha256:06a71c34b05cd9d74fb9aa904ba256b525a7c39df0708b8cbbfcce923ad8af01"

func Basic(ns, name string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "test",
				Image: basicApplicationImage,
			}},
		},
	}
}
