package watchparty

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Hub struct {
	store    *Store
	logger   *slog.Logger
	upgrader websocket.Upgrader

	mu    sync.Mutex
	rooms map[string]*room
}

type ConnectionParams struct {
	Party       WatchParty
	Participant Participant
}

type ClientMessage struct {
	Type            string  `json:"type"`
	PositionSeconds float64 `json:"positionSeconds"`
	DurationSeconds float64 `json:"durationSeconds"`
	IsPlaying       bool    `json:"isPlaying"`
}

type ServerMessage struct {
	Type         string        `json:"type"`
	Party        *WatchParty   `json:"party,omitempty"`
	Participant  Participant   `json:"participant,omitempty"`
	Participants []Participant `json:"participants,omitempty"`
	Error        string        `json:"error,omitempty"`
}

type room struct {
	partyID string
	clients map[*client]struct{}
}

type client struct {
	hub         *Hub
	room        *room
	party       WatchParty
	participant Participant
	conn        *websocket.Conn
	send        chan ServerMessage
}

func NewHub(store *Store, logger *slog.Logger) *Hub {
	if logger == nil {
		logger = slog.Default()
	}
	return &Hub{
		store:  store,
		logger: logger,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
		rooms: make(map[string]*room),
	}
}

func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request, params ConnectionParams) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Warn("watch party websocket upgrade failed", slog.Any("error", err))
		return
	}

	rm := h.addClientRoom(params.Party.ID)
	cl := &client{
		hub:         h,
		room:        rm,
		party:       params.Party,
		participant: params.Participant,
		conn:        conn,
		send:        make(chan ServerMessage, 16),
	}

	h.register(cl)
	go cl.writeLoop()
	cl.readLoop(r.Context())
}

func (h *Hub) BroadcastEnded(party WatchParty) {
	h.broadcast(party.ID, ServerMessage{Type: "ended", Party: &party})
}

func (h *Hub) addClientRoom(partyID string) *room {
	h.mu.Lock()
	defer h.mu.Unlock()

	rm := h.rooms[partyID]
	if rm == nil {
		rm = &room{partyID: partyID, clients: make(map[*client]struct{})}
		h.rooms[partyID] = rm
	}
	return rm
}

func (h *Hub) register(cl *client) {
	h.mu.Lock()
	cl.room.clients[cl] = struct{}{}
	h.mu.Unlock()

	_ = h.store.SetParticipantConnected(context.Background(), cl.participant.ID, true)
	participants, _ := h.store.ListConnectedParticipants(context.Background(), cl.party.ID)
	cl.enqueue(ServerMessage{
		Type:         "snapshot",
		Party:        &cl.party,
		Participant:  cl.participant,
		Participants: participants,
	})
	h.broadcastExcept(cl.party.ID, cl, ServerMessage{
		Type:         "participant_joined",
		Participant:  cl.participant,
		Participants: participants,
	})
}

func (h *Hub) unregister(cl *client) {
	h.mu.Lock()
	delete(cl.room.clients, cl)
	empty := len(cl.room.clients) == 0
	if empty {
		delete(h.rooms, cl.room.partyID)
	}
	h.mu.Unlock()

	_ = h.store.SetParticipantConnected(context.Background(), cl.participant.ID, false)
	participants, _ := h.store.ListConnectedParticipants(context.Background(), cl.party.ID)
	h.broadcast(cl.party.ID, ServerMessage{
		Type:         "participant_left",
		Participant:  cl.participant,
		Participants: participants,
	})
}

func (h *Hub) broadcast(partyID string, message ServerMessage) {
	h.broadcastExcept(partyID, nil, message)
}

func (h *Hub) broadcastExcept(partyID string, excluded *client, message ServerMessage) {
	h.mu.Lock()
	rm := h.rooms[partyID]
	if rm == nil {
		h.mu.Unlock()
		return
	}
	clients := make([]*client, 0, len(rm.clients))
	for cl := range rm.clients {
		if cl != excluded {
			clients = append(clients, cl)
		}
	}
	h.mu.Unlock()

	for _, cl := range clients {
		cl.enqueue(message)
	}
}

func (cl *client) enqueue(message ServerMessage) {
	select {
	case cl.send <- message:
	default:
		cl.hub.logger.Warn("watch party websocket send queue full",
			slog.String("party_id", cl.party.ID),
			slog.String("participant_id", cl.participant.ID),
		)
	}
}

func (cl *client) readLoop(ctx context.Context) {
	defer func() {
		cl.hub.unregister(cl)
		_ = cl.conn.Close()
	}()

	cl.conn.SetReadLimit(4096)
	_ = cl.conn.SetReadDeadline(time.Now().Add(75 * time.Second))
	cl.conn.SetPongHandler(func(string) error {
		return cl.conn.SetReadDeadline(time.Now().Add(75 * time.Second))
	})

	for {
		var message ClientMessage
		if err := cl.conn.ReadJSON(&message); err != nil {
			return
		}
		cl.handleMessage(ctx, message)
	}
}

func (cl *client) handleMessage(ctx context.Context, message ClientMessage) {
	switch message.Type {
	case "play", "pause", "seek", "sync":
	default:
		cl.enqueue(ServerMessage{Type: "error", Error: "unsupported message type"})
		return
	}

	if cl.party.ControlMode == ControlModeHostOnly && cl.participant.Role != RoleHost {
		cl.enqueue(ServerMessage{Type: "error", Error: "only the host can control playback"})
		return
	}

	isPlaying := message.IsPlaying
	if message.Type == "play" {
		isPlaying = true
	}
	if message.Type == "pause" {
		isPlaying = false
	}

	party, err := cl.hub.store.UpdatePlayback(ctx, UpdatePlaybackParams{
		ID:              cl.party.ID,
		PositionSeconds: message.PositionSeconds,
		DurationSeconds: message.DurationSeconds,
		IsPlaying:       isPlaying,
	})
	if err != nil {
		cl.enqueue(ServerMessage{Type: "error", Error: "unable to update playback"})
		return
	}
	cl.party = party

	cl.hub.broadcast(cl.party.ID, ServerMessage{
		Type:        message.Type,
		Party:       &party,
		Participant: cl.participant,
	})
}

func (cl *client) writeLoop() {
	ticker := time.NewTicker(25 * time.Second)
	defer func() {
		ticker.Stop()
		_ = cl.conn.Close()
	}()

	for {
		select {
		case message, ok := <-cl.send:
			if !ok {
				_ = cl.conn.WriteMessage(websocket.CloseMessage, nil)
				return
			}
			body, err := json.Marshal(message)
			if err != nil {
				continue
			}
			if err := cl.conn.WriteMessage(websocket.TextMessage, body); err != nil {
				return
			}
		case <-ticker.C:
			if err := cl.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
