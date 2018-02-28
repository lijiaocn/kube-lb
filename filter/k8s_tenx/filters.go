//create: 2018/01/18 16:43:46 Change: 2018/02/28 15:51:30 lijiaocn@foxmail.com
package k8s_tenx

import (
	"errors"
	"fmt"
	"github.com/golang/glog"
	"github.com/lijiaocn/kube-lb/filter"
	L "github.com/lijiaocn/kube-lb/listener"
	"k8s.io/api/core/v1"
	"reflect"
	"strconv"
	"strings"
)

func UpdateCheck(action string, ls *L.Listeners, objs ...interface{}) int {
	if len(objs) != 2 {
		glog.Errorf("update operation need 2 obj")
		return filter.IGNORE
	}

	if check("call by update: old obj", ls, objs[0]) == filter.IGNORE &&
		check("call by update: new obj", ls, objs[1]) == filter.IGNORE {
		return filter.IGNORE
	}
	return filter.CONTINUE
}

func AddCheck(action string, ls *L.Listeners, objs ...interface{}) int {
	return check("call by add", ls, objs[0])
}

func DeleteCheck(action string, ls *L.Listeners, objs ...interface{}) int {
	return check("call by delete", ls, objs[0])
}

func check(action string, ls *L.Listeners, obj interface{}) int {

	if svc, ok := obj.(*v1.Service); ok {
		if svc.Namespace == "kube-system" || svc.Namespace == "default" {
			glog.V(1).Infof("ingore service %s %s: in kube-system/default namespace (%s)",
				svc.Namespace, svc.Name, action)
			return filter.IGNORE
		}
		if group, ok := svc.Annotations["system/lbgroup"]; ok {
			if !isMyGroup(group) { //not in my group
				glog.V(1).Infof("ingore service %s %s not in my group: %s != %s (%s)",
					svc.Namespace, svc.Name, group, localConfig.group.Id, action)
				return filter.IGNORE
			} else { // is in my group, continue
				return filter.CONTINUE
			}
		} else { //group is not set
			glog.V(1).Infof("ingore service %s %s : system/lbgroup is not set (%s)", svc.Namespace, svc.Name, action)
			return filter.IGNORE
		}
		glog.V(1).Infof("accept service %s %s (%s)", svc.Namespace, svc.Name, action)
	}

	if pod, ok := obj.(*v1.Pod); ok {
		if pod.Namespace == "kube-system" || pod.Namespace == "default" {
			glog.V(1).Infof("ingore pod %s %s: in kube-system/default namespace (%s)",
				pod.Namespace, pod.Name, action)
			return filter.IGNORE
		}
		if pod.Status.Phase != v1.PodRunning {
			glog.V(1).Infof("ingore pod %s %s: not running(%s) (%s)",
				pod.Namespace, pod.Name, pod.Status.Phase, action)
			return filter.IGNORE
		}
		label_job := "system/jobType"
		if v, _ := pod.Labels[label_job]; v == "devflows" {
			glog.V(1).Infof("ingore pod %s %s: is a job %s (%s)", pod.Namespace, pod.Name, v, action)
			return filter.IGNORE
		}

		ready := false
		for _, v := range pod.Status.Conditions {
			if v.Type == "Ready" && string(v.Status) == "True" {
				ready = true
			}
		}
		if !ready {
			glog.V(1).Infof("ingore pod %s %s: not ready (%s)", pod.Namespace, pod.Name, action)
			return filter.IGNORE
		}
	}
	return filter.CONTINUE
}

func deletePod(pod *v1.Pod, ls *L.Listeners) error {
	key, err := serverGroupKey(pod)
	if err != nil {
		return err
	}

	ls.Rwlock.Lock()
	defer ls.Rwlock.Unlock()
	if ss, ok := ls.Servers[key]; ok {
		if _, ok := (*ss)[pod.Name]; ok {
			delete(*ss, pod.Name)
			glog.V(1).Infof("clear cache in servers: %s %s", key, pod.Name)
			if len(*ss) == 0 {
				delete(ls.Servers, key)
				glog.V(1).Infof("clear cache in servers: %s", key)
			}
			return nil
		}
	}
	return errors.New(fmt.Sprintf("the pod need to be deleted doesn't exist: %s", key))
}

