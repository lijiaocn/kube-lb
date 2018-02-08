#PREFIX=harbor.enncloud.cn/paas
PREFIX=reg.enncloud.cn/paas
TAG="v1.0.3"
IMAGE=${PREFIX}/kube-lb:${TAG}

.PHONY: build image push

build:
	go build
image:
	cp kube-lb images/tenx/
	docker build -t ${IMAGE} images/tenx/
	rm images/tenx/kube-lb
push:
	docker push ${IMAGE}
clean:
	rm kube-lb
