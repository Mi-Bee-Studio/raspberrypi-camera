package onvif

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
	"github.com/google/uuid"
	"github.com/Mi-Bee-Studio/raspberrypi-camera/internal/config"
)

// WS-Discovery constants.
const (
	DiscoveryAddr    = "239.255.255.250:3702"
	DiscoveryTTL     = 1
	ProbeAction      = "http://schemas.xmlsoap.org/ws/2004/09/discovery/Probe"
	ProbeMatchesAction = "http://schemas.xmlsoap.org/ws/2004/09/discovery/ProbeMatches"
	DiscoveryNS      = "http://schemas.xmlsoap.org/ws/2004/09/discovery"
)

// Discovery handles WS-Discovery Probe/ProbeMatches for ONVIF device discovery.
// Discovery handles WS-Discovery Probe/ProbeMatches for ONVIF device discovery.
type Discovery struct {
	uuid       string
	scopes     []string
	fallbackIP string // device's own IP, used only when client IP is unavailable
	port       int
	listener   *net.UDPConn
	mu         sync.Mutex
}

// NewDiscovery creates a new WS-Discovery responder.
//
// fallbackIP is the device's own IP address. It is used as the XAddr host in
// ProbeMatches only when no per-request client IP can be determined (rare —
// both UDP and HTTP probes carry the requester's source address).
func NewDiscovery(cfg *config.Config, fallbackIP string) *Discovery {
	port := cfg.ONVIF.Port
	if port == 0 {
		port = 8080
	}
	if fallbackIP == "" {
		fallbackIP = detectLocalIP()
	}

	name := cfg.Device.Name
	if name == "" {
		name = "Pi Camera V1"
	}
	hw := cfg.Device.HardwareID
	if hw == "" {
		hw = "OV5647"
	}

	return &Discovery{
		uuid:       "uuid:" + uuid.New().String(),
		fallbackIP: fallbackIP,
		port:       port,
		scopes: []string{
			"onvif://www.onvif.org/name/" + name,
			"onvif://www.onvif.org/hardware/" + hw,
		},
	}
}

// UUID returns the device UUID.
func (d *Discovery) UUID() string { return d.uuid }

// Scopes returns the ONVIF scope URIs.
func (d *Discovery) Scopes() []string { return d.scopes }

// XAddrs returns the XAddr endpoint URLs built with the given client IP
// (falling back to the device's own address when clientIP is empty).
func (d *Discovery) XAddrs(clientIP string) []string {
	ip := clientIP
	if ip == "" {
		ip = d.fallbackIP
	}
	return []string{
		fmt.Sprintf("http://%s:%d/onvif/device_service", ip, d.port),
	}
}

// StartUDP starts listening for UDP multicast probes on 239.255.255.250:3702.
func (d *Discovery) StartUDP(ctx context.Context) error {
	addr, err := net.ResolveUDPAddr("udp", DiscoveryAddr)
	if err != nil {
		return fmt.Errorf("resolve multicast address: %w", err)
	}

	conn, err := net.ListenMulticastUDP("udp", nil, addr)
	if err != nil {
		return fmt.Errorf("listen multicast: %w", err)
	}

	d.mu.Lock()
	d.listener = conn
	d.mu.Unlock()

	go d.readLoop(ctx)

	log.Printf("onvif: discovery UDP listener started on %s", DiscoveryAddr)
	return nil
}

// StopUDP stops the UDP listener.
func (d *Discovery) StopUDP() error {
	d.mu.Lock()
	conn := d.listener
	d.listener = nil
	d.mu.Unlock()

	if conn == nil {
		return nil
	}
	return conn.Close()
}

// readLoop reads UDP multicast messages and responds to Probe requests.
func (d *Discovery) readLoop(ctx context.Context) {
	buf := make([]byte, 4096)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		d.mu.Lock()
		conn := d.listener
		d.mu.Unlock()
		if conn == nil {
			return
		}

		conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		n, src, err := conn.ReadFromUDP(buf)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			continue
		}

		msg := buf[:n]
		clientIP := ""
		if src != nil {
			clientIP = src.IP.String()
		}
		resp := d.handleProbe(msg, clientIP)
		if resp == nil {
			continue
		}

		_, err = conn.WriteToUDP(resp, src)
		if err != nil {
			log.Printf("onvif: failed to send ProbeMatches to %s: %v", src, err)
		}
	}
}

