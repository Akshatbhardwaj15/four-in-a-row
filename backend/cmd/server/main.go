package main

import (
	"encoding/json"
	"fmt"
	"four-in-a-row/internal/bot"
	"four-in-a-row/internal/database"
	"four-in-a-row/internal/game"
	"four-in-a-row/internal/handlers"
	"four-in-a-row/internal/matchmaking"
	ws "four-in-a-row/internal/websocket"
	"four-in-a-row/pkg/kafka"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Server struct {
	Hub         *ws.Hub
	MatchMaker  *matchmaking.MatchMaker
	DB          *database.Database
	Kafka       *kafka.Producer
	BotPlayers  map[string]*bot.Bot
}

func main() {
	port := getEnv("PORT", "8080")
	dbConnStr := getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/connect_four?sslmode=disable")
	kafkaBroker := getEnv("KAFKA_BROKER", "")
	kafkaTopic := getEnv("KAFKA_TOPIC", "game-events")

	db, err := database.NewDatabase(dbConnStr)
	if err != nil {
		log.Printf("Warning: Database connection failed: %v (running without persistence)", err)
		db = nil
	}

	var kafkaProducer *kafka.Producer
	if kafkaBroker != "" {
		kafkaProducer = kafka.NewProducer([]string{kafkaBroker}, kafkaTopic)
	} else {
		kafkaProducer = kafka.NewProducer(nil, "")
	}

	hub := ws.NewHub()
	go hub.Run()

	server := &Server{
		Hub:        hub,
		DB:         db,
		Kafka:      kafkaProducer,
		BotPlayers: make(map[string]*bot.Bot),
	}

	server.MatchMaker = matchmaking.NewMatchMaker(hub)
	server.MatchMaker.OnGameStart = server.onGameStart

	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "healthy"})
	})

	if db != nil {
		h := handlers.NewHandlers(db)
		api := r.Group("/api")
		{
			api.GET("/leaderboard", h.GetLeaderboard)
			api.GET("/player/:username", h.GetPlayerStats)
			api.GET("/games", h.GetRecentGames)
		}
	} else {
		api := r.Group("/api")
		{
			api.GET("/leaderboard", func(c *gin.Context) {
				c.JSON(200, gin.H{"leaderboard": []interface{}{}})
			})
			api.GET("/player/:username", func(c *gin.Context) {
				c.JSON(200, gin.H{"username": c.Param("username"), "wins": 0, "losses": 0, "draws": 0, "games": 0})
			})
			api.GET("/games", func(c *gin.Context) {
				c.JSON(200, gin.H{"games": []interface{}{}})
			})
		}
	}

	r.GET("/ws", func(c *gin.Context) {
		server.handleWebSocket(c.Writer, c.Request)
	})

	log.Printf("Server starting on port %s", port)
	log.Fatal(r.Run(":" + port))
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	client := &ws.Client{
		ID:   uuid.New().String(),
		Conn: conn,
		Hub:  s.Hub,
		Send: make(chan []byte, 256),
	}

	s.Hub.Register <- client

	go client.WritePump()
	go client.ReadPump(s.handleMessage)
}

func (s *Server) handleMessage(client *ws.Client, data []byte) {
	var msg ws.Message
	if err := json.Unmarshal(data, &msg); err != nil {
		log.Printf("Failed to parse message: %v", err)
		return
	}

	switch msg.Type {
	case "join":
		s.handleJoin(client, msg)
	case "move":
		s.handleMove(client, msg)
	case "reconnect":
		s.handleReconnect(client, msg)
	}
}

func (s *Server) handleJoin(client *ws.Client, msg ws.Message) {
	if msg.Username == "" {
		s.Hub.SendToClient(client.ID, &ws.Message{
			Type:    "error",
			Message: "Username is required",
		})
		return
	}

	client.Username = msg.Username

	existingGameID := s.Hub.GetPlayerGame(msg.Username)
	if existingGameID != "" {
		existingGame := s.Hub.GetGame(existingGameID)
		if existingGame != nil && !existingGame.IsOver {
			client.GameID = existingGameID
			s.Hub.SetPlayerGame(client.ID, existingGameID)
			s.Hub.CancelDisconnectTimer(msg.Username)

			yourTurn := (existingGame.CurrentPlayer == game.Player1 && existingGame.Player1Name == msg.Username) ||
				(existingGame.CurrentPlayer == game.Player2 && existingGame.Player2Name == msg.Username)

			opponent := existingGame.Player2Name
			playerNum := game.Player1
			if existingGame.Player1Name != msg.Username {
				opponent = existingGame.Player1Name
				playerNum = game.Player2
			}

			s.Hub.SendToClient(client.ID, &ws.Message{
				Type:     "game_reconnected",
				GameID:   existingGameID,
				Board:    &existingGame.Board,
				Opponent: opponent,
				YourTurn: yourTurn,
				Player:   playerNum,
				IsBot:    existingGame.IsBot,
			})

			log.Printf("Player %s reconnected to game %s", msg.Username, existingGameID)
			return
		}
	}

	s.MatchMaker.AddPlayer(client)
}

