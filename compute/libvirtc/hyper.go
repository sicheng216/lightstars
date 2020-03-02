package libvirtc

import (
	"github.com/danieldin95/lightstar/libstar"
	"github.com/libvirt/libvirt-go"
	"strings"
	"sync"
	"time"
)

type HyperListener struct {
	Opened func(Conn *libvirt.Connect) error
	Closed func(Conn *libvirt.Connect) error
}

type HyperVisor struct {
	Name     string
	Schema   string
	Address  string
	Path     string
	Conn     *libvirt.Connect
	Listener []HyperListener

	lock     sync.RWMutex
	ticker   *time.Ticker
	cpuSts   *libvirt.NodeCPUStats
	done     chan bool
	idleUtil uint64
	domsUtil map[string]uint64
}

func parseQemuTCP(name string) (address, path string) {
	if strings.Contains(name, "://") {
		addrs := strings.SplitN(name, "://", 2)[1]
		address = strings.SplitN(addrs, "/", 2)[0]
		if strings.Contains(addrs, "/") {
			path = strings.SplitN(addrs, "/", 2)[1]
		}
	}
	return address, path
}

func parseQemuSSH(name string) (address, path string) {
	if strings.Contains(name, "://") {
		addrs := strings.SplitN(name, "://", 2)[1]
		address = strings.SplitN(addrs, "/", 2)[0]
		if strings.Contains(addrs, "/") {
			path = strings.SplitN(addrs, "/", 2)[1]
		}
		if strings.Contains(address, "@") {
			address = strings.SplitN(address, "@", 2)[1]
		}
	}
	return address, path
}

func (h *HyperVisor) OpenNotSafe() error {
	if hyper.Conn != nil {
		if _, err := hyper.Conn.GetVersion(); err != nil {
			libstar.Error("HyperVisor.Open %s", err)
			hyper.Conn.Close()
			hyper.Conn = nil
		}
	}
	if hyper.Conn == nil {
		conn, err := libvirt.NewConnect(hyper.Name)
		if err != nil {
			return err
		}
		hyper.Conn = conn
		for _, listen := range h.Listener {
			if listen.Opened != nil {
				listen.Opened(h.Conn)
			}
		}
	}
	if hyper.Conn == nil {
		return libstar.NewErr("Not connect.")
	}
	return nil
}

func (h *HyperVisor) Open() error {
	h.lock.Lock()
	defer h.lock.Unlock()
	return h.OpenNotSafe()
}

func (h *HyperVisor) AddListener(listen HyperListener) {
	h.Listener = append(h.Listener, listen)
}

func (h *HyperVisor) SetName(name string) {
	hyper.Name = name

	h.Schema = strings.SplitN(h.Name, ":", 2)[0]
	switch h.Schema {
	case "qemu+ssh":
		h.Address, h.Path = parseQemuSSH(h.Name)
	case "qemu+tcp", "qemu+tls":
		h.Address, h.Path = parseQemuTCP(h.Name)
	default:
		h.Address = "localhost"
		h.Path = "system"
	}
	if strings.Contains(h.Address, ":") {
		h.Address = strings.SplitN(h.Address, ":", 2)[0]
	}
}

func (h *HyperVisor) FigureCPU() (err error) {
	h.lock.Lock()
	defer h.lock.Unlock()

	if err := h.OpenNotSafe(); err != nil {
		return err
	}
	if h.cpuSts == nil {
		h.cpuSts, err = h.Conn.GetCPUStats(-1, 0)
		if err != nil {
			libstar.Warn("HyperVisor.FigureCpu %s", err)
			return err
		}
	}
	newerSts, err := h.Conn.GetCPUStats(-1, 0)
	if err != nil {
		libstar.Warn("HyperVisor.FigureCpu %s", err)
		return err
	}
	older := h.cpuSts.User
	older += h.cpuSts.Idle
	older += h.cpuSts.Kernel
	older += h.cpuSts.Intr
	older += h.cpuSts.Iowait

	newer := newerSts.User
	newer += newerSts.Idle
	newer += newerSts.Kernel
	newer += newerSts.Intr
	newer += newerSts.Iowait

	h.idleUtil = 1000 * (newerSts.Idle - h.cpuSts.Idle) / (newer - older)
	// record last statics
	h.cpuSts = newerSts
	return nil
}

func (h *HyperVisor) LoopForever() {
	for {
		select {
		case <-h.done:
			return
		case <-h.ticker.C:
			h.FigureCPU()
		}
	}
}

