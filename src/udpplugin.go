package main

import (
	"encoding/binary"
	"errors"
	"log"
	"math"
	"net"
	"strconv"
	"time"

	"golang.org/x/text/encoding/unicode/utf32"
)

const (
	acspProtocolVersion     = 4
	acspNewSession          = 50
	acspNewConnection       = 51
	acspConnectionClosed    = 52
	acspCarUpdate           = 53
	acspCarInfo             = 54
	acspEndSession          = 55
	acspLapCompleted        = 73
	acspVersion             = 56
	acspChat                = 57
	acspClientLoaded        = 58
	acspSessionInfo         = 59
	acspError               = 60
	acspClientEvent         = 130
	acspCeCollisionWithCar  = 10
	acspCeCollisionWithEnv  = 11
	acspRealtimeposInterval = 200
	acspGetCarInfo          = 201
	acspSendChat            = 202
	acspBroadcastChat       = 203
	acspGetSessionInfo      = 204
	acspSetSessionInfo      = 205
	acspKickUser            = 206
	acspNextSession         = 207
	acspRestartSession      = 208
	acspAdminCommand        = 209
)

type SessionInfo struct {
	version             int
	sessionIndex        int
	currentSessionIndex int
	sessionCount        int
	serverName          string
	track               string
	trackConfig         string
	name                string
	typ                 int
	time                int
	laps                int
	waitTime            int
	ambientTemp         int
	roadTemp            int
	weatherGraphics     string
	elapsedMs           int32
}

func (s SessionInfo) sameAs(other SessionInfo) bool {
	return s.version == other.version &&
		s.sessionIndex == other.sessionIndex &&
		s.currentSessionIndex == other.currentSessionIndex &&
		s.sessionCount == other.sessionCount &&
		s.serverName == other.serverName &&
		s.track == other.track &&
		s.trackConfig == other.trackConfig &&
		s.name == other.name &&
		s.typ == other.typ &&
		s.time == other.time &&
		s.laps == other.laps &&
		s.waitTime == other.waitTime &&
		s.ambientTemp == other.ambientTemp &&
		s.roadTemp == other.roadTemp &&
		s.weatherGraphics == other.weatherGraphics &&
		s.elapsedMs == other.elapsedMs
}

type ClientEvent struct {
	carId       int
	otherCarId  int
	eventType   int
	impactSpeed float32
	worldPos    Vector
	relPos      Vector
}

type CarInfo struct {
	carId       int
	isConnected bool
	carModel    string
	carSkin     string
	driverName  string
	driverTeam  string
	driverGuid  string
}

type CarUpdate struct {
	carId               int
	position            Vector
	velocity            Vector
	gear                int
	engineRpm           int
	normalizedSplinePos float32
}

type NewConnection struct {
	driverName string
	driverGuid string
	carId      int
	carModel   string
	carSkin    string
}

type ConnectionClosed struct {
	driverName string
	driverGuid string
	carId      int
	carModel   string
	carSkin    string
}

type LapCompleted struct {
	carId   int
	laptime uint32
	cuts    int
}

type Vector struct {
	x float32
	y float32
	z float32
}

type UdpReader struct {
	data      []byte
	readIndex int64
}

func (r *UdpReader) New(p []byte) {
	r.data = p
}

func (r *UdpReader) ReadUint8() int {
	val := r.data[r.readIndex]
	r.readIndex = r.readIndex + 1
	return int(val)
}

func (r *UdpReader) ReadBytes(bytes int) []byte {
	val := r.data[r.readIndex : r.readIndex+int64(bytes)]
	r.readIndex = r.readIndex + int64(bytes)
	return val
}

func (r *UdpReader) ReadString() string {
	length := int(r.ReadUint8())
	bytes := r.ReadBytes(length)
	return string(bytes)
}

func (r *UdpReader) ReadUTF32String() string {
	length := int(r.ReadUint8() * 4)
	bytes := r.ReadBytes(length)

	val, err := utf32.UTF32(utf32.LittleEndian, utf32.IgnoreBOM).NewDecoder().Bytes(bytes)

	if err != nil {
		log.Print("Could not convert to UTF32: ", err)
	}

	return string(val)
}

