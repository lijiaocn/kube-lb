#To record log need reconfig: /etc/rsyslog.conf
#   $ModLoad  imudp
#   $UDPServerRun  514
#   local2.*    /var/log/haproxy.log
# systemctl restart rsyslog

global
	log 127.0.0.1 local2 info
	chroot /var/lib/haproxy
	stats socket /run/haproxy/admin.sock mode 660 level admin
	stats timeout 30s
	user haproxy
	group haproxy
	daemon
	tune.ssl.default-dh-param 2048

defaults
	mode                    http
	log                     global
	option                  dontlognull
	option http-server-close
	option                  redispatch
	retries                 3
	timeout check           60s
	timeout client          900s
	timeout client-fin      3s
	timeout connect         15s
	timeout http-keep-alive 900s
	timeout http-request    60s
	timeout queue           300s
	timeout server          900s
	timeout server-fin      3s
	timeout tarpit          900s
	timeout tunnel          24h
	maxconn                 50000

listen stats
	bind *:8889
	mode http
	stats uri /tenx-stats
	stats realm Haproxy\ Statistics
	stats auth tenxcloud:haproxy-agent

{{with .FrontendHTTP}}{{range .}}frontend {{.Name}}
	mode http
	bind {{.BindIP}}:{{.BindPort}}
	option forwardfor except 127.0.0.0/8
	errorfile 503 /etc/haproxy/errors/503.http{{range .Backend}}
	acl {{.Name}} {{.ACL}}
	use_backend {{.Name}} if {{.Name}}{{end}}
{{end}}{{end}}

{{with .FrontendHTTPS}}{{range .}}frontend {{.Name}}
	mode http
	bind {{.BindIP}}:{{.BindPort}} ssl crt /etc/sslkeys/default.pem {{range .CertFiles}} crt /etc/sslkeys/certs/{{.}} {{end}}
	option forwardfor except 127.0.0.0/8
	errorfile 503 /etc/haproxy/errors/503.http{{range .Backend}}
	acl {{.Name}} {{.ACL}}
	use_backend {{.Name}} if {{.Name}} { ssl_fc_sni {{range .Hosts}}{{.}} {{end}} }{{end}}
{{end}}{{end}}

{{with .FrontendTCP}}{{range .}}frontend {{.Name}}
	mode tcp
	bind {{.BindIP}}:{{.BindPort}}{{range .Backend}}
	use_backend {{.Name}}{{end}}
{{end}}{{end}}

{{with .FrontendSSL}}{{range .}}frontend {{.Name}}
	mode tcp
	bind {{.BindIP}}:{{.BindPort}} ssl crt {{range .CertFiles}}/etc/sslkeys/certs/{{.}} {{end}}{{range .Backend}}
	use_backend {{.Name}}{{end}}
{{end}}{{end}}

{{with .Backend}}{{range .}}backend {{.Name}}
	mode {{ .Mode }}
	balance {{.Algorithm}}{{range .Servers}}
	server {{.Name}} {{.Addr}}:{{.Port}} {{.Option}}{{end}}
{{end}}{{end}}
