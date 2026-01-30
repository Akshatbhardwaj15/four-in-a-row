import { memo } from 'react';

const GameBoard = memo(function GameBoard({ board, onColumnClick, disabled, currentPlayer }) {
    return (
        <div className="board">
            <div className="board-grid">
                {Array(7).fill(null).map((_, colIndex) => (
                    <div
                        key={colIndex}
                        className={`column ${disabled ? 'disabled' : ''}`}
                        onClick={() => !disabled && onColumnClick(colIndex)}
                    >
                        {Array(6).fill(null).map((_, rowIndex) => {
                            const cellValue = board[rowIndex][colIndex];
                            return (
                                <div
                                    key={rowIndex}
                                    className={`cell ${cellValue === 1 ? 'player1' : cellValue === 2 ? 'player2' : ''}`}
                                />
                            );
                        })}
                    </div>
                ))}
            </div>
        </div>
    );
});

export default GameBoard;
