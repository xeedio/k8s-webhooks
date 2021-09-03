package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	kwhhttp "github.com/slok/kubewebhook/v2/pkg/http"
	kwhlogrus "github.com/slok/kubewebhook/v2/pkg/log/logrus"
	kwhprometheus "github.com/slok/kubewebhook/v2/pkg/metrics/prometheus"
	kwhmodel "github.com/slok/kubewebhook/v2/pkg/model"
	kwhwebhook "github.com/slok/kubewebhook/v2/pkg/webhook"
	kwhmutating "github.com/slok/kubewebhook/v2/pkg/webhook/mutating"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type config struct {
	certFile            string
	keyFile             string
	imagePullSecretName string
}

func initFlags() *config {
	cfg := &config{}

	fl := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	fl.StringVar(&cfg.certFile, "tls-cert-file", "", "TLS certificate file")
	fl.StringVar(&cfg.keyFile, "tls-key-file", "", "TLS key file")
	fl.StringVar(&cfg.imagePullSecretName, "image-pull-secret-name", "cluster-docker-creds", "Image Pull Secret Name")

	_ = fl.Parse(os.Args[1:])
	return cfg
}

func run() error {
	logrusLogEntry := logrus.NewEntry(logrus.New())
	logrusLogEntry.Logger.SetLevel(logrus.DebugLevel)
	logger := kwhlogrus.NewLogrus(logrusLogEntry)

	cfg := initFlags()

	// Create mutator.
	mt := kwhmutating.MutatorFunc(func(_ context.Context, _ *kwhmodel.AdmissionReview, obj metav1.Object) (*kwhmutating.MutatorResult, error) {
		pod, ok := obj.(*corev1.Pod)
		if !ok {
			return &kwhmutating.MutatorResult{}, nil
		}

		podName := pod.GetName()

		if podName == "" {
			podName = pod.GetGenerateName()
		}

		podNamespace := pod.GetNamespace()
		if podNamespace != "" {
			podName = fmt.Sprintf("%s/%s", podNamespace, podName)
		}

		logger.Debugf("Received mutation request for pod %s", podName)

		if pod.Spec.ImagePullSecrets == nil {
			pod.Spec.ImagePullSecrets = make([]corev1.LocalObjectReference, 0)
		}

		shouldMutate := true
		for _, imagePullSecret := range pod.Spec.ImagePullSecrets {
			if imagePullSecret.Name == cfg.imagePullSecretName {
				shouldMutate = false
			}
		}

		if !shouldMutate {
			logger.Debugf("Not mutating pod %s with existing secret!", podName)
			return &kwhmutating.MutatorResult{
				MutatedObject: nil,
			}, nil
		}

		secretRef := corev1.LocalObjectReference{Name: cfg.imagePullSecretName}
		pod.Spec.ImagePullSecrets = append(pod.Spec.ImagePullSecrets, secretRef)

		logger.Debugf("Finished mutating pod %s", podName)

		return &kwhmutating.MutatorResult{
			MutatedObject: pod,
		}, nil
	})

	// Prepare metrics
	reg := prometheus.NewRegistry()
	metricsRec, err := kwhprometheus.NewRecorder(kwhprometheus.RecorderConfig{Registry: reg})
	if err != nil {
		return fmt.Errorf("could not create Prometheus metrics recorder: %w", err)
	}

	// Create webhook.
	mcfg := kwhmutating.WebhookConfig{
		ID:      "pod-add-image-pull-secret",
		Mutator: mt,
		Logger:  logger,
	}
	wh, err := kwhmutating.NewWebhook(mcfg)
	if err != nil {
		return fmt.Errorf("error creating webhook: %w", err)
	}

	// Get HTTP handler from webhook.
	whHandler, err := kwhhttp.HandlerFor(kwhhttp.HandlerConfig{
		Webhook: kwhwebhook.NewMeasuredWebhook(metricsRec, wh),
		Logger:  logger,
	})
	if err != nil {
		return fmt.Errorf("error creating webhook handler: %w", err)
	}

	errCh := make(chan error)
	// Serve webhook.
	go func() {
		logger.Infof("Listening on :8080")
		err = http.ListenAndServeTLS(":8080", cfg.certFile, cfg.keyFile, whHandler)
		if err != nil {
			errCh <- fmt.Errorf("error serving webhook: %w", err)
		}
		errCh <- nil
	}()

	// Serve metrics.
	go func() {
		logger.Infof("Listening metrics on :8081")
		err = http.ListenAndServe(":8081", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
		if err != nil {
			errCh <- fmt.Errorf("error serving webhook metrics: %w", err)
		}
		errCh <- nil
	}()

	err = <-errCh
	if err != nil {
		return err
	}

	return nil
}

func main() {
	err := run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error running app: %s", err)
		os.Exit(1)
	}
}
