package main

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"
)

// Instance bundles everything one acServer process needs: its DB-backed
// config (ports, name), the OS process handle, the UDP plugin connection
// and the rendered server configuration.
type Instance struct {
	Conf ServerInstance

	// mu guards cmd, lines, Status, drivers, positions, driftScorers and tel.
	mu                  sync.Mutex
	cmd                 *exec.Cmd
	lines               string
	drivers             map[int]*DriverState
	positions           map[int]*CarPositionState
	lastPositionPublish time.Time
	// driftScorers holds per-car server-side drift scoring state, fed by the
	// telemetry ingest WebSocket. lastDriftPublish throttles the high-rate live
	// drift updates pushed to SSE subscribers. See drivers.go / telemetryingest.go.
	driftScorers     map[int]*driftScorer
	driftMode        DriftScoringMode
	lastDriftPublish time.Time
	tel              telemetryHealth

	Udp    *UdpPlugin
	Status ServerStatus
	Cr     ConfigRenderer
}

// telemetryHealth tracks the liveness of the AC UDP plugin stream for one
// instance. Guarded by Instance.mu. It lets the UI tell "server running but no
// telemetry" (plugin misconfigured / crashed) apart from "running, no cars yet".
type telemetryHealth struct {
	udpOnline      bool
	lastPacketAt   time.Time
	lastDriverAt   time.Time
	lastPositionAt time.Time
}

// TelemetrySnapshot is the JSON-facing view of telemetryHealth plus the
// configured plugin ports. Times are unix-millis, 0 meaning "never".
type TelemetrySnapshot struct {
	UdpOnline        bool  `json:"udp_online"`
	LastPacketMs     int64 `json:"last_packet_ms"`
	LastDriverMs     int64 `json:"last_driver_ms"`
	LastPositionMs   int64 `json:"last_position_ms"`
	PluginListenPort int   `json:"plugin_listen_port"`
	PluginSendPort   int   `json:"plugin_send_port"`
}

func (inst *Instance) markPacket() {
	inst.mu.Lock()
	inst.tel.lastPacketAt = time.Now()
	inst.mu.Unlock()
}

func (inst *Instance) markOnline(online bool) {
	inst.mu.Lock()
	inst.tel.udpOnline = online
	inst.mu.Unlock()
}

func (inst *Instance) telemetrySnapshot() TelemetrySnapshot {
	listen, server := inst.pluginPorts()
	inst.mu.Lock()
	defer inst.mu.Unlock()
	ms := func(t time.Time) int64 {
		if t.IsZero() {
			return 0
		}
		return t.UnixMilli()
	}
	return TelemetrySnapshot{
		UdpOnline:        inst.tel.udpOnline,
		LastPacketMs:     ms(inst.tel.lastPacketAt),
		LastDriverMs:     ms(inst.tel.lastDriverAt),
		LastPositionMs:   ms(inst.tel.lastPositionAt),
		PluginListenPort: listen,
		PluginSendPort:   server,
	}
}

func (inst *Instance) Id() int {
	if inst.Conf.Id != nil {
		return *inst.Conf.Id
	}
	return 0
}

func (inst *Instance) Name() string {
	if inst.Conf.Name != nil {
		return *inst.Conf.Name
	}
	return "Instance " + strconv.Itoa(inst.Id())
}

// Dir is the working directory the acServer process runs from. The default
// instance keeps the historical TempFolder location so existing installs
// keep working; additional instances get their own subfolder.
func (inst *Instance) Dir() string {
	if inst.Id() <= 1 {
		return TempFolder
	}
	return filepath.Join(TempFolder, "instance_"+strconv.Itoa(inst.Id()))
}

// repeatEventId returns the event id this instance auto-repeats, and whether
// repeat mode is active. In repeat mode the queue is left untouched.
func (inst *Instance) repeatEventId() (int, bool) {
	if inst.Conf.RunMode != nil && *inst.Conf.RunMode == runModeRepeatEvent &&
		inst.Conf.RepeatEventId != nil && *inst.Conf.RepeatEventId > 0 {
		return *inst.Conf.RepeatEventId, true
	}
	return 0, false
}

func (inst *Instance) pluginPorts() (int, int) {
	listen := 5001
	server := 5000
	if inst.Conf.PluginListenPort != nil {
		listen = *inst.Conf.PluginListenPort
	}
	if inst.Conf.PluginPort != nil {
		server = *inst.Conf.PluginPort
	}
	return listen, server
}

