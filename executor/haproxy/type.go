//create: 2018/01/04 16:52:24 Change: 2018/02/28 15:11:39 lijiaocn@foxmail.com
package haproxy

type HaproxyConfig struct {
	FrontendHTTP  []*FrontendHTTP
	FrontendHTTPS []*FrontendHTTPS
	FrontendTCP   []*FrontendTCP
	FrontendSSL   []*FrontendSSL
	Backend       []*Backend
}

type FrontendHTTP struct {
	Name     string
	BindIP   string
	BindPort string
	Backend  []*Backend
}

type FrontendHTTPS struct {
	Name      string
	BindIP    string
	BindPort  string
	CertFiles []string
	Backend   []*Backend
}

type FrontendTCP struct {
	Name     string
	BindIP   string
	BindPort string
	Backend  []*Backend
}

type FrontendSSL struct {
	Name      string
	BindIP    string
	BindPort  string
	CertFiles []string
	Backend   []*Backend
}

type Backend struct {
	Mode      string
	Name      string
	ACL       string
	Hosts     []string
	Algorithm string
	Servers   []*Server
}

type Server struct {
	Name   string
	Addr   string
	Port   string
	Option string
}
