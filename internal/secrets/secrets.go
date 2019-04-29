package secrets

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"
)

const (
	userKey     = "BC_RPC_USER"
	passwordKey = "BC_RPC_PASSWORD"
)

// Default credentials
const (
	DefaultRPCUser     = "user"
	DefaultRPCPassword = "password"
)

// CredentialsForSecret looks up credentials in a secret. If the secret does not exist,
// or any error occurs, the default credentials are returned
// If the secret Name is the empty string, the standard credentials are returned
func CredentialsForSecret(secretName string, secretNamespace string, clientset *kubernetes.Clientset) (user string, password string, err error) {
	if secretName == "" {
		return DefaultRPCUser, DefaultRPCPassword, nil
	}
	// Try to get secret
	secret, err := clientset.CoreV1().Secrets(secretNamespace).Get(secretName, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("Could not get secret, error is %s\n", err)
		return DefaultRPCUser, DefaultRPCPassword, err
	}
	data, ok := secret.Data[userKey]
	if !ok {
		return DefaultRPCUser, DefaultRPCPassword, fmt.Errorf("Secret does not contain key %s", userKey)
	}
	user = string(data)
	data, ok = secret.Data[passwordKey]
	password = string(data)
	if !ok {
		return DefaultRPCUser, DefaultRPCPassword, fmt.Errorf("Secret does not contain key %s", passwordKey)
	}
	return user, password, nil
}
