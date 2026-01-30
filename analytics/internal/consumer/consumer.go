package consumer

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"
)

type Consumer struct {
	reader  *kafka.Reader
	db      *sql.DB
	metrics *Metrics
	mu      sync.Mutex
}

type Metrics struct {
	TotalGames      int64
	TotalMoves      int64
	TotalDuration   int64
	WinnerCounts    map[string]int
	GamesPerHour    map[string]int
	BotGames        int64
	PlayerGames     int64
}

type GameEvent struct {
	Type      string          `json:"type"`
	GameID    string          `json:"game_id"`
	Timestamp int64           `json:"timestamp"`
	Data      json.RawMessage `json:"data"`
}

type GameStartData struct {
	Player1 string `json:"player1"`
	Player2 string `json:"player2"`
	IsBot   bool   `json:"is_bot"`
}

type MoveData struct {
	Player int `json:"player"`
	Column int `json:"column"`
	Row    int `json:"row"`
}

type GameEndData struct {
	Winner   string `json:"winner"`
	IsDraw   bool   `json:"is_draw"`
	Duration int64  `json:"duration"`
	Moves    int    `json:"moves"`
}

func NewConsumer(brokers []string, topic, groupID string, db *sql.DB) *Consumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        brokers,
		Topic:          topic,
		GroupID:        groupID,
		MinBytes:       1,
		MaxBytes:       10e6,
		CommitInterval: time.Second,
		StartOffset:    kafka.LastOffset,
	})

	return &Consumer{
		reader: reader,
		db:     db,
		metrics: &Metrics{
			WinnerCounts: make(map[string]int),
			GamesPerHour: make(map[string]int),
		},
	}
}

func (c *Consumer) Start(ctx context.Context) {
	log.Println("Kafka consumer started, waiting for events...")

	for {
		select {
		case <-ctx.Done():
			log.Println("Consumer shutting down...")
			return
		default:
			msg, err := c.reader.ReadMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				log.Printf("Error reading message: %v", err)
				continue
			}

			c.processMessage(msg.Value)
		}
	}
}

func (c *Consumer) processMessage(data []byte) {
	var event GameEvent
	if err := json.Unmarshal(data, &event); err != nil {
		log.Printf("Failed to unmarshal event: %v", err)
		return
	}

	switch event.Type {
	case "game_start":
		c.handleGameStart(event)
	case "move":
		c.handleMove(event)
	case "game_end":
		c.handleGameEnd(event)
	default:
		log.Printf("Unknown event type: %s", event.Type)
	}
}

func (c *Consumer) handleGameStart(event GameEvent) {
	var data GameStartData
	if err := json.Unmarshal(event.Data, &data); err != nil {
		log.Printf("Failed to unmarshal game_start data: %v", err)
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if data.IsBot {
		c.metrics.BotGames++
	} else {
		c.metrics.PlayerGames++
	}

	log.Printf("[GAME START] %s vs %s (Bot: %v) - Game ID: %s",
		data.Player1, data.Player2, data.IsBot, event.GameID)
}

func (c *Consumer) handleMove(event GameEvent) {
	var data MoveData
	if err := json.Unmarshal(event.Data, &data); err != nil {
		log.Printf("Failed to unmarshal move data: %v", err)
		return
	}

	c.mu.Lock()
	c.metrics.TotalMoves++
	c.mu.Unlock()

	log.Printf("[MOVE] Game %s - Player %d placed at column %d, row %d",
		event.GameID, data.Player, data.Column, data.Row)
}

func (c *Consumer) handleGameEnd(event GameEvent) {
	var data GameEndData
	if err := json.Unmarshal(event.Data, &data); err != nil {
		log.Printf("Failed to unmarshal game_end data: %v", err)
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.metrics.TotalGames++
	c.metrics.TotalDuration += data.Duration

	if !data.IsDraw && data.Winner != "" {
		c.metrics.WinnerCounts[data.Winner]++
	}

	hourKey := time.Unix(event.Timestamp, 0).Format("2006-01-02-15")
	c.metrics.GamesPerHour[hourKey]++

	avgDuration := float64(0)
	if c.metrics.TotalGames > 0 {
		avgDuration = float64(c.metrics.TotalDuration) / float64(c.metrics.TotalGames)
	}

	result := "Draw"
	if !data.IsDraw {
		result = "Winner: " + data.Winner
	}

	log.Printf("[GAME END] Game %s - %s | Duration: %ds | Moves: %d",
		event.GameID, result, data.Duration, data.Moves)
	log.Printf("[METRICS] Total Games: %d | Avg Duration: %.1fs | Bot Games: %d | Player Games: %d",
		c.metrics.TotalGames, avgDuration, c.metrics.BotGames, c.metrics.PlayerGames)

	c.printTopWinners()

	if c.db != nil {
		c.storeEvent(event, data)
	}
}

func (c *Consumer) printTopWinners() {
	if len(c.metrics.WinnerCounts) == 0 {
		return
	}

	log.Println("[TOP WINNERS]")
	for player, wins := range c.metrics.WinnerCounts {
		log.Printf("  %s: %d wins", player, wins)
	}
}

func (c *Consumer) storeEvent(event GameEvent, data GameEndData) {
	query := `
	INSERT INTO game_analytics (game_id, winner, is_draw, duration, moves, timestamp)
	VALUES ($1, $2, $3, $4, $5, $6)
	ON CONFLICT (game_id) DO NOTHING
	`
	_, err := c.db.Exec(query, event.GameID, data.Winner, data.IsDraw, data.Duration, data.Moves, time.Unix(event.Timestamp, 0))
	if err != nil {
		log.Printf("Failed to store analytics: %v", err)
	}
}

func (c *Consumer) GetMetrics() *Metrics {
	c.mu.Lock()
	defer c.mu.Unlock()

	metricsCopy := &Metrics{
		TotalGames:    c.metrics.TotalGames,
		TotalMoves:    c.metrics.TotalMoves,
		TotalDuration: c.metrics.TotalDuration,
		BotGames:      c.metrics.BotGames,
		PlayerGames:   c.metrics.PlayerGames,
		WinnerCounts:  make(map[string]int),
		GamesPerHour:  make(map[string]int),
	}

	for k, v := range c.metrics.WinnerCounts {
		metricsCopy.WinnerCounts[k] = v
	}
	for k, v := range c.metrics.GamesPerHour {
		metricsCopy.GamesPerHour[k] = v
	}

	return metricsCopy
}

func (c *Consumer) Close() error {
	return c.reader.Close()
}
