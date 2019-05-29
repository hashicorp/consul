package kubernetes

import (
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

func serviceWatchFunc(c kubernetes.Interface, ns string, s labels.Selector) func(options meta.ListOptions) (watch.Interface, error) {
	return func(options meta.ListOptions) (watch.Interface, error) {
		if s != nil {
			options.LabelSelector = s.String()
		}
		w, err := c.CoreV1().Services(ns).Watch(options)
		return w, err
	}
}

func podWatchFunc(c kubernetes.Interface, ns string, s labels.Selector) func(options meta.ListOptions) (watch.Interface, error) {
	return func(options meta.ListOptions) (watch.Interface, error) {
		if s != nil {
			options.LabelSelector = s.String()
		}
		if len(options.FieldSelector) > 0 {
			options.FieldSelector = options.FieldSelector + ","
		}
		options.FieldSelector = options.FieldSelector + "status.phase!=Succeeded,status.phase!=Failed,status.phase!=Unknown"
		w, err := c.CoreV1().Pods(ns).Watch(options)
		return w, err
	}
}

func endpointsWatchFunc(c kubernetes.Interface, ns string, s labels.Selector) func(options meta.ListOptions) (watch.Interface, error) {
	return func(options meta.ListOptions) (watch.Interface, error) {
		if s != nil {
			options.LabelSelector = s.String()
		}
		w, err := c.CoreV1().Endpoints(ns).Watch(options)
		return w, err
	}
}

func namespaceWatchFunc(c kubernetes.Interface, s labels.Selector) func(options meta.ListOptions) (watch.Interface, error) {
	return func(options meta.ListOptions) (watch.Interface, error) {
		if s != nil {
			options.LabelSelector = s.String()
		}
		w, err := c.CoreV1().Namespaces().Watch(options)
		return w, err
	}
}
