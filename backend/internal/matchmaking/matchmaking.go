package matchmaking

import (
	"four-in-a-row/internal/bot"
	"four-in-a-row/internal/game"
	ws "four-in-a-row/internal/websocket"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	MatchTimeout = 10 * time.Second
)

type WaitingPlayer struct {
	Client    *ws.Client
	JoinedAt  time.Time
	Timer     *time.Timer
}

type MatchMaker struct {
	Hub           *ws.Hub
	WaitingQueue  []*WaitingPlayer
	mu            sync.Mutex
	OnGameStart   func(g *game.Game, p1Client, p2Client *ws.Client)
	OnBotMove     func(g *game.Game, botPlayer *bot.Bot)
}

func NewMatchMaker(hub *ws.Hub) *MatchMaker {
	return &MatchMaker{
		Hub:          hub,
		WaitingQueue: make([]*WaitingPlayer, 0),
	}
}

func (m *MatchMaker) AddPlayer(client *ws.Client) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, wp := range m.WaitingQueue {
		if wp.Client.ID != client.ID && wp.Client.Username != client.Username {
			wp.Timer.Stop()
			m.WaitingQueue = append(m.WaitingQueue[:i], m.WaitingQueue[i+1:]...)
			
			go m.startGame(wp.Client, client, false)
			return
		}
	}

	timer := time.AfterFunc(MatchTimeout, func() {
		m.handleTimeout(client)
	})

	m.WaitingQueue = append(m.WaitingQueue, &WaitingPlayer{
		Client:   client,
		JoinedAt: time.Now(),
		Timer:    timer,
	})

	m.Hub.SendToClient(client.ID, &ws.Message{
		Type:    "waiting",
		Message: "Looking for an opponent...",
	})

	log.Printf("Player %s added to queue, queue size: %d", client.Username, len(m.WaitingQueue))
}

func (m *MatchMaker) RemovePlayer(clientID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, wp := range m.WaitingQueue {
		if wp.Client.ID == clientID {
			wp.Timer.Stop()
			m.WaitingQueue = append(m.WaitingQueue[:i], m.WaitingQueue[i+1:]...)
			log.Printf("Player removed from queue, queue size: %d", len(m.WaitingQueue))
			return
		}
	}
}

func (m *MatchMaker) handleTimeout(client *ws.Client) {
	m.mu.Lock()
	
	found := false
	for i, wp := range m.WaitingQueue {
		if wp.Client.ID == client.ID {
			m.WaitingQueue = append(m.WaitingQueue[:i], m.WaitingQueue[i+1:]...)
			found = true
			break
		}
	}
	m.mu.Unlock()

	if found {
		log.Printf("No opponent found for %s, starting bot game", client.Username)
		m.startGame(client, nil, true)
	}
}

func (m *MatchMaker) startGame(player1 *ws.Client, player2 *ws.Client, isBot bool) {
	gameID := uuid.New().String()
	
	p2ID := ""
	p2Name := "Bot"
	if player2 != nil {
		p2ID = player2.ID
		p2Name = player2.Username
	} else {
		p2ID = "bot-" + gameID
	}

	newGame := game.NewGame(
		gameID,
		player1.ID,
		player1.Username,
		p2ID,
		p2Name,
		isBot,
	)
	newGame.StartTime = time.Now().Unix()

	m.Hub.SetGame(gameID, newGame)
	m.Hub.SetPlayerGame(player1.ID, gameID)
	player1.GameID = gameID

	if player2 != nil {
		m.Hub.SetPlayerGame(player2.ID, gameID)
		player2.GameID = gameID
	}

	m.Hub.SendToClient(player1.ID, &ws.Message{
		Type:     "game_start",
		GameID:   gameID,
		Opponent: p2Name,
		YourTurn: true,
		IsBot:    isBot,
		Player:   game.Player1,
	})

	if player2 != nil {
		m.Hub.SendToClient(player2.ID, &ws.Message{
			Type:     "game_start",
			GameID:   gameID,
			Opponent: player1.Username,
			YourTurn: false,
			IsBot:    false,
			Player:   game.Player2,
		})
	}

	log.Printf("Game started: %s vs %s (bot: %v)", player1.Username, p2Name, isBot)

	if m.OnGameStart != nil {
		m.OnGameStart(newGame, player1, player2)
	}
}

func (m *MatchMaker) GetWaitingCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.WaitingQueue)
}
