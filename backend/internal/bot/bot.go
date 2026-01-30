package bot

import (
	"four-in-a-row/internal/game"
	"math"
	"math/rand"
	"time"
)

const (
	MaxDepth     = 6
	WinScore     = 100000
	BlockScore   = 90000
	ThreeScore   = 100
	TwoScore     = 10
	CenterBonus  = 3
)

type Bot struct {
	Player int
}

func NewBot(player int) *Bot {
	rand.Seed(time.Now().UnixNano())
	return &Bot{Player: player}
}

func (b *Bot) GetMove(g *game.Game) int {
	opponent := game.Player1
	if b.Player == game.Player1 {
		opponent = game.Player2
	}

	for _, col := range g.GetValidMoves() {
		clone := g.Clone()
		clone.CurrentPlayer = b.Player
		row, valid := clone.MakeMove(col)
		if valid && clone.Winner == b.Player {
			return col
		}
		_ = row
	}

	for _, col := range g.GetValidMoves() {
		clone := g.Clone()
		clone.CurrentPlayer = opponent
		row, valid := clone.MakeMove(col)
		if valid && clone.Winner == opponent {
			return col
		}
		_ = row
	}

	bestScore := math.MinInt32
	bestMoves := make([]int, 0)

	for _, col := range g.GetValidMoves() {
		clone := g.Clone()
		clone.CurrentPlayer = b.Player
		_, valid := clone.MakeMove(col)
		if !valid {
			continue
		}

		score := b.minimax(clone, MaxDepth-1, math.MinInt32, math.MaxInt32, false, opponent)

		if score > bestScore {
			bestScore = score
			bestMoves = []int{col}
		} else if score == bestScore {
			bestMoves = append(bestMoves, col)
		}
	}

	if len(bestMoves) == 0 {
		validMoves := g.GetValidMoves()
		if len(validMoves) > 0 {
			return validMoves[rand.Intn(len(validMoves))]
		}
		return 3
	}

	centerPreference := make([]int, 0)
	for _, col := range bestMoves {
		if col == 3 {
			centerPreference = append(centerPreference, col, col, col)
		} else if col == 2 || col == 4 {
			centerPreference = append(centerPreference, col, col)
		} else {
			centerPreference = append(centerPreference, col)
		}
	}

	return centerPreference[rand.Intn(len(centerPreference))]
}

func (b *Bot) minimax(g *game.Game, depth int, alpha, beta int, isMaximizing bool, opponent int) int {
	if g.IsOver || depth == 0 {
		return b.evaluate(g, opponent)
	}

	validMoves := g.GetValidMoves()
	if len(validMoves) == 0 {
		return 0
	}

	if isMaximizing {
		maxScore := math.MinInt32
		for _, col := range validMoves {
			clone := g.Clone()
			clone.CurrentPlayer = b.Player
			_, valid := clone.MakeMove(col)
			if !valid {
				continue
			}

			score := b.minimax(clone, depth-1, alpha, beta, false, opponent)
			maxScore = max(maxScore, score)
			alpha = max(alpha, score)
			if beta <= alpha {
				break
			}
		}
		return maxScore
	} else {
		minScore := math.MaxInt32
		for _, col := range validMoves {
			clone := g.Clone()
			clone.CurrentPlayer = opponent
			_, valid := clone.MakeMove(col)
			if !valid {
				continue
			}

			score := b.minimax(clone, depth-1, alpha, beta, true, opponent)
			minScore = min(minScore, score)
			beta = min(beta, score)
			if beta <= alpha {
				break
			}
		}
		return minScore
	}
}

func (b *Bot) evaluate(g *game.Game, opponent int) int {
	if g.Winner == b.Player {
		return WinScore
	}
	if g.Winner == opponent {
		return -WinScore
	}
	if g.IsDraw {
		return 0
	}

	score := 0

	for c := 0; c < game.Columns; c++ {
		for r := 0; r < game.Rows; r++ {
			if g.Board[r][c] == b.Player {
				score += CenterBonus - abs(c-3)
			} else if g.Board[r][c] == opponent {
				score -= CenterBonus - abs(c-3)
			}
		}
	}

	score += b.evaluateLines(g, opponent)

	return score
}

func (b *Bot) evaluateLines(g *game.Game, opponent int) int {
	score := 0

	for r := 0; r < game.Rows; r++ {
		for c := 0; c <= game.Columns-4; c++ {
			score += b.evaluateWindow(g, r, c, 0, 1, opponent)
		}
	}

	for c := 0; c < game.Columns; c++ {
		for r := 0; r <= game.Rows-4; r++ {
			score += b.evaluateWindow(g, r, c, 1, 0, opponent)
		}
	}

	for r := 0; r <= game.Rows-4; r++ {
		for c := 0; c <= game.Columns-4; c++ {
			score += b.evaluateWindow(g, r, c, 1, 1, opponent)
		}
	}

	for r := 0; r <= game.Rows-4; r++ {
		for c := 3; c < game.Columns; c++ {
			score += b.evaluateWindow(g, r, c, 1, -1, opponent)
		}
	}

	return score
}

func (b *Bot) evaluateWindow(g *game.Game, startRow, startCol, rowDir, colDir int, opponent int) int {
	botCount := 0
	oppCount := 0
	emptyCount := 0

	for i := 0; i < 4; i++ {
		r := startRow + i*rowDir
		c := startCol + i*colDir
		
		if g.Board[r][c] == b.Player {
			botCount++
		} else if g.Board[r][c] == opponent {
			oppCount++
		} else {
			emptyCount++
		}
	}

	if botCount > 0 && oppCount > 0 {
		return 0
	}

	if botCount == 3 && emptyCount == 1 {
		return ThreeScore
	}
	if botCount == 2 && emptyCount == 2 {
		return TwoScore
	}

	if oppCount == 3 && emptyCount == 1 {
		return -ThreeScore * 2
	}
	if oppCount == 2 && emptyCount == 2 {
		return -TwoScore
	}

	return 0
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
