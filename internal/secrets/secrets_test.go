package secrets_test

import (
	"path/filepath"
	"testing"

	"github.com/christianb93/bitcoin-controller/internal/secrets"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// TestCredentialsForSecretIntegration tests retrieving a secret with credentials
func TestCredentialForSecretIntegration(t *testing.T) {
	home := homedir.HomeDir()
	kubeconfig := filepath.Join(home, ".kube", "config")
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		panic("Could not get configuration file")
	}
	// Create BitcoinNetwork client set
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic("Could not create BitcoinNetwork clientset")
	}
	mySecret := &v1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-secret",
			Namespace: "default",
		},
		StringData: map[string]string{"BC_RPC_USER": "john", "BC_RPC_PASSWORD": "secret"},
	}
	//  First create a secret
	_, err = client.CoreV1().Secrets("default").Create(mySecret)
	if err != nil {
		t.Errorf("Got unexpected error %s\n", err)
		t.FailNow()
	}
	// Defer cleanup
	defer func() {
		client.CoreV1().Secrets("default").Delete("my-secret", &metav1.DeleteOptions{})
	}()
	// Now try to read secret
	user, password, err := secrets.CredentialsForSecret("my-secret", "default", client)
	if err != nil {
		t.Errorf("Got unexpected error %s\n", err)
		t.FailNow()
	}
	if user != "john" || password != "secret" {
		t.Errorf("Got unexpected credentials %s:%s\n", user, password)
	}
}
