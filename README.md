# 4 in a Row - Real-Time Multiplayer Game

A real-time multiplayer Connect Four game built with GoLang backend, React frontend, WebSocket communication, PostgreSQL persistence, and Kafka analytics.

## Features

- **Real-time Multiplayer**: Play against other players using WebSockets
- **Smart Bot AI**: If no opponent found in 10 seconds, play against a competitive bot using minimax algorithm
- **Reconnection Support**: Reconnect to ongoing games within 30 seconds
- **Leaderboard**: Track wins and losses across all players
- **Kafka Analytics**: Real-time game event streaming for analytics

## Tech Stack

- **Backend**: GoLang with Gin framework
- **Frontend**: React with Vite
- **Database**: PostgreSQL
- **Message Queue**: Apache Kafka
- **Real-time**: WebSocket

## Project Structure

```
four-in-a-row/
├── backend/                 # GoLang backend server
│   ├── cmd/server/          # Main entry point
│   ├── internal/
│   │   ├── bot/             # Minimax AI bot
│   │   ├── game/            # Game logic
│   │   ├── websocket/       # WebSocket hub
│   │   ├── matchmaking/     # Player matching
│   │   ├── handlers/        # HTTP handlers
│   │   └── database/        # PostgreSQL layer
│   └── pkg/kafka/           # Kafka producer
├── analytics/               # Kafka consumer service
├── frontend/                # React frontend
└── docker-compose.yml       # Infrastructure
```

## Prerequisites

- Go 1.21+
- Node.js 18+
- Docker & Docker Compose (for PostgreSQL and Kafka)

## Quick Start

### 1. Start Infrastructure (PostgreSQL & Kafka)

```bash
cd E:\four-in-a-row
docker-compose up -d
```

Wait for services to be healthy (~30 seconds).

### 2. Run Backend Server

```bash
cd E:\four-in-a-row\backend
go mod tidy
go run cmd/server/main.go
```

Server runs at `http://localhost:8080`

### 3. Run Frontend

```bash
cd E:\four-in-a-row\frontend
npm install
npm run dev
```

Frontend runs at `http://localhost:5173`

### 4. (Optional) Run Analytics Service

```bash
cd E:\four-in-a-row\analytics
go mod tidy
KAFKA_BROKER=localhost:9092 go run cmd/analytics/main.go
```

## How to Play

1. Open `http://localhost:5173` in your browser
2. Enter a username and click "Find Match"
3. Wait for an opponent (or bot after 10 seconds)
4. Click on a column to drop your disc
5. Connect 4 discs horizontally, vertically, or diagonally to win!

## Environment Variables

### Backend
| Variable | Default | Description |
|----------|---------|-------------|
| PORT | 8080 | Server port |
| DATABASE_URL | postgres://postgres:postgres@localhost:5432/connect_four?sslmode=disable | PostgreSQL connection |
| KAFKA_BROKER | (empty) | Kafka broker address |
| KAFKA_TOPIC | game-events | Kafka topic name |

### Analytics Service
| Variable | Default | Description |
|----------|---------|-------------|
| KAFKA_BROKER | localhost:9092 | Kafka broker address |
| KAFKA_TOPIC | game-events | Kafka topic name |
| KAFKA_GROUP | analytics-group | Consumer group ID |
| DATABASE_URL | (empty) | PostgreSQL for analytics storage |

## API Endpoints

- `GET /health` - Health check
- `GET /api/leaderboard` - Get top players
- `GET /api/player/:username` - Get player stats
- `GET /api/games` - Get recent games
- `WS /ws` - WebSocket connection

## WebSocket Messages

### Client → Server
```json
{"type": "join", "username": "player1"}
{"type": "move", "column": 3}
{"type": "reconnect", "game_id": "...", "username": "player1"}
```

### Server → Client
```json
{"type": "waiting", "message": "Looking for opponent..."}
{"type": "game_start", "opponent": "player2", "your_turn": true, "player": 1}
{"type": "move", "column": 3, "row": 5, "player": 1, "board": [...]}
{"type": "game_end", "winner": "player1", "reason": "connect4"}
```

## Bot AI Strategy

The bot uses minimax algorithm with alpha-beta pruning:
1. **Immediate Win**: Takes winning moves
2. **Block Opponent**: Blocks opponent's winning moves
3. **Strategic Play**: Prefers center columns and builds winning paths
4. **Depth**: Searches 6 moves ahead

## Game Rules

- 7 columns × 6 rows grid
- Players take turns dropping discs
- Discs fall to lowest available position
- First to connect 4 in any direction wins
- Full board with no winner = draw

## Kafka Events

Events sent to Kafka:
- `game_start` - Game begins
- `move` - Player makes a move
- `game_end` - Game concludes

## Running Without Docker

If you prefer not to use Docker:

1. Install PostgreSQL locally and create database `connect_four`
2. Update `DATABASE_URL` environment variable
3. For Kafka (optional), install locally or skip analytics

The backend works without PostgreSQL (leaderboard won't persist) and without Kafka (analytics disabled).

## Testing

### Test Multiplayer
1. Open two browser tabs
2. Enter different usernames
3. Both players matched together

### Test Bot
1. Open one browser tab
2. Enter username and wait 10 seconds
3. Bot automatically joins

### Test Reconnection
1. Start a game
2. Close browser tab
3. Reopen and enter same username within 30 seconds
4. Game resumes


