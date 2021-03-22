package types

import (
	"sort"

	corev1 "k8s.io/api/core/v1"
)

type PodConstructor func(string, string) *corev1.Pod
type Constructors map[string]PodConstructor

var SupportedConstructors Constructors

func (c Constructors) Names() []string {
	names := make([]string, 0, len(c))
	for name := range c {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func addConstructor(name string, fn func(string, string) *corev1.Pod) {
	if SupportedConstructors == nil {
		SupportedConstructors = make(Constructors, 1)
	}
	SupportedConstructors[name] = fn
}