func (r *UdpReader) ReadUint16() int {
	bytes := r.ReadBytes(2)
	data := binary.LittleEndian.Uint16(bytes)
	return int(data)
}

func (r *UdpReader) ReadInt32() int32 {
	bytes := r.ReadBytes(4)
	data := binary.LittleEndian.Uint32(bytes)
	return int32(data)
}

func (r *UdpReader) ReadUint32() uint32 {
	bytes := r.ReadBytes(4)
	data := binary.LittleEndian.Uint32(bytes)
	return data
}

func (r *UdpReader) ReadFloat() float32 {
	bytes := r.ReadBytes(4)
	a := binary.LittleEndian.Uint32(bytes)
	a2 := math.Float32frombits(a)
	return a2
}

type UdpWriter struct {
	data []byte
}

func (w *UdpWriter) WriteByte(d byte) error {
	w.data = append(w.data, d)
	return nil
}

func (w *UdpWriter) WriteUint16(d int) {
	var b [2]byte
	binary.LittleEndian.PutUint16(b[:], uint16(d))
	w.data = append(w.data, b[:]...)
}

func (w *UdpWriter) WriteUTF32String(str string) {
	val, err := utf32.UTF32(utf32.LittleEndian, utf32.IgnoreBOM).NewEncoder().Bytes([]byte(str))

	w.WriteByte(byte(len(str)))

	if err != nil {
		log.Print("Could not convert to UTF32: ", err)
	}
	w.data = append(w.data, val[:]...)
}

func readSessionInfo(r UdpReader) SessionInfo {
	var s SessionInfo
	s.version = r.ReadUint8()
	s.sessionIndex = r.ReadUint8()
	s.currentSessionIndex = r.ReadUint8()
	s.sessionCount = r.ReadUint8()
	s.serverName = r.ReadUTF32String()
	s.track = r.ReadString()
	s.trackConfig = r.ReadString()
	s.name = r.ReadString()
	s.typ = r.ReadUint8()
	s.time = r.ReadUint16()
	s.laps = r.ReadUint16()
	s.waitTime = r.ReadUint16()
	s.ambientTemp = r.ReadUint8()
	s.roadTemp = r.ReadUint8()
	s.weatherGraphics = r.ReadString()
	s.elapsedMs = r.ReadInt32()
	return s
}

type UdpPlugin struct {
	conn   *net.UDPConn
	online bool
}

func logSession(prefix string, sess SessionInfo) {
	log.Printf("%s index=%d/%d current=%d name=%q type=%d track=%q config=%q time=%d laps=%d openWait=%d elapsed_ms=%d",
		prefix,
		sess.sessionIndex,
		sess.sessionCount,
		sess.currentSessionIndex,
		sess.name,
		sess.typ,
		sess.track,
		sess.trackConfig,
		sess.time,
		sess.laps,
		sess.waitTime,
		sess.elapsedMs,
	)
}

// udpListen binds the plugin socket pair for one instance: we listen on
// listenPort (UDP_PLUGIN_ADDRESS) and send commands to acServer on serverPort
// (UDP_PLUGIN_LOCAL_PORT).
func udpListen(listenPort int, serverPort int) *UdpPlugin {
	udp := &UdpPlugin{}
	udpClient, err := net.ResolveUDPAddr("udp", ":"+strconv.Itoa(listenPort))
	if err != nil {
		log.Printf("Could not resolve UDP on %d: %v", listenPort, err)
	}

	udpServer, err := net.ResolveUDPAddr("udp", ":"+strconv.Itoa(serverPort))
	if err != nil {
		log.Printf("Could not resolve UDP on %d: %v", serverPort, err)
	}

	udp.conn, err = net.DialUDP("udp", udpClient, udpServer)
	if err != nil {
		log.Print("Could not establish udp connection: ", err)
	}

	return udp
}

func (udp *UdpPlugin) Close() {
	if udp.conn != nil {
		udp.conn.Close()
	}
}

