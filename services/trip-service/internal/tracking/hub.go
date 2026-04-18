package tracking

import (
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type client struct {
	conn   *websocket.Conn
	tripID string
}

// Hub manages WebSocket connections per trip.
type Hub struct {
	mu      sync.RWMutex
	clients map[string]map[*client]struct{} // tripID → set of clients
}

func NewHub() *Hub {
	return &Hub{clients: make(map[string]map[*client]struct{})}
}

func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request, tripID string) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error().Err(err).Msg("ws upgrade failed")
		return
	}
	c := &client{conn: conn, tripID: tripID}
	h.register(c)
	defer h.unregister(c)

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}
		// Driver sends location; broadcast to all clients of that trip.
		h.broadcast(tripID, msg)
	}
}

func (h *Hub) broadcast(tripID string, msg []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients[tripID] {
		if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			c.conn.Close()
		}
	}
}

func (h *Hub) register(c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.clients[c.tripID] == nil {
		h.clients[c.tripID] = make(map[*client]struct{})
	}
	h.clients[c.tripID][c] = struct{}{}
}

func (h *Hub) unregister(c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients[c.tripID], c)
	c.conn.Close()
}
