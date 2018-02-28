PREFIX=harbor.enncloud.cn/paas
#PREFIX=reg.enncloud.cn/paas
TAG="v1.0.6"
IMAGE=${PREFIX}/kube-lb:${TAG}

.PHONY: build image push

build:
	CGO_ENABLED=0 go build
image:
	cp kube-lb images/tenx/
	docker build -t ${IMAGE} images/tenx/
	rm images/tenx/kube-lb
push:
	docker push ${IMAGE}
clean:
	rm kube-lb