func addOrUpdatePod(pod *v1.Pod, ls *L.Listeners) error {
	label_lb_maxconn := "lb/maxconn"
	label_lb_check := "lb/check"

	if pod.Status.Phase != v1.PodRunning {
		return errors.New(fmt.Sprintf("pod is not running(%s): %s %s", pod.Status.Phase, pod.Namespace, pod.Name))
	}

	if pod.Status.PodIP == "" {
		return errors.New(fmt.Sprintf("pod ip is unkown: %s %s", pod.Namespace, pod.Name))
	}

	key, err := serverGroupKey(pod)
	if err != nil {
		return err
	}

	max := 2000
	if n, ok := pod.Labels[label_lb_maxconn]; ok {
		if m, err := strconv.Atoi(n); err != nil {
			return errors.New("pod 's label " + label_lb_maxconn + " is wrong: " + err.Error())
		} else {
			max = m
		}
	}

	check := true
	if n, ok := pod.Labels[label_lb_check]; ok {
		if m, err := strconv.ParseBool(n); err != nil {
			return errors.New("pod 's label " + label_lb_check + " is wrong: " + err.Error())
		} else {
			check = m
		}
	}

	s := L.Server{
		Name:        pod.Name,
		Addr:        pod.Status.PodIP,
		MaxConn:     max,
		HealthCheck: check,
	}

	ls.Rwlock.Lock()
	defer ls.Rwlock.Unlock()
	if ss, ok := ls.Servers[key]; !ok {
		tmp := make(map[string]L.Server, 0)
		ss = &tmp
		(*ss)[s.Name] = s
		ls.Servers[key] = ss
	} else {
		if v, ok := (*ss)[s.Name]; ok {
			if reflect.DeepEqual(s, v) {
				return errors.New(fmt.Sprintf("this pod existed: %s %s %s", pod.Namespace, pod.Name, s))
			} else {
				delete(*ss, s.Name)
			}
		}
		(*ss)[s.Name] = s
	}
	return nil
}

func serverGroupKey(pod *v1.Pod) (string, error) {

	label_lb := "name"
	if v, ok := pod.Labels[label_lb]; ok {
		key := pod.Namespace + "-" + v + "-" + "default"
		return key, nil
	}

	label_stateful := "enncloud.com/statefulSetName"
	if v, ok := pod.Labels[label_stateful]; ok {
		key := pod.Namespace + "-" + v + "-" + "default"
		return key, nil
	}

	label_petset := "tenxcloud.com/petsetName"
	if v, ok := pod.Labels[label_petset]; ok {
		key := pod.Namespace + "-" + v + "-" + "default"
		return key, nil
	}

	label_app := "tenxcloud.com/appName"
	if v, ok := pod.Labels[label_app]; ok {
		key := pod.Namespace + "-" + v + "-" + "default"
		return key, nil
	}

	label_svc := "tenxcloud.com/svcName"
	if v, ok := pod.Labels[label_svc]; ok {
		key := pod.Namespace + "-" + v + "-" + "default"
		return key, nil
	}

	return "", errors.New(fmt.Sprintf("none of the labels is set on pod %s %s: %s %s %s %s %s",
		pod.Namespace, pod.Name, label_lb, label_stateful, label_petset, label_app, label_svc))
}

