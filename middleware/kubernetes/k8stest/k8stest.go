package k8stest

import (
	"net/http"
)

// checkKubernetesRunning performs a basic
func CheckKubernetesRunning() bool {
	_, err := http.Get("http://localhost:8080/api/v1")
	return err == nil
}
