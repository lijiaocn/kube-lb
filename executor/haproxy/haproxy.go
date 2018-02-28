//create: 2018/01/04 11:09:11 Change: 2018/02/28 15:09:26 lijiaocn@foxmail.com
package haproxy

import (
	"errors"
	"github.com/golang/glog"
	L "github.com/lijiaocn/kube-lb/listener"
	"os"
	"strconv"
	"strings"
	"text/template"
)

type Haproxy struct {
	defaultIP string
	template  string
	result    string
}

func NewHaproxy(defaultip, template, result string) *Haproxy {
	glog.V(2).Infof("create new haproxy: %s %s %s", defaultip, template, result)
	haproxy := Haproxy{
		defaultIP: defaultip,
		template:  template,
		result:    result,
	}
	glog.V(3).Infof("create new haproxy: %s %s %s => %s", defaultip, template, result, haproxy)
	return &haproxy
}

//Don't support Self-defined BindIP, ignore BindIP
//Don't support UDP now

func (self *Haproxy) Convert2Config(ls []L.Listener) error {
	glog.V(2).Infof("convert listeners to haproxy config")
	glog.V(3).Infof("convert listeners to haproxy config, listners => %s", ls)

	if len(ls) == 0 {
		return errors.New("there is no listener")
	}
	if err := valid(ls); err != nil {
		return err
	}

	f, err := os.OpenFile(self.result, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}

	t, err := template.ParseFiles(self.template)
	if err != nil {
		return err
	}

	ha, err := convert(ls)
	if err != nil {
		return err
	}

	if err := t.Execute(f, ha); err != nil {
		return err
	}

	if err := f.Close(); err != nil {
		return err
	}
	return nil
}

func collect_common_info(key string, ls []*L.Listener) (certs []string, backends []*Backend, err error) {
	glog.V(2).Infof("collect common info: %s", key)
	err = nil
	for _, l := range ls {
		if l.CertFile != "" {
			certs = append(certs, l.CertFile)
		}
		for _, sg := range l.ServerGroups {
			if len(sg.Servers) == 0 {
				continue
			}
			backend := &Backend{
				Mode:      "tcp",
				Name:      key + "-" + l.Name + "-" + sg.Name,
				Algorithm: sg.Algorithm,
			}
			if len(sg.L7.Hosts) != 0 {
				backend.Mode = "http"
				backend.ACL = " hdr(host) -i "
				backend.Hosts = sg.L7.Hosts
				for _, host := range sg.L7.Hosts {
					backend.ACL = backend.ACL + " " + host
				}
			}
			for _, s := range sg.Servers {
				server := &Server{
					Name: s.Name,
					Addr: s.Addr,
					Port: sg.Port,
				}
				if s.HealthCheck {
					server.Option += " check"
				}
				if s.MaxConn > 0 {
					server.Option += " maxconn " + strconv.Itoa(s.MaxConn)
				}
				backend.Servers = append(backend.Servers, server)
			}
			backends = append(backends, backend)
		}
	}
	glog.V(3).Infof("collect common info: %s certs=>%s backends=>%s", key, certs, backends)
	return
}

func collect_tcp_info(key string, ls []*L.Listener) (backends []*Backend, err error) {
	glog.V(2).Infof("collect tcp info:  %s\n", key)
	_, backends, err = collect_common_info(key, ls)
	return
}

func collect_http_info(key string, ls []*L.Listener) (backends []*Backend, err error) {
	glog.V(2).Infof("collect http info:  %s\n", key)
	_, backends, err = collect_common_info(key, ls)
	return backends, err
}

func collect_ssl_info(key string, ls []*L.Listener) (certs []string, backends []*Backend, err error) {
	glog.V(2).Infof("collect ssl info:  %s\n", key)
	certs, backends, err = collect_common_info(key, ls)
	return
}

func collect_https_info(key string, ls []*L.Listener) (certs []string, backends []*Backend, err error) {
	glog.V(2).Infof("collect https info:  %s\n", key)
	certs, backends, err = collect_common_info(key, ls)
	return
}

