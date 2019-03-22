package kubernetes

// filteredNamespaceExists checks if namespace exists in this cluster
// according to any `namespace_labels` plugin configuration specified.
// Returns true even for namespaces not exposed by plugin configuration,
// see namespaceExposed.
func (k *Kubernetes) filteredNamespaceExists(namespace string) bool {
	ns, err := k.APIConn.GetNamespaceByName(namespace)
	if err != nil {
		return false
	}
	return ns.ObjectMeta.Name == namespace
}

// configuredNamespace returns true when the namespace is exposed through the plugin
// `namespaces` configuration.
func (k *Kubernetes) configuredNamespace(namespace string) bool {
	_, ok := k.Namespaces[namespace]
	if len(k.Namespaces) > 0 && !ok {
		return false
	}
	return true
}

func (k *Kubernetes) namespaceExposed(namespace string) bool {
	return k.configuredNamespace(namespace) && k.filteredNamespaceExists(namespace)
}