// HandleHTTPProbe handles HTTP POST probe to /onvif/device_service.
// The client IP is taken from the request context (populated by Server.ServeHTTP
// from r.RemoteAddr), so the response uses whichever address the NVR reached.
func (d *Discovery) HandleHTTPProbe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	clientIP := ServerIPFromContext(r.Context(), "")
	resp := d.handleProbe(body, clientIP)
	if resp == nil {
		w.WriteHeader(http.StatusOK)
		return
	}

	w.Header().Set("Content-Type", "application/soap+xml; charset=utf-8")
	w.Write(resp)
}

// probeEnvelope is a minimal struct to extract MessageID from a Probe message.
type probeEnvelope struct {
	XMLName xml.Name `xml:"Envelope"`
	Header  struct {
		Action    string `xml:"Action"`
		MessageID string `xml:"MessageID"`
	} `xml:"Header"`
}

// handleProbe processes a Probe message and responds with ProbeMatches.
// clientIP is the source IP of the probe (UDP src or HTTP RemoteAddr host);
// when empty the device's fallback address is used.
func (d *Discovery) handleProbe(msg []byte, clientIP string) []byte {
	var env probeEnvelope
	if err := xml.Unmarshal(msg, &env); err != nil {
		return nil
	}

	// Check if this is a Probe action (match with or without namespace prefix)
	action := env.Header.Action
	if action == "" {
		return nil
	}
	if !isProbeAction(action) {
		return nil
	}

	messageID := env.Header.MessageID
	if messageID == "" {
		messageID = "uuid:unknown"
	}

	return d.buildProbeMatches(messageID, clientIP)
}

// isProbeAction checks if the SOAP action is a WS-Discovery Probe.
func isProbeAction(action string) bool {
	// Match both 2004/09 and 2005/04 namespace variants used by different ONVIF clients
	return strings.HasSuffix(action, "/discovery/Probe") ||
		action == ProbeAction
}

// buildProbeMatches creates the ProbeMatches XML response.
// clientIP overrides the XAddr host when non-empty; pass "" to use the
// device's own fallback address.
func (d *Discovery) buildProbeMatches(messageID, clientIP string) []byte {
	scopesStr := strings.Join(d.scopes, " ")
	xaddrsStr := strings.Join(d.XAddrs(clientIP), " ")

	// Build raw XML to maintain precise namespace control matching NVR expectations.
	// The NVR's probeMatchEnvelope uses local-name matching, so the XML element names
	// (ProbeMatch, Scopes, XAddrs, Types) must be exact regardless of namespace prefix.
	var buf bytes.Buffer
	buf.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	buf.WriteString(`<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope" xmlns:a="http://www.w3.org/2005/08/addressing">`)
	buf.WriteString(`<s:Header>`)
	buf.WriteString(fmt.Sprintf(`<a:Action s:mustUnderstand="1">%s</a:Action>`, ProbeMatchesAction))
	buf.WriteString(fmt.Sprintf(`<a:RelatesTo>%s</a:RelatesTo>`, messageID))
	buf.WriteString(`<a:To s:mustUnderstand="1">http://schemas.xmlsoap.org/ws/2004/08/addressing/role/anonymous</a:To>`)
	buf.WriteString(`</s:Header>`)
	buf.WriteString(`<s:Body>`)
	buf.WriteString(`<d:ProbeMatches xmlns:d="http://schemas.xmlsoap.org/ws/2004/09/discovery">`)
	buf.WriteString(`<d:ProbeMatch>`)
	buf.WriteString(`<a:EndpointReference xmlns:a="http://www.w3.org/2005/08/addressing"><a:Address>`)
	buf.WriteString(d.uuid)
	buf.WriteString(`</a:Address></a:EndpointReference>`)
	buf.WriteString(fmt.Sprintf(`<d:Scopes>%s</d:Scopes>`, scopesStr))
	buf.WriteString(fmt.Sprintf(`<d:XAddrs>%s</d:XAddrs>`, xaddrsStr))
	buf.WriteString(`<d:Types>tdn:NetworkVideoTransmitter tdn:Device</d:Types>`)
	buf.WriteString(`<d:MetadataVersion>1</d:MetadataVersion>`)
	buf.WriteString(`</d:ProbeMatch>`)
	buf.WriteString(`</d:ProbeMatches>`)
	buf.WriteString(`</s:Body>`)
	buf.WriteString(`</s:Envelope>`)

	return buf.Bytes()
}

// detectLocalIP finds a non-loopback IPv4 address for XAddr generation.
func detectLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
			return ipnet.IP.String()
		}
	}

	return "127.0.0.1"
}
