package web

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/Mi-Bee-Studio/mibee-eye-raspi/internal/camera"
	"github.com/gorilla/websocket"
)

const (
	wsWriteWait  = 10 * time.Second
	wsPongWait   = 60 * time.Second
	wsPingPeriod = 30 * time.Second
	wsMaxMsgSize = 512
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// wsHub maintains a set of active WebSocket connections and broadcasts events.
type wsHub struct {
	logger  *log.Logger
	mu      sync.RWMutex
	clients map[*websocket.Conn]struct{}
	events  chan wsEvent
	done    chan struct{}
}

func newWSHub(logger *log.Logger) *wsHub {
	return &wsHub{
		logger:  logger,
		clients: make(map[*websocket.Conn]struct{}),
		events:  make(chan wsEvent, 64),
		done:    make(chan struct{}),
	}
}

// sendEvent queues an event for broadcast. Non-blocking — drops if full.
func (h *wsHub) sendEvent(e wsEvent) {
	select {
	case h.events <- e:
	default:
		// Drop event if channel is full — non-blocking for hook callbacks.
	}
}

// run consumes events from the channel and broadcasts to all clients.
func (h *wsHub) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-h.done:
			return
		case e := <-h.events:
			data, err := json.Marshal(e)
			if err != nil {
				continue
			}

			h.mu.RLock()
			for conn := range h.clients {
				if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
					conn.Close()
					delete(h.clients, conn)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// close shuts down the hub and closes all connections.
func (h *wsHub) close() {
	close(h.done)
	h.mu.Lock()
	for conn := range h.clients {
		conn.Close()
	}
	h.clients = make(map[*websocket.Conn]struct{})
	h.mu.Unlock()
}

// addClient registers a new WebSocket connection.
func (h *wsHub) addClient(conn *websocket.Conn) {
	h.mu.Lock()
	h.clients[conn] = struct{}{}
	h.mu.Unlock()
}

// removeClient unregisters a WebSocket connection.
func (h *wsHub) removeClient(conn *websocket.Conn) {
	h.mu.Lock()
	delete(h.clients, conn)
	h.mu.Unlock()
	conn.Close()
}

// handleWS handles WebSocket upgrade requests.
func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Printf("web: WebSocket upgrade failed: %v", err)
		return
	}

	s.hub.addClient(conn)
	go s.wsWritePump(conn)
	go s.wsReadPump(conn)
}

// wsReadPump pumps messages from the WebSocket connection.
// Client messages are discarded except for pong handling.
func (s *Server) wsReadPump(conn *websocket.Conn) {
	defer s.hub.removeClient(conn)

	conn.SetReadLimit(wsMaxMsgSize)
	conn.SetReadDeadline(time.Now().Add(wsPongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(wsPongWait))
		return nil
	})

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
		// Discard all client messages. Pong handling is done by SetPongHandler.
	}
}

// wsWritePump sends server pings to keep the connection alive.
// Actual state-change events are broadcast by the hub's run loop.
func (s *Server) wsWritePump(conn *websocket.Conn) {
	ticker := time.NewTicker(wsPingPeriod)
	defer func() {
		ticker.Stop()
		conn.Close()
	}()

	// Send initial state snapshot on connect.
	s.sendInitialState(conn)

	for {
		select {
		case <-ticker.C:
			conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
			if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"ping"}`)); err != nil {
				return
			}
		}
	}
}

// sendInitialState sends the current parameter values, PTZ position, and presets
// to a newly connected WebSocket client.
func (s *Server) sendInitialState(conn *websocket.Conn) {
	conn.SetWriteDeadline(time.Now().Add(wsWriteWait))

	// Send current params.
	if s.cfg.Params != nil {
		for name := range camera.ParamRanges {
			if val, err := s.cfg.Params.Get(name); err == nil {
				msg, _ := json.Marshal(wsEvent{
					Type:  "param-changed",
					Name:  name,
					Value: val,
				})
				conn.WriteMessage(websocket.TextMessage, msg)
			}
		}
		for name := range camera.ParamEnums {
			if val, err := s.cfg.Params.Get(name); err == nil {
				msg, _ := json.Marshal(wsEvent{
					Type:  "param-changed",
					Name:  name,
					Value: val,
				})
				conn.WriteMessage(websocket.TextMessage, msg)
			}
		}
	}

	// Send PTZ position.
	if s.cfg.PTZ != nil {
		pos := s.cfg.PTZ.GetPosition()
		msg, _ := json.Marshal(map[string]interface{}{
			"type":     "ptz-position",
			"position": pos,
		})
		conn.WriteMessage(websocket.TextMessage, msg)
	}

	// Send preset list.
	if s.cfg.PTZ != nil {
		tokens := s.cfg.PTZ.GetPresets()
		for _, token := range tokens {
			pos, err := s.cfg.PTZ.GetPresetPosition(token)
			if err != nil {
				continue
			}
			msg, _ := json.Marshal(map[string]interface{}{
				"type":     "ptz-preset-added",
				"token":    token,
				"name":     token,
				"position": pos,
			})
			conn.WriteMessage(websocket.TextMessage, msg)
		}
	}
}