type InstanceManager struct {
	mu        sync.RWMutex
	instances map[int]*Instance
}

func NewInstanceManager() *InstanceManager {
	return &InstanceManager{instances: make(map[int]*Instance)}
}

// LoadFromDb syncs the runtime instance set with the server_instance table:
// new rows get a runtime instance with a UDP listener, removed rows are
// stopped and discarded, existing rows get their config refreshed.
func (m *InstanceManager) LoadFromDb() error {
	confs, err := Dba.selectServerInstances()
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	seen := make(map[int]bool, len(confs))
	for _, conf := range confs {
		if conf.Id == nil {
			continue
		}
		id := *conf.Id
		seen[id] = true

		if inst, ok := m.instances[id]; ok {
			oldListen, oldServer := inst.pluginPorts()
			inst.Conf = conf
			newListen, newServer := inst.pluginPorts()
			if (oldListen != newListen || oldServer != newServer) && !inst.isRunning() {
				inst.rebindUdp()
			}
			continue
		}

		inst := &Instance{Conf: conf}
		listen, server := inst.pluginPorts()
		inst.Udp = udpListen(listen, server)
		go inst.udpLoop()
		m.instances[id] = inst
	}

	for id, inst := range m.instances {
		if !seen[id] {
			inst.stop()
			if inst.Udp != nil {
				inst.Udp.Close()
			}
			delete(m.instances, id)
		}
	}

	return nil
}

func (inst *Instance) rebindUdp() {
	if inst.Udp != nil {
		inst.Udp.Close()
	}
	listen, server := inst.pluginPorts()
	inst.Udp = udpListen(listen, server)
	go inst.udpLoop()
}

func (m *InstanceManager) Get(id int) *Instance {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.instances[id]
}

// Default returns the instance with the lowest id (the pre-multi-server
// behavior for endpoints that do not specify an instance).
func (m *InstanceManager) Default() *Instance {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var def *Instance
	for _, inst := range m.instances {
		if def == nil || inst.Id() < def.Id() {
			def = inst
		}
	}
	return def
}

func (m *InstanceManager) All() []*Instance {
	m.mu.RLock()
	defer m.mu.RUnlock()

	list := make([]*Instance, 0, len(m.instances))
	for _, inst := range m.instances {
		list = append(list, inst)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Id() < list[j].Id() })
	return list
}

// validatePorts rejects a config whose ports collide with another instance.
func (m *InstanceManager) validatePorts(si ServerInstance) error {
	ports := func(c ServerInstance) []int {
		vals := []int{}
		for _, p := range []*int{c.UdpPort, c.TcpPort, c.HttpPort, c.PluginPort, c.PluginListenPort} {
			if p != nil {
				vals = append(vals, *p)
			}
		}
		return vals
	}

	own := ports(si)
	ownSet := make(map[int]bool)
	for _, p := range own {
		// UDP and TCP game port may legitimately share a number (different protocols)
		ownSet[p] = true
	}

	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, other := range m.instances {
		if si.Id != nil && other.Id() == *si.Id {
			continue
		}
		for _, p := range ports(other.Conf) {
			if ownSet[p] {
				return fmt.Errorf("port %d is already used by instance %q", p, other.Name())
			}
		}
	}
	return nil
}

var errInstanceNotFound = errors.New("instance not found")

// instanceById resolves an instance or returns the default when id <= 0.
func instanceById(id int) (*Instance, error) {
	if id > 0 {
		inst := Instances.Get(id)
		if inst == nil {
			return nil, errInstanceNotFound
		}
		return inst, nil
	}
	inst := Instances.Default()
	if inst == nil {
		return nil, errInstanceNotFound
	}
	return inst, nil
}

func (inst *Instance) statusSnapshot() ServerStatus {
	inst.mu.Lock()
	defer inst.mu.Unlock()
	return inst.Status
}

// defaultStatus feeds the legacy HTML templates that expect a single
// server status (the default instance).
func defaultStatus() ServerStatus {
	inst := Instances.Default()
	if inst == nil {
		return ServerStatus{PublicIp: publicIp}
	}
	inst.refresh()
	return inst.statusSnapshot()
}