func parserListeners(svc *v1.Service) ([]L.Listener, error) {
	anno_ports := "tenxcloud.com/schemaPortname"
	anno_https := "tenxcloud.com/https"
	anno_bindip := "lb/bindip"
	newListeners := make([]L.Listener, 0)

	var name string = ""

	label_lb := "name"
	if v, ok := svc.Spec.Selector[label_lb]; ok {
		name = v
	}

	label_svcName := "tenxcloud.com/svcName"
	if v, ok := svc.Spec.Selector[label_svcName]; ok {
		name = v
	}

	label_appName := "tenxcloud.com/appName"
	if v, ok := svc.Spec.Selector[label_appName]; ok {
		name = v
	}

	label_petsetName := "tenxcloud.com/petsetName"
	if v, ok := svc.Spec.Selector[label_petsetName]; ok {
		name = v
	}

	label_stateful := "enncloud.com/statefulSetName"
	if v, ok := svc.Spec.Selector[label_stateful]; ok {
		name = v
	}

	if name == "" {
		return newListeners, errors.New(fmt.Sprintf("none of these selector is set on %s %s: %s %s %s %s %s",
			svc.Namespace, svc.Name, label_lb, label_stateful, label_petsetName, label_appName, label_svcName))
	}

	schemaPorts, ok := svc.Annotations[anno_ports]
	if !ok {
		return newListeners, errors.New(fmt.Sprintf("service ports is not set %s %s", svc.Namespace, svc.Name))
	}

	enable_https, ok := svc.Annotations[anno_https] //"true"
	if !ok {
		enable_https = "false"
	}

	bindip := "0.0.0.0"
	if v, ok := svc.Annotations[anno_bindip]; ok {
		bindip = v
	}

	exportPorts := strings.Split(schemaPorts, ",")
	for _, eport := range exportPorts {
		l := L.Listener{
			Encryption: false,
			BindIP:     bindip,
		}
		infos := strings.Split(eport, "/")
		len := len(infos)
		if len < 2 {
			return newListeners, errors.New(fmt.Sprintf("%s %s's annotation %s's value is invalid: %s",
				svc.Namespace, svc.Name, anno_ports, eport))
		}
		switch strings.ToLower(infos[1]) {
		case "udp":
			if len == 2 {
				return newListeners, errors.New(fmt.Sprintf("%s %s's annotation %s's value is invalid: %s",
					svc.Namespace, svc.Name, anno_ports, eport))
			}
			l.BindPort = infos[2]
			l.L4Proto = L.PROTO_UDP
		case "tcp":
			if len == 2 {
				return newListeners, errors.New(fmt.Sprintf("%s %s's annotation %s's value is invalid: %s",
					svc.Namespace, svc.Name, anno_ports, eport))
			}
			l.BindPort = infos[2]
			l.L4Proto = L.PROTO_TCP
		case "http":
			if strings.ToLower(enable_https) == "true" {
				l.CertFile = svc.Namespace + ".tenxsep" + "." + svc.Name
				l.Encryption = true
				l.BindPort = "443"
				l.L4Proto = L.PROTO_TCP
				l.L7Proto = L.PROTO_HTTPS
			} else {
				l.BindPort = "80"
				l.L4Proto = L.PROTO_TCP
				l.L7Proto = L.PROTO_HTTP
			}
		case "https":
			l.CertFile = svc.Namespace + ".tenxsep" + svc.Name
			l.Encryption = true
			l.BindPort = "443"
			l.L4Proto = L.PROTO_TCP
			l.L7Proto = L.PROTO_HTTPS
		}
		l.Name = svc.Namespace + "-" + name
		isL7 := l.L7Proto == L.PROTO_HTTPS || l.L7Proto == L.PROTO_HTTP
		if sgs, err := parseServerGroup(svc, infos[0], isL7); err != nil {
			return newListeners, err
		} else {
			l.ServerGroups = append(l.ServerGroups, sgs...)
		}
		newListeners = append(newListeners, l)
	}
	return newListeners, nil
}

