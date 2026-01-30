package websocket

import (
	"encoding/json"
	"four-in-a-row/internal/game"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Client struct {
	ID       string
	Username string
	Conn     *websocket.Conn
	Hub      *Hub
	GameID   string
	Send     chan []byte
	mu       sync.Mutex
}

type Hub struct {
	Clients          map[string]*Client
	Games            map[string]*game.Game
	PlayerToGame     map[string]string
	DisconnectTimers map[string]*time.Timer
	Register         chan *Client
	Unregister       chan *Client
	Broadcast        chan *Message
	mu               sync.RWMutex
}

type Message struct {
	Type     string          `json:"type"`
	GameID   string          `json:"game_id,omitempty"`
	Username string          `json:"username,omitempty"`
	Column   int             `json:"column,omitempty"`
	Row      int             `json:"row,omitempty"`
	Player   int             `json:"player,omitempty"`
	Board    *game.Board     `json:"board,omitempty"`
	Winner   string          `json:"winner,omitempty"`
	Reason   string          `json:"reason,omitempty"`
	Opponent string          `json:"opponent,omitempty"`
	YourTurn bool            `json:"your_turn,omitempty"`
	Message  string          `json:"message,omitempty"`
	IsBot    bool            `json:"is_bot,omitempty"`
	Data     json.RawMessage `json:"data,omitempty"`
}

func NewHub() *Hub {
	return &Hub{
		Clients:          make(map[string]*Client),
		Games:            make(map[string]*game.Game),
		PlayerToGame:     make(map[string]string),
		DisconnectTimers: make(map[string]*time.Timer),
		Register:         make(chan *Client),
		Unregister:       make(chan *Client),
		Broadcast:        make(chan *Message, 256),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.Register:
			h.mu.Lock()
			h.Clients[client.ID] = client
			h.mu.Unlock()
			log.Printf("Client registered: %s (%s)", client.Username, client.ID)

		case client := <-h.Unregister:
			h.mu.Lock()
			if _, ok := h.Clients[client.ID]; ok {
				delete(h.Clients, client.ID)
				close(client.Send)
			}
			h.mu.Unlock()
			log.Printf("Client unregistered: %s (%s)", client.Username, client.ID)

		case message := <-h.Broadcast:
			h.broadcastToGame(message)
		}
	}
}

func (h *Hub) broadcastToGame(msg *Message) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	gameObj, exists := h.Games[msg.GameID]
	if !exists {
		return
	}

	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Error marshaling message: %v", err)
		return
	}

	for _, client := range h.Clients {
		if client.GameID == msg.GameID {
			select {
			case client.Send <- data:
			default:
				close(client.Send)
				delete(h.Clients, client.ID)
			}
		}
	}
	_ = gameObj
}

func (h *Hub) SendToClient(clientID string, msg *Message) {
	h.mu.RLock()
	client, exists := h.Clients[clientID]
	h.mu.RUnlock()

	if !exists {
		return
	}

	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Error marshaling message: %v", err)
		return
	}

	select {
	case client.Send <- data:
	default:
		log.Printf("Failed to send message to client %s", clientID)
	}
}

func (h *Hub) GetGame(gameID string) *game.Game {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.Games[gameID]
}

func (h *Hub) SetGame(gameID string, g *game.Game) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.Games[gameID] = g
}

func (h *Hub) GetPlayerGame(playerID string) string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.PlayerToGame[playerID]
}

func (h *Hub) SetPlayerGame(playerID, gameID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.PlayerToGame[playerID] = gameID
}

func (h *Hub) RemovePlayerGame(playerID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.PlayerToGame, playerID)
}

func (h *Hub) GetClient(clientID string) *Client {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.Clients[clientID]
}

func (h *Hub) GetClientByUsername(username string) *Client {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, client := range h.Clients {
		if client.Username == username {
			return client
		}
	}
	return nil
}

func (h *Hub) StartDisconnectTimer(clientID string, duration time.Duration, onTimeout func()) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if timer, exists := h.DisconnectTimers[clientID]; exists {
		timer.Stop()
	}

	h.DisconnectTimers[clientID] = time.AfterFunc(duration, onTimeout)
}

func (h *Hub) CancelDisconnectTimer(clientID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if timer, exists := h.DisconnectTimers[clientID]; exists {
		timer.Stop()
		delete(h.DisconnectTimers, clientID)
	}
}

func (c *Client) ReadPump(handleMessage func(*Client, []byte)) {
	defer func() {
		c.Hub.Unregister <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(512 * 1024)
	c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}
		handleMessage(c, message)
	}
}

func (c *Client) WritePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			c.mu.Lock()
			err := c.Conn.WriteMessage(websocket.TextMessage, message)
			c.mu.Unlock()
			if err != nil {
				return
			}

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
