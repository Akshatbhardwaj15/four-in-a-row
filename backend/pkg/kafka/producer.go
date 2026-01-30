package kafka

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
)

type Producer struct {
	writer  *kafka.Writer
	enabled bool
}

type GameEvent struct {
	Type      string      `json:"type"`
	GameID    string      `json:"game_id"`
	Timestamp int64       `json:"timestamp"`
	Data      interface{} `json:"data"`
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

func NewProducer(brokers []string, topic string) *Producer {
	if len(brokers) == 0 || brokers[0] == "" {
		log.Println("Kafka disabled: no brokers configured")
		return &Producer{enabled: false}
	}

	writer := &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Topic:        topic,
		Balancer:     &kafka.LeastBytes{},
		BatchSize:    1,
		BatchTimeout: 10 * time.Millisecond,
		RequiredAcks: kafka.RequireOne,
	}

	log.Printf("Kafka producer initialized for topic: %s", topic)
	return &Producer{
		writer:  writer,
		enabled: true,
	}
}

func (p *Producer) SendGameStart(gameID, player1, player2 string, isBot bool) {
	if !p.enabled {
		return
	}

	event := GameEvent{
		Type:      "game_start",
		GameID:    gameID,
		Timestamp: time.Now().Unix(),
		Data: GameStartData{
			Player1: player1,
			Player2: player2,
			IsBot:   isBot,
		},
	}

	p.send(event)
}

func (p *Producer) SendMove(gameID string, player, column, row int) {
	if !p.enabled {
		return
	}

	event := GameEvent{
		Type:      "move",
		GameID:    gameID,
		Timestamp: time.Now().Unix(),
		Data: MoveData{
			Player: player,
			Column: column,
			Row:    row,
		},
	}

	p.send(event)
}

func (p *Producer) SendGameEnd(gameID, winner string, isDraw bool, duration int64, moves int) {
	if !p.enabled {
		return
	}

	event := GameEvent{
		Type:      "game_end",
		GameID:    gameID,
		Timestamp: time.Now().Unix(),
		Data: GameEndData{
			Winner:   winner,
			IsDraw:   isDraw,
			Duration: duration,
			Moves:    moves,
		},
	}

	p.send(event)
}

func (p *Producer) send(event GameEvent) {
	data, err := json.Marshal(event)
	if err != nil {
		log.Printf("Failed to marshal Kafka event: %v", err)
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := p.writer.WriteMessages(ctx, kafka.Message{
			Key:   []byte(event.GameID),
			Value: data,
		})

		if err != nil {
			log.Printf("Failed to send Kafka message: %v", err)
		} else {
			log.Printf("Kafka event sent: %s for game %s", event.Type, event.GameID)
		}
	}()
}

func (p *Producer) Close() error {
	if p.enabled && p.writer != nil {
		return p.writer.Close()
	}
	return nil
}