func insert_tcp_listener(ha *HaproxyConfig, key string, ls []*L.Listener) error {
	switch ls[0].L7Proto {
	case "":
		front_tcp := &FrontendTCP{
			Name:     key,
			BindIP:   ls[0].BindIP,
			BindPort: ls[0].BindPort,
		}
		if backends, err := collect_tcp_info(key, ls); err == nil {
			if len(backends) == 0 {
				return nil
			}
			front_tcp.Backend = backends
			ha.Backend = append(ha.Backend, backends...)
		} else {
			return err
		}
		ha.FrontendTCP = append(ha.FrontendTCP, front_tcp)
	case L.PROTO_SSL:
		front_ssl := &FrontendSSL{
			Name:     key,
			BindIP:   ls[0].BindIP,
			BindPort: ls[0].BindPort,
		}
		if certs, backends, err := collect_ssl_info(key, ls); err == nil {
			if len(backends) == 0 {
				return nil
			}
			front_ssl.CertFiles = certs
			front_ssl.Backend = backends
			ha.Backend = append(ha.Backend, backends...)
		} else {
			return err
		}
		ha.FrontendSSL = append(ha.FrontendSSL, front_ssl)
	case L.PROTO_HTTP:
		front_http := &FrontendHTTP{
			Name:     key,
			BindIP:   ls[0].BindIP,
			BindPort: ls[0].BindPort,
		}
		if backends, err := collect_http_info(key, ls); err == nil {
			if len(backends) == 0 {
				return nil
			}
			front_http.Backend = backends
			ha.Backend = append(ha.Backend, backends...)
		} else {
			return err
		}
		ha.FrontendHTTP = append(ha.FrontendHTTP, front_http)
	case L.PROTO_HTTPS:
		front_https := &FrontendHTTPS{
			Name:     key,
			BindIP:   ls[0].BindIP,
			BindPort: ls[0].BindPort,
		}
		if certs, backends, err := collect_https_info(key, ls); err == nil {
			if len(backends) == 0 {
				return nil
			}
			front_https.CertFiles = certs
			front_https.Backend = backends
			ha.Backend = append(ha.Backend, backends...)
		} else {
			return err
		}
		ha.FrontendHTTPS = append(ha.FrontendHTTPS, front_https)
	default:
		return errors.New("unsupport l7 proto: " + ls[0].L7Proto)
	}
	return nil
}

func insert_udp_listener(ha *HaproxyConfig, key string, ls []*L.Listener) error {
	return errors.New("udp is not supported")
}

func divide_listener(ls []L.Listener) (map[string][]*L.Listener, error) {
	glog.V(2).Infof("devide listeners into groups")
	groups := make(map[string][]*L.Listener)
	for i, l := range ls {
		glog.V(2).Infof("divide listener: %s", l.Name)
		key := l.L4Proto + "_" + strings.Replace(l.BindIP, ".", "_", -1) + "_" + l.BindPort
		if g, ok := groups[key]; ok {
			if g[0].L7Proto != l.L7Proto {
				return nil, errors.New("don't support more than one L7 proto on single L4 port: " + strconv.Itoa(i))
			}
			groups[key] = append(g, &ls[i])
		} else {
			var g []*L.Listener
			groups[key] = append(g, &ls[i])
		}
	}
	glog.V(3).Infof("devide listeners into groups =>%s", groups)
	return groups, nil
}

func convert(ls []L.Listener) (*HaproxyConfig, error) {
	glog.V(2).Infof("start convert listeners to haproxy config")
	var ha HaproxyConfig

	groups, err := divide_listener(ls)
	if err != nil {
		return nil, err
	}

	for key, ls := range groups {
		switch ls[0].L4Proto {
		case L.PROTO_TCP:
			glog.V(2).Infof("parse group: %s\n", key)
			if err := insert_tcp_listener(&ha, key, ls); err != nil {
				return nil, err
			}
		case L.PROTO_UDP:
			if err := insert_udp_listener(&ha, key, ls); err != nil {
				return nil, err
			}
		default:
			return nil, errors.New("unsupport l4 proto: " + ls[0].L4Proto)
		}
	}
	glog.V(3).Infof("convert result haproxy config => %s", ha)
	return &ha, nil
}
