package game

const (
	Rows    = 6
	Columns = 7
	Empty   = 0
	Player1 = 1
	Player2 = 2
)

type Board [Rows][Columns]int

type Game struct {
	ID            string
	Board         Board
	CurrentPlayer int
	Player1ID     string
	Player2ID     string
	Player1Name   string
	Player2Name   string
	IsBot         bool
	Winner        int
	IsOver        bool
	IsDraw        bool
	Moves         []Move
	StartTime     int64
	EndTime       int64
}

type Move struct {
	Player int `json:"player"`
	Column int `json:"column"`
	Row    int `json:"row"`
}

func NewGame(id, p1ID, p1Name, p2ID, p2Name string, isBot bool) *Game {
	return &Game{
		ID:            id,
		Board:         Board{},
		CurrentPlayer: Player1,
		Player1ID:     p1ID,
		Player2ID:     p2ID,
		Player1Name:   p1Name,
		Player2Name:   p2Name,
		IsBot:         isBot,
		Winner:        0,
		IsOver:        false,
		IsDraw:        false,
		Moves:         make([]Move, 0),
		StartTime:     0,
		EndTime:       0,
	}
}

func (g *Game) MakeMove(column int) (int, bool) {
	if column < 0 || column >= Columns {
		return -1, false
	}

	row := -1
	for r := Rows - 1; r >= 0; r-- {
		if g.Board[r][column] == Empty {
			row = r
			break
		}
	}

	if row == -1 {
		return -1, false
	}

	g.Board[row][column] = g.CurrentPlayer

	move := Move{
		Player: g.CurrentPlayer,
		Column: column,
		Row:    row,
	}
	g.Moves = append(g.Moves, move)

	if g.CheckWin(row, column) {
		g.Winner = g.CurrentPlayer
		g.IsOver = true
		return row, true
	}

	if g.IsBoardFull() {
		g.IsDraw = true
		g.IsOver = true
		return row, true
	}

	if g.CurrentPlayer == Player1 {
		g.CurrentPlayer = Player2
	} else {
		g.CurrentPlayer = Player1
	}

	return row, true
}

func (g *Game) CheckWin(row, col int) bool {
	player := g.Board[row][col]
	
	count := 0
	for c := 0; c < Columns; c++ {
		if g.Board[row][c] == player {
			count++
			if count >= 4 {
				return true
			}
		} else {
			count = 0
		}
	}

	count = 0
	for r := 0; r < Rows; r++ {
		if g.Board[r][col] == player {
			count++
			if count >= 4 {
				return true
			}
		} else {
			count = 0
		}
	}

	count = 0
	startRow, startCol := row, col
	for startRow > 0 && startCol > 0 {
		startRow--
		startCol--
	}
	for startRow < Rows && startCol < Columns {
		if g.Board[startRow][startCol] == player {
			count++
			if count >= 4 {
				return true
			}
		} else {
			count = 0
		}
		startRow++
		startCol++
	}

	count = 0
	startRow, startCol = row, col
	for startRow > 0 && startCol < Columns-1 {
		startRow--
		startCol++
	}
	for startRow < Rows && startCol >= 0 {
		if g.Board[startRow][startCol] == player {
			count++
			if count >= 4 {
				return true
			}
		} else {
			count = 0
		}
		startRow++
		startCol--
	}

	return false
}

func (g *Game) IsBoardFull() bool {
	for c := 0; c < Columns; c++ {
		if g.Board[0][c] == Empty {
			return false
		}
	}
	return true
}

func (g *Game) GetValidMoves() []int {
	moves := make([]int, 0)
	for c := 0; c < Columns; c++ {
		if g.Board[0][c] == Empty {
			moves = append(moves, c)
		}
	}
	return moves
}

func (g *Game) Clone() *Game {
	clone := &Game{
		ID:            g.ID,
		CurrentPlayer: g.CurrentPlayer,
		Player1ID:     g.Player1ID,
		Player2ID:     g.Player2ID,
		Player1Name:   g.Player1Name,
		Player2Name:   g.Player2Name,
		IsBot:         g.IsBot,
		Winner:        g.Winner,
		IsOver:        g.IsOver,
		IsDraw:        g.IsDraw,
		StartTime:     g.StartTime,
		EndTime:       g.EndTime,
	}
	
	for r := 0; r < Rows; r++ {
		for c := 0; c < Columns; c++ {
			clone.Board[r][c] = g.Board[r][c]
		}
	}
	
	clone.Moves = make([]Move, len(g.Moves))
	copy(clone.Moves, g.Moves)
	
	return clone
}
