package database

import (
	"database/sql"
	"encoding/json"
	"four-in-a-row/internal/game"
	"log"
	"time"

	_ "github.com/lib/pq"
)

type Database struct {
	DB *sql.DB
}

type GameRecord struct {
	ID          string    `json:"id"`
	Player1     string    `json:"player1"`
	Player2     string    `json:"player2"`
	Winner      string    `json:"winner"`
	IsDraw      bool      `json:"is_draw"`
	IsBot       bool      `json:"is_bot"`
	MovesJSON   string    `json:"moves"`
	Duration    int64     `json:"duration"`
	CompletedAt time.Time `json:"completed_at"`
}

type LeaderboardEntry struct {
	Username string `json:"username"`
	Wins     int    `json:"wins"`
	Losses   int    `json:"losses"`
	Draws    int    `json:"draws"`
	Games    int    `json:"games"`
}

func NewDatabase(connStr string) (*Database, error) {
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	database := &Database{DB: db}
	if err := database.createTables(); err != nil {
		return nil, err
	}

	log.Println("Database connected and tables created")
	return database, nil
}

func (d *Database) createTables() error {
	query := `
	CREATE TABLE IF NOT EXISTS games (
		id VARCHAR(36) PRIMARY KEY,
		player1 VARCHAR(50) NOT NULL,
		player2 VARCHAR(50) NOT NULL,
		winner VARCHAR(50),
		is_draw BOOLEAN DEFAULT FALSE,
		is_bot BOOLEAN DEFAULT FALSE,
		moves JSONB,
		duration INTEGER,
		completed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS leaderboard (
		username VARCHAR(50) PRIMARY KEY,
		wins INTEGER DEFAULT 0,
		losses INTEGER DEFAULT 0,
		draws INTEGER DEFAULT 0,
		games INTEGER DEFAULT 0,
		last_played TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_games_player1 ON games(player1);
	CREATE INDEX IF NOT EXISTS idx_games_player2 ON games(player2);
	CREATE INDEX IF NOT EXISTS idx_games_completed ON games(completed_at);
	CREATE INDEX IF NOT EXISTS idx_leaderboard_wins ON leaderboard(wins DESC);
	`
	_, err := d.DB.Exec(query)
	return err
}

func (d *Database) SaveGame(g *game.Game) error {
	movesJSON, err := json.Marshal(g.Moves)
	if err != nil {
		return err
	}

	winner := ""
	if g.Winner == game.Player1 {
		winner = g.Player1Name
	} else if g.Winner == game.Player2 {
		winner = g.Player2Name
	}

	duration := g.EndTime - g.StartTime

	query := `
	INSERT INTO games (id, player1, player2, winner, is_draw, is_bot, moves, duration, completed_at)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	ON CONFLICT (id) DO NOTHING
	`
	_, err = d.DB.Exec(query, g.ID, g.Player1Name, g.Player2Name, winner, g.IsDraw, g.IsBot, movesJSON, duration, time.Now())
	if err != nil {
		log.Printf("Error saving game: %v", err)
		return err
	}

	if err := d.updateLeaderboard(g); err != nil {
		log.Printf("Error updating leaderboard: %v", err)
	}

	return nil
}

func (d *Database) updateLeaderboard(g *game.Game) error {
	players := []string{g.Player1Name}
	if !g.IsBot {
		players = append(players, g.Player2Name)
	}

	for _, player := range players {
		if player == "" || player == "Bot" {
			continue
		}

		upsertQuery := `
		INSERT INTO leaderboard (username, wins, losses, draws, games, last_played)
		VALUES ($1, 0, 0, 0, 0, CURRENT_TIMESTAMP)
		ON CONFLICT (username) DO NOTHING
		`
		d.DB.Exec(upsertQuery, player)

		var updateQuery string
		if g.IsDraw {
			updateQuery = `
			UPDATE leaderboard 
			SET draws = draws + 1, games = games + 1, last_played = CURRENT_TIMESTAMP
			WHERE username = $1
			`
		} else {
			winnerName := ""
			if g.Winner == game.Player1 {
				winnerName = g.Player1Name
			} else if g.Winner == game.Player2 {
				winnerName = g.Player2Name
			}

			if player == winnerName {
				updateQuery = `
				UPDATE leaderboard 
				SET wins = wins + 1, games = games + 1, last_played = CURRENT_TIMESTAMP
				WHERE username = $1
				`
			} else {
				updateQuery = `
				UPDATE leaderboard 
				SET losses = losses + 1, games = games + 1, last_played = CURRENT_TIMESTAMP
				WHERE username = $1
				`
			}
		}

		if _, err := d.DB.Exec(updateQuery, player); err != nil {
			return err
		}
	}

	return nil
}

func (d *Database) GetLeaderboard(limit int) ([]LeaderboardEntry, error) {
	query := `
	SELECT username, wins, losses, draws, games
	FROM leaderboard
	ORDER BY wins DESC, games ASC
	LIMIT $1
	`
	
	rows, err := d.DB.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := make([]LeaderboardEntry, 0)
	for rows.Next() {
		var entry LeaderboardEntry
		if err := rows.Scan(&entry.Username, &entry.Wins, &entry.Losses, &entry.Draws, &entry.Games); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

func (d *Database) GetPlayerStats(username string) (*LeaderboardEntry, error) {
	query := `
	SELECT username, wins, losses, draws, games
	FROM leaderboard
	WHERE username = $1
	`
	
	var entry LeaderboardEntry
	err := d.DB.QueryRow(query, username).Scan(&entry.Username, &entry.Wins, &entry.Losses, &entry.Draws, &entry.Games)
	if err == sql.ErrNoRows {
		return &LeaderboardEntry{Username: username}, nil
	}
	if err != nil {
		return nil, err
	}

	return &entry, nil
}

func (d *Database) GetRecentGames(limit int) ([]GameRecord, error) {
	query := `
	SELECT id, player1, player2, COALESCE(winner, ''), is_draw, is_bot, COALESCE(moves::text, '[]'), duration, completed_at
	FROM games
	ORDER BY completed_at DESC
	LIMIT $1
	`
	
	rows, err := d.DB.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := make([]GameRecord, 0)
	for rows.Next() {
		var record GameRecord
		if err := rows.Scan(&record.ID, &record.Player1, &record.Player2, &record.Winner, &record.IsDraw, &record.IsBot, &record.MovesJSON, &record.Duration, &record.CompletedAt); err != nil {
			return nil, err
		}
		records = append(records, record)
	}

	return records, nil
}

func (d *Database) Close() error {
	return d.DB.Close()
}
