package main

import (
	"fmt"
	"testing"
)

func TestGetSecert(t *testing.T) {
	initClient()

	secretName := "cluster-docker-creds"
	secret, err := getSecret(operatorNamespace, secretName)
	if err != nil {
		t.Errorf("Get secret err: %v", err)
	}
	fmt.Printf("Got secret: %+v\n", secret)
	if err := saveSecret("default", secret); err != nil {
		t.Errorf("Save secret err: %v", err)
	}

	if err := saveSecret("default", secret); err != nil {
		t.Errorf("Save secret err: %v", err)
	}
}