func (udp UdpPlugin) write(data []byte) {
	if udp.conn == nil {
		return
	}
	udp.conn.Write(data)
}

// udpLoop pumps plugin packets until the connection is closed (instance
// removed or ports rebound).
func (inst *Instance) udpLoop() {
	for inst.udpReceive() {
	}
}

// udpReceive handles one packet; returns false when the loop should stop.
func (inst *Instance) udpReceive() bool {
	udp := inst.Udp
	if udp == nil || udp.conn == nil {
		return false
	}

	data := make([]byte, 1024)
	if _, err := udp.conn.Read(data); err != nil {
		if errors.Is(err, net.ErrClosed) {
			return false
		}
		// Transient errors (e.g. ICMP port unreachable while acServer is
		// down) just mean there is nothing to process right now.
		time.Sleep(100 * time.Millisecond)
		return true
	}

	// A packet arrived: the plugin stream is alive.
	inst.markPacket()

	r := UdpReader{}
	r.New(data)

	acsp := r.ReadUint8()

	switch acsp {
	case acspError:
		err := r.ReadUTF32String()
		log.Print("ACSP_ERROR: ", err)

	case acspChat:
		car := r.ReadUint8()
		msg := r.ReadUTF32String()
		log.Print("ACSP_CHAT: " + strconv.Itoa(car) + "; " + msg)
		// The drift HUD reports run scores over chat; pick them off and attach
		// to the sending driver so the broadcast can show them.
		if live, score, best, ok := parseDriftChat(msg); ok {
			inst.driverDrift(car, live, score, best)
		}

	case acspClientLoaded:
		car := r.ReadUint8()
		log.Print("ACSP_CLIENT_LOADED: ", car)

	case acspVersion:
		v := r.ReadUint8()
		log.Print("ACSP_VERSION: ", v)
		udp.online = true
		inst.markOnline(true)
		inst.publishTelemetry()
		udp.WriteRealtimePositionInterval(200)

	case acspNewSession:
		sess := readSessionInfo(r)
		inst.mu.Lock()
		changed := !sess.sameAs(inst.Status.Session)
		if changed {
			logSession("ACSP_NEW_SESSION", sess)
		}
		inst.Status.Session = sess
		inst.mu.Unlock()
		if changed {
			Events.Publish("session", inst.Id(), sessionEventPayload(sess))
			inst.resetDriverLaps()
		}

	case acspSessionInfo:
		sess := readSessionInfo(r)
		inst.mu.Lock()
		changed := !sess.sameAs(inst.Status.Session)
		if changed {
			logSession("ACSP_SESSION_INFO", sess)
		}
		inst.Status.Session = sess
		inst.mu.Unlock()
		if changed {
			Events.Publish("session", inst.Id(), sessionEventPayload(sess))
		}

	case acspEndSession:
		file := r.ReadUTF32String()
		log.Print("ACSP_END_SESSION: " + file)

		inst.mu.Lock()
		lastSession := inst.Status.Session.currentSessionIndex == inst.Status.Session.sessionCount-1
		inst.mu.Unlock()
		if lastSession {
			log.Print("UDP Plugin triggers Server Change Track")
			inst.serverChangeTrack()
		}

	case acspClientEvent:
		var ce ClientEvent
		ce.eventType = r.ReadUint8()
		ce.carId = r.ReadUint8()
		if ce.eventType == acspCeCollisionWithCar {
			ce.otherCarId = r.ReadUint8()
		}
		ce.impactSpeed = r.ReadFloat()
		ce.worldPos = Vector{r.ReadFloat(), r.ReadFloat(), r.ReadFloat()}
		ce.relPos = Vector{r.ReadFloat(), r.ReadFloat(), r.ReadFloat()}
		log.Print("ACSP_CLIENT_EVENT: ")
		PrintInterface(ce)

	case acspCarInfo:
		var ci CarInfo
		ci.carId = r.ReadUint8()
		ci.isConnected = r.ReadUint8() != 0
		ci.carModel = r.ReadUTF32String()
		ci.carSkin = r.ReadUTF32String()
		ci.driverName = r.ReadUTF32String()
		ci.driverTeam = r.ReadUTF32String()
		ci.driverGuid = r.ReadUTF32String()
		log.Print("ACSP_CAR_INFO: ")
		PrintInterface(ci)

	case acspCarUpdate:
		var cu CarUpdate
		cu.carId = r.ReadUint8()
		cu.position = Vector{r.ReadFloat(), r.ReadFloat(), r.ReadFloat()}
		cu.velocity = Vector{r.ReadFloat(), r.ReadFloat(), r.ReadFloat()}
		cu.gear = r.ReadUint8()
		cu.engineRpm = r.ReadUint16()
		cu.normalizedSplinePos = r.ReadFloat()
		inst.updateCarPosition(cu)

	case acspNewConnection:
		var nc NewConnection
		nc.driverName = r.ReadUTF32String()
		nc.driverGuid = r.ReadUTF32String()
		nc.carId = r.ReadUint8()
		nc.carModel = r.ReadString()
		nc.carSkin = r.ReadString()
		inst.mu.Lock()
		inst.Status.Players = inst.Status.Players + 1
		inst.mu.Unlock()
		inst.publishPlayers()
		inst.driverJoin(nc)
		log.Print("ACSP_NEW_CONNECTION: ")
		PrintInterface(nc)

	case acspConnectionClosed:
		var cc ConnectionClosed
		cc.driverName = r.ReadUTF32String()
		cc.driverGuid = r.ReadUTF32String()
		cc.carId = r.ReadUint8()
		cc.carModel = r.ReadString()
		cc.carSkin = r.ReadString()
		inst.mu.Lock()
		inst.Status.Players = inst.Status.Players - 1
		inst.mu.Unlock()
		inst.publishPlayers()
		inst.driverLeave(cc.carId)
		log.Print("ACSP_CONNECTION_CLOSED: ")
		PrintInterface(cc)

	case acspLapCompleted:
		var lc LapCompleted
		lc.carId = r.ReadUint8()
		lc.laptime = r.ReadUint32()
		lc.cuts = r.ReadUint8()
		inst.driverLap(lc)
		log.Print("ACSP_LAP_COMPLETED: ")
		PrintInterface(lc)

	default:
		log.Print("ACSP Unknown code: "+strconv.Itoa(acsp), data)
	}

	return true
}