//just support one server group now
func parseServerGroup(svc *v1.Service, portName string, isl7 bool) ([]*L.ServerGroup, error) {
	anno_domains := "binding_domains"
	anno_sticky := "lb/sticky"
	anno_algorithm := "lb/algorithm"

	sticky := L.STI_COOKIE_POST
	if s, ok := svc.Annotations[anno_sticky]; ok {
		sticky = s
	}

	algorithm := L.ALG_RR
	if s, ok := svc.Annotations[anno_algorithm]; ok {
		algorithm = s
	}

	sgs := make([]*L.ServerGroup, 0)
	sg := L.ServerGroup{
		Name:      "default",
		Sticky:    sticky,
		Algorithm: algorithm,
	}
	if isl7 {
		default_domain := svc.Name + "-" + svc.Namespace + "." + strings.TrimSpace(localConfig.group.Domain)
		sg.L7.Hosts = append(make([]string, 0), default_domain)
		if d, ok := svc.Annotations[anno_domains]; ok {
			hosts := strings.Split(d, ",")
			sg.L7.Hosts = append(sg.L7.Hosts, hosts...)
		}
	}

	for _, p := range svc.Spec.Ports {
		if p.Name == portName {
			sg.Port = strconv.Itoa(int(p.Port))
			break
		}
	}

	sgs = append(sgs, &sg)
	return sgs, nil
}

func deleteService(svc *v1.Service, ls *L.Listeners) error {
	newls, err := parserListeners(svc)
	if err != nil {
		return err
	}

	var update bool = false

	for _, l := range newls {
		key := l.Key()
		ls.Rwlock.Lock()
		if _, ok := ls.Listeners[key]; ok {
			delete(ls.Listeners, key)
			update = true
			glog.V(1).Infof("clear cache in listeners: %s", key)
		}
		/* pod is absolutely isolated from service, so we don't delete servers at here
		for _, sg := range l.ServerGroups {
			key := l.Name + "-" + sg.Name
			if _, ok := ls.Servers[key]; ok {
				delete(ls.Servers, key)
				update = true
				glog.V(1).Infof("clear cache in servers: %s", key)
			}
		}
		*/
		ls.Rwlock.Unlock()
	}
	if update {
		return nil
	}
	return errors.New(fmt.Sprintf("no listener is deleted: %s %s", svc.Namespace, svc.Name))
}

func addOrUpdateService(svc *v1.Service, ls *L.Listeners) error {
	newls, err := parserListeners(svc)
	if err != nil {
		return err
	}

	for _, l := range newls {
		key := l.Key()
		ls.Rwlock.Lock()
		if v, ok := ls.Listeners[key]; ok {
			if reflect.DeepEqual(v, l) {
				ls.Rwlock.Unlock()
				return errors.New(fmt.Sprintf("service exists: %s", key))
			} else {
				delete(ls.Listeners, key)
			}
		}
		ls.Listeners[key] = l
		ls.Rwlock.Unlock()
	}
	return nil
}

func Add(action string, ls *L.Listeners, objs ...interface{}) int {
	obj := objs[0]
	if pod, ok := obj.(*v1.Pod); ok {
		glog.V(1).Infof("add pod: %s %s (%s)", pod.Namespace, pod.Name, action)
		if err := addOrUpdatePod(pod, ls); err != nil {
			glog.Errorf("%s", err.Error())
		} else {
			return filter.UPDATE
		}
	}

	if svc, ok := obj.(*v1.Service); ok {
		glog.V(1).Infof("add service: %s %s (%s)", svc.Namespace, svc.Name, action)
		if err := addOrUpdateService(svc, ls); err != nil {
			glog.Errorf("%s", err.Error())
		} else {
			return filter.UPDATE
		}
	}

	return filter.NOUPDATE
}

func Delete(action string, ls *L.Listeners, objs ...interface{}) int {
	obj := objs[0]
	if pod, ok := obj.(*v1.Pod); ok {
		glog.V(1).Infof("delete pod: %s %s (%s)", pod.Namespace, pod.Name, action)
		if err := deletePod(pod, ls); err != nil {
			glog.V(1).Infof("%s", err.Error())
			return filter.NOUPDATE
		} else {
			return filter.UPDATE
		}
	}

	if svc, ok := obj.(*v1.Service); ok {
		glog.V(1).Infof("delete service: %s %s (%s)", svc.Namespace, svc.Name, action)
		if err := deleteService(svc, ls); err != nil {
			glog.V(1).Infof("%s", err.Error())
			return filter.NOUPDATE
		} else {
			return filter.UPDATE
		}
	}

	return filter.NOUPDATE
}