func (h *HyperVisor) GetCPU() (uint, string, uint64) {
	if err := h.Open(); err != nil {
		return 0, "", 1000
	}

	h.lock.RLock()
	defer h.lock.RUnlock()
	if info, err := h.Conn.GetNodeInfo(); err == nil {
		return info.Cpus, info.Model, h.idleUtil
	}
	return 0, "", 1000
}

func (h *HyperVisor) GetMem() (t uint64, f uint64, c uint64) {
	if err := h.Open(); err != nil {
		return 0, 0, 0
	}
	if stats, err := h.Conn.GetMemoryStats(-1, 0); err == nil {
		if stats.TotalSet {
			t = stats.Total * 1024
		}
		if stats.FreeSet {
			f = stats.Free * 1024
		}
		if stats.CachedSet {
			c = stats.Cached * 1024
		}
	}
	return t, f, c
}

func (h *HyperVisor) GetRootfs() string {
	if err := h.Open(); err != nil {
		return ""
	}
	return ""
}

func (h *HyperVisor) ListAllDomains() ([]Domain, error) {
	if err := h.Open(); err != nil {
		return nil, err
	}

	domains, err := h.Conn.ListAllDomains(DOMAIN_ALL)
	if err != nil {
		return nil, err
	}
	newDomains := make([]Domain, 0, 32)
	for _, m := range domains {
		newDomains = append(newDomains, *NewDomainFromVir(&m))
	}
	return newDomains, nil
}

func (h *HyperVisor) LookupDomainByUUIDString(id string) (*Domain, error) {
	if err := h.Open(); err != nil {
		return nil, err
	}
	domain, err := hyper.Conn.LookupDomainByUUIDString(id)
	if err != nil {
		return nil, err
	}
	return NewDomainFromVir(domain), nil
}

func (h *HyperVisor) LookupDomainByUUIDName(id string) (*Domain, error) {
	if err := h.Open(); err != nil {
		return nil, err
	}
	domain, err := hyper.Conn.LookupDomainByUUIDString(id)
	if err != nil {
		domain, err := hyper.Conn.LookupDomainByName(id)
		if err != nil {
			return nil, err
		}
		return NewDomainFromVir(domain), nil
	}
	return NewDomainFromVir(domain), nil
}

func (h *HyperVisor) LookupDomainByName(id string) (*Domain, error) {
	if err := h.Open(); err != nil {
		return nil, err
	}
	domain, err := hyper.Conn.LookupDomainByName(id)
	if err != nil {
		return nil, err
	}
	return &Domain{*domain}, nil
}

func (h *HyperVisor) DomainDefineXML(xmlConfig string) (*Domain, error) {
	if err := h.Open(); err != nil {
		return nil, err
	}
	domain, err := h.Conn.DomainDefineXML(xmlConfig)
	if err != nil {
		return nil, err
	}
	return &Domain{*domain}, nil
}

func (h *HyperVisor) Close() {
	if h.Conn == nil {
		return
	}
	for _, listen := range h.Listener {
		if listen.Closed != nil {
			listen.Closed(h.Conn)
		}
	}
	h.Conn = nil
}

var hyper = HyperVisor{
	Listener: make([]HyperListener, 0, 32),
	ticker:   time.NewTicker(2 * time.Second),
	done:     make(chan bool),
	idleUtil: 1000,
	domsUtil: make(map[string]uint64, 32),
}

func GetHyper() (*HyperVisor, error) {
	return &hyper, hyper.Open()
}

func SetHyper(name string) (*HyperVisor, error) {
	if name == hyper.Name {
		return &hyper, nil
	}
	hyper.Close()
	hyper.SetName(name)
	return &hyper, hyper.Open()
}

func LookupDomainByUUIDString(uuid string) (*Domain, error) {
	hyper, err := GetHyper()
	if err != nil {
		return nil, err
	}
	dom, err := hyper.LookupDomainByUUIDString(uuid)
	if err != nil {
		return nil, err
	}
	return dom, nil
}

func LookupDomainByUUIDName(uuid string) (*Domain, error) {
	hyper, err := GetHyper()
	if err != nil {
		return nil, err
	}
	dom, err := hyper.LookupDomainByUUIDName(uuid)
	if err != nil {
		return nil, err
	}
	return dom, nil
}

func AddHyperListener(listen HyperListener) {
	hyper.AddListener(listen)
}

func init() {
	hyper.SetName("qemu:///system")
	go hyper.LoopForever()
}