func (s *Server) handleMove(client *ws.Client, msg ws.Message) {
	gameID := client.GameID
	if gameID == "" {
		s.Hub.SendToClient(client.ID, &ws.Message{
			Type:    "error",
			Message: "Not in a game",
		})
		return
	}

	g := s.Hub.GetGame(gameID)
	if g == nil || g.IsOver {
		s.Hub.SendToClient(client.ID, &ws.Message{
			Type:    "error",
			Message: "Game not found or already over",
		})
		return
	}

	isPlayer1 := g.Player1ID == client.ID || g.Player1Name == client.Username
	isPlayer2 := g.Player2ID == client.ID || g.Player2Name == client.Username

	if !isPlayer1 && !isPlayer2 {
		s.Hub.SendToClient(client.ID, &ws.Message{
			Type:    "error",
			Message: "You are not a player in this game",
		})
		return
	}

	expectedPlayer := game.Player1
	if isPlayer2 {
		expectedPlayer = game.Player2
	}

	if g.CurrentPlayer != expectedPlayer {
		s.Hub.SendToClient(client.ID, &ws.Message{
			Type:    "error",
			Message: "Not your turn",
		})
		return
	}

	row, valid := g.MakeMove(msg.Column)
	if !valid {
		s.Hub.SendToClient(client.ID, &ws.Message{
			Type:    "error",
			Message: "Invalid move",
		})
		return
	}

	if s.Kafka != nil {
		s.Kafka.SendMove(gameID, expectedPlayer, msg.Column, row)
	}

	s.Hub.Broadcast <- &ws.Message{
		Type:   "move",
		GameID: gameID,
		Column: msg.Column,
		Row:    row,
		Player: expectedPlayer,
		Board:  &g.Board,
	}

	if g.IsOver {
		s.endGame(g)
		return
	}

	if g.IsBot && g.CurrentPlayer == game.Player2 {
		go s.makeBotMove(g)
	}
}

func (s *Server) handleReconnect(client *ws.Client, msg ws.Message) {
	gameID := msg.GameID
	username := msg.Username

	if gameID == "" || username == "" {
		s.Hub.SendToClient(client.ID, &ws.Message{
			Type:    "error",
			Message: "Game ID and username are required for reconnection",
		})
		return
	}

	g := s.Hub.GetGame(gameID)
	if g == nil {
		s.Hub.SendToClient(client.ID, &ws.Message{
			Type:    "error",
			Message: "Game not found",
		})
		return
	}

	if g.IsOver {
		s.Hub.SendToClient(client.ID, &ws.Message{
			Type:    "error",
			Message: "Game is already over",
		})
		return
	}

	if g.Player1Name != username && g.Player2Name != username {
		s.Hub.SendToClient(client.ID, &ws.Message{
			Type:    "error",
			Message: "You are not a player in this game",
		})
		return
	}

	s.Hub.CancelDisconnectTimer(username)

	client.Username = username
	client.GameID = gameID
	s.Hub.SetPlayerGame(client.ID, gameID)
	s.Hub.SetPlayerGame(username, gameID)

	yourTurn := (g.CurrentPlayer == game.Player1 && g.Player1Name == username) ||
		(g.CurrentPlayer == game.Player2 && g.Player2Name == username)

	opponent := g.Player2Name
	playerNum := game.Player1
	if g.Player1Name != username {
		opponent = g.Player1Name
		playerNum = game.Player2
	}

	s.Hub.SendToClient(client.ID, &ws.Message{
		Type:     "game_reconnected",
		GameID:   gameID,
		Board:    &g.Board,
		Opponent: opponent,
		YourTurn: yourTurn,
		Player:   playerNum,
		IsBot:    g.IsBot,
	})

	log.Printf("Player %s reconnected to game %s", username, gameID)
}

func (s *Server) onGameStart(g *game.Game, p1Client, p2Client *ws.Client) {
	if s.Kafka != nil {
		s.Kafka.SendGameStart(g.ID, g.Player1Name, g.Player2Name, g.IsBot)
	}

	if g.IsBot {
		s.BotPlayers[g.ID] = bot.NewBot(game.Player2)
	}
}

func (s *Server) makeBotMove(g *game.Game) {
	time.Sleep(500 * time.Millisecond)

	botPlayer := s.BotPlayers[g.ID]
	if botPlayer == nil {
		botPlayer = bot.NewBot(game.Player2)
		s.BotPlayers[g.ID] = botPlayer
	}

	column := botPlayer.GetMove(g)
	row, valid := g.MakeMove(column)
	if !valid {
		log.Printf("Bot made invalid move: column %d", column)
		return
	}

	if s.Kafka != nil {
		s.Kafka.SendMove(g.ID, game.Player2, column, row)
	}

	s.Hub.Broadcast <- &ws.Message{
		Type:   "move",
		GameID: g.ID,
		Column: column,
		Row:    row,
		Player: game.Player2,
		Board:  &g.Board,
	}

	if g.IsOver {
		s.endGame(g)
	}
}

func (s *Server) endGame(g *game.Game) {
	g.EndTime = time.Now().Unix()

	winnerName := ""
	reason := "connect4"

	if g.IsDraw {
		reason = "draw"
	} else if g.Winner == game.Player1 {
		winnerName = g.Player1Name
	} else if g.Winner == game.Player2 {
		winnerName = g.Player2Name
	}

	s.Hub.Broadcast <- &ws.Message{
		Type:   "game_end",
		GameID: g.ID,
		Winner: winnerName,
		Reason: reason,
	}

	if s.Kafka != nil {
		duration := g.EndTime - g.StartTime
		s.Kafka.SendGameEnd(g.ID, winnerName, g.IsDraw, duration, len(g.Moves))
	}

	if s.DB != nil {
		if err := s.DB.SaveGame(g); err != nil {
			log.Printf("Failed to save game: %v", err)
		}
	}

	delete(s.BotPlayers, g.ID)
	s.Hub.RemovePlayerGame(g.Player1ID)
	if !g.IsBot {
		s.Hub.RemovePlayerGame(g.Player2ID)
	}

	log.Printf("Game %s ended. Winner: %s, Reason: %s", g.ID, winnerName, reason)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	fmt.Println("4 in a Row Game Server")
	fmt.Println("======================")
}
