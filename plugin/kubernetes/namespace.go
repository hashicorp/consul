package kubernetes

// namespace checks if namespace n exists in this cluster. This returns true
// even for non exposed namespaces, see namespaceExposed.
func (k *Kubernetes) namespace(n string) bool {
	ns, err := k.APIConn.GetNamespaceByName(n)
	if err != nil {
		return false
	}
	return ns.ObjectMeta.Name == n
}

// namespaceExposed returns true when the namespace is exposed.
func (k *Kubernetes) namespaceExposed(namespace string) bool {
	_, ok := k.Namespaces[namespace]
	if len(k.Namespaces) > 0 && !ok {
		return false
	}
	return true
}