func (udp UdpPlugin) WriteAdminCommand(command string) {
	var w UdpWriter
	w.WriteByte(acspAdminCommand)
	w.WriteUTF32String(command)
	udp.write(w.data)
}

func (udp UdpPlugin) WriteBroadcastChat(message string) {
	var w UdpWriter
	w.WriteByte(acspBroadcastChat)
	w.WriteUTF32String(message)
	udp.write(w.data)
}

func (udp UdpPlugin) WriteGetCarInfo(carid int) {
	var w UdpWriter
	w.WriteByte(acspGetCarInfo)
	w.WriteByte(byte(carid))
	udp.write(w.data)
}

func (udp UdpPlugin) WriteRealtimePositionInterval(intervalMs int) {
	var w UdpWriter
	w.WriteByte(acspRealtimeposInterval)
	w.WriteUint16(intervalMs)
	udp.write(w.data)
}

func (udp UdpPlugin) WriteKickUser(carid int) {
	var w UdpWriter
	w.WriteByte(acspKickUser)
	w.WriteByte(byte(carid))
	udp.write(w.data)
}

func (udp UdpPlugin) WriteNextSession() {
	var w UdpWriter
	w.WriteByte(acspNextSession)
	udp.write(w.data)
}

func (udp UdpPlugin) WriteRestartSession() {
	var w UdpWriter
	w.WriteByte(acspRestartSession)
	udp.write(w.data)
}

func (udp UdpPlugin) WriteSendChat(carid int, message string) {
	var w UdpWriter
	w.WriteByte(acspSendChat)
	w.WriteByte(byte(carid))
	w.WriteUTF32String(message)
	udp.write(w.data)
}
