FROM golang:1.16-alpine AS build-stage

ARG target

WORKDIR /go/src/github.com/xeedio/k8s-webhooks
COPY . .

RUN CGO_ENABLED=0 go build -o /bin/entrypoint --ldflags "-w -extldflags '-static'"  ./${target}

# Final image.
FROM alpine:latest

RUN apk --no-cache add ca-certificates

COPY --from=build-stage /bin/entrypoint /usr/local/bin/entrypoint

ENTRYPOINT ["/usr/local/bin/entrypoint"]
