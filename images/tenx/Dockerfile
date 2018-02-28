FROM alpine:latest

RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/' /etc/apk/repositories
RUN apk update && apk upgrade
RUN apk add --no-cache tzdata supervisor haproxy
RUN ln -snf /usr/share/zoneinfo/Asia/Shanghai /etc/localtime

RUN mkdir -p /etc/sslkeys
RUN mkdir -p /run/haproxy

ADD ./haproxy.tpl /etc/default/hafolder/haproxy.tpl
ADD ./default.pem /etc/sslkeys/default.pem
ADD ./503.http /etc/haproxy/errors/503.http

ADD ./kube-lb /kube-lb

ADD ./update.sh /update.sh
ADD ./port_reused.sh  /port_reused.sh
ADD ./run.sh /run.sh
CMD ["/run.sh"]
