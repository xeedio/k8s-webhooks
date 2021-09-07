IMAGE_TAG = 0.1.3
IMAGE = xeedio/pod-image-pull-secrets-webhook:$(IMAGE_TAG)
BUILD_TARGET = pod-add-image-pull-secret

.PHONY: build-image
build-image:
	docker build -t $(IMAGE) --build-arg target=$(BUILD_TARGET) -f ./Dockerfile .

.PHONY: push-image
push-image:
	docker push $(IMAGE)
