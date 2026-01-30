package main

import (
	"analytics/internal/consumer"
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/lib/pq"
)

func main() {
	fmt.Println("4 in a Row - Analytics Service")
	fmt.Println("==============================")

	kafkaBroker := getEnv("KAFKA_BROKER", "localhost:9092")
	kafkaTopic := getEnv("KAFKA_TOPIC", "game-events")
	kafkaGroup := getEnv("KAFKA_GROUP", "analytics-group")
	dbConnStr := getEnv("DATABASE_URL", "")

	var db *sql.DB
	var err error

	if dbConnStr != "" {
		db, err = sql.Open("postgres", dbConnStr)
		if err != nil {
			log.Printf("Warning: Database connection failed: %v (running without persistence)", err)
			db = nil
		} else {
			if err := db.Ping(); err != nil {
				log.Printf("Warning: Database ping failed: %v (running without persistence)", err)
				db = nil
			} else {
				createAnalyticsTable(db)
				log.Println("Connected to database for analytics storage")
			}
		}
	}

	c := consumer.NewConsumer([]string{kafkaBroker}, kafkaTopic, kafkaGroup, db)

	ctx, cancel := context.WithCancel(context.Background())

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Shutdown signal received")
		cancel()
	}()

	log.Printf("Starting Kafka consumer on broker: %s, topic: %s", kafkaBroker, kafkaTopic)
	c.Start(ctx)

	c.Close()
	if db != nil {
		db.Close()
	}
	log.Println("Analytics service stopped")
}

func createAnalyticsTable(db *sql.DB) {
	query := `
	CREATE TABLE IF NOT EXISTS game_analytics (
		game_id VARCHAR(36) PRIMARY KEY,
		winner VARCHAR(50),
		is_draw BOOLEAN DEFAULT FALSE,
		duration INTEGER,
		moves INTEGER,
		timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_analytics_timestamp ON game_analytics(timestamp);
	CREATE INDEX IF NOT EXISTS idx_analytics_winner ON game_analytics(winner);
	`
	_, err := db.Exec(query)
	if err != nil {
		log.Printf("Warning: Failed to create analytics table: %v", err)
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
