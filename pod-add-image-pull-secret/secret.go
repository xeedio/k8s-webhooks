package main

import (
	"context"
	"os"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// API client for managing secrets
var kubeClientSet *kubernetes.Clientset

var secretCache map[string]bool

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func kubeConfigFromPath(kubeConfigPath string) *rest.Config {
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		panic(err.Error())
	}
	return config
}

func getClusterConfig() *rest.Config {
	kubeConfigPath := os.Getenv("KUBECONFIG")
	if fileExists(kubeConfigPath) {
		logger.Infof("Valid kube config from %s", kubeConfigPath)
		return kubeConfigFromPath(kubeConfigPath)
	}

	kubeConfigPath = os.Getenv("HOME") + "/.kube/config"
	if fileExists(kubeConfigPath) {
		logger.Infof("Valid kube config from %s", kubeConfigPath)
		return kubeConfigFromPath(kubeConfigPath)
	}

	// default in-cluster config
	return kubeConfigFromPath("")
}

func secretExists(namespace, secretName string) bool {
	if _, found := secretCache[namespace]; found {
		logger.Debugf("Cached secret %s/%s exists!", namespace, secretName)
		return true
	}

	secret, err := kubeClientSet.CoreV1().Secrets(namespace).Get(context.TODO(), secretName, metav1.GetOptions{})
	if err == nil {
		logger.Debugf("Secret %s/%s exists!", secret.GetNamespace(), secret.GetName())
		secretCache[namespace] = true
		return true
	}

	logger.Infof("Secret %s/%s does not exist!", namespace, secretName)

	return false
}

func saveSecret(namespace string, sourceSecret *corev1.Secret) error {
	if secretExists(namespace, sourceSecret.GetName()) {
		return nil
	}

	targetSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sourceSecret.GetName(),
			Namespace: namespace,
			Annotations: map[string]string{
				"created-by-webhook": "true",
			},
		},
		Type: corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{
			corev1.DockerConfigJsonKey: sourceSecret.Data[corev1.DockerConfigJsonKey],
		},
	}

	_, err := kubeClientSet.CoreV1().Secrets(namespace).Create(context.TODO(), &targetSecret, metav1.CreateOptions{})
	return err
}

func getSecret(namespace string, secretName string) (*corev1.Secret, error) {
	secret, err := kubeClientSet.CoreV1().Secrets(namespace).Get(context.TODO(), secretName, metav1.GetOptions{})
	return secret, err
}