func Update(action string, ls *L.Listeners, objs ...interface{}) int {
	if len(objs) != 2 {
		glog.Errorf("Update: the objs's length should be 2")
	}

	ret1 := Delete("call by update", ls, objs[0]) // delete old obj
	ret2 := Add("call by update", ls, objs[1])    // add new obj

	if ret1 == filter.UPDATE || ret2 == filter.UPDATE {
		return filter.UPDATE
	}

	return filter.NOUPDATE
}

func DEBUG_ADD(action string, ls *L.Listeners, objs ...interface{}) int {
	if len(objs) != 1 {
		glog.Errorf("debug_ADD: objs's length is not 1")
		return filter.IGNORE
	}
	obj := objs[0]
	if pod, ok := obj.(*v1.Pod); ok {
		glog.V(1).Infof("ADD pod: %s %s", pod.Namespace, pod.Name)
		return filter.CONTINUE
	}
	if svc, ok := obj.(*v1.Service); ok {
		glog.V(1).Infof("ADD service: %s %s", svc.Namespace, svc.Name)
		return filter.CONTINUE
	}
	if ns, ok := obj.(*v1.Namespace); ok {
		glog.V(1).Infof("ADD namespace: %s", ns.Name)
		return filter.CONTINUE
	}
	glog.Errorf("debug_ADD: obj's type is unknown")
	return filter.IGNORE
}

func DEBUG_DELETE(action string, ls *L.Listeners, objs ...interface{}) int {
	if len(objs) != 1 {
		glog.Errorf("debug_DELETE: objs's length is not 1")
		return filter.IGNORE
	}
	obj := objs[0]
	if pod, ok := obj.(*v1.Pod); ok {
		glog.V(1).Infof("DELETE pod: %s %s", pod.Namespace, pod.Name)
		return filter.CONTINUE
	}
	if svc, ok := obj.(*v1.Service); ok {
		glog.V(1).Infof("DELETE service: %s %s", svc.Namespace, svc.Name)
		return filter.CONTINUE
	}
	if ns, ok := obj.(*v1.Namespace); ok {
		glog.V(1).Infof("DELETE namespace: %s", ns.Name)
		return filter.CONTINUE
	}
	glog.Errorf("debug_DELETE: obj's type is unknown")
	return filter.IGNORE
}

func DEBUG_UPDATE(action string, ls *L.Listeners, objs ...interface{}) int {
	if len(objs) != 2 {
		glog.Errorf("debug_UPDTAE: objs's length is not 2")
		return filter.IGNORE
	}
	old := objs[0]
	new := objs[1]

	if oldpod, ok := old.(*v1.Pod); ok {
		if newpod, ok := new.(*v1.Pod); ok {
			glog.V(1).Infof("UPDATE pod:  %s -> %s in %s", oldpod.Name, newpod.Name, oldpod.Namespace)
			return filter.CONTINUE
		}
		glog.Errorf("new obj's type is not pod")
		return filter.IGNORE
	}

	if oldsvc, ok := old.(*v1.Service); ok {
		if newsvc, ok := new.(*v1.Service); ok {
			glog.V(1).Infof("UPDATE svc:  %s -> %s in %s", oldsvc.Name, newsvc.Name, oldsvc.Namespace)
			return filter.CONTINUE
		}
		glog.Errorf("new obj's type is not svc")
		return filter.IGNORE
	}

	if oldns, ok := old.(*v1.Namespace); ok {
		if newns, ok := new.(*v1.Namespace); ok {
			glog.V(1).Infof("UPDATE ns:  %s->%s ", oldns.Name, newns.Name)
			return filter.CONTINUE
		}
		glog.Errorf("new obj's type is not ns")
		return filter.IGNORE
	}

	glog.Errorf("debug_UPDTAE: obj's type is unknown")
	return filter.IGNORE
}
