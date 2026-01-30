import { useState, useEffect, useCallback } from 'react';
import { useWebSocket } from './hooks/useWebSocket';
import GameBoard from './components/GameBoard';
import Leaderboard from './components/Leaderboard';

function App() {
    const { isConnected, sendMessage, addMessageHandler } = useWebSocket();

    const [username, setUsername] = useState('');
    const [gameState, setGameState] = useState({
        status: 'idle',
        gameId: null,
        board: Array(6).fill(null).map(() => Array(7).fill(0)),
        currentPlayer: 1,
        myPlayer: 1,
        opponent: null,
        isBot: false,
        winner: null,
        reason: null
    });
    const [inputUsername, setInputUsername] = useState('');
    const [message, setMessage] = useState('');

    useEffect(() => {
        const removeHandler = addMessageHandler((msg) => {
            console.log('Received message:', msg);

            switch (msg.type) {
                case 'waiting':
                    setGameState(prev => ({ ...prev, status: 'waiting' }));
                    setMessage(msg.message || 'Looking for an opponent...');
                    break;

                case 'game_start':
                    setGameState(prev => ({
                        ...prev,
                        status: 'playing',
                        gameId: msg.game_id,
                        board: Array(6).fill(null).map(() => Array(7).fill(0)),
                        currentPlayer: msg.your_turn ? msg.player : (msg.player === 1 ? 2 : 1),
                        myPlayer: msg.player,
                        opponent: msg.opponent,
                        isBot: msg.is_bot,
                        winner: null,
                        reason: null
                    }));
                    setMessage(msg.your_turn ? 'Your turn!' : `Waiting for ${msg.opponent}...`);
                    break;

                case 'game_reconnected':
                    if (msg.board) {
                        setGameState(prev => ({
                            ...prev,
                            status: 'playing',
                            gameId: msg.game_id,
                            board: msg.board,
                            currentPlayer: msg.your_turn ? msg.player : (msg.player === 1 ? 2 : 1),
                            myPlayer: msg.player,
                            opponent: msg.opponent,
                            isBot: msg.is_bot,
                            winner: null,
                            reason: null
                        }));
                        setMessage(msg.your_turn ? 'Your turn!' : `Waiting for ${msg.opponent}...`);
                    }
                    break;

                case 'move':
                    if (msg.board) {
                        const isMyTurn = msg.player !== gameState.myPlayer;
                        setGameState(prev => ({
                            ...prev,
                            board: msg.board,
                            currentPlayer: msg.player === 1 ? 2 : 1
                        }));
                        setMessage(isMyTurn ? 'Your turn!' : `Waiting for ${gameState.opponent || 'opponent'}...`);
                    }
                    break;

                case 'game_end':
                    setGameState(prev => ({
                        ...prev,
                        status: 'ended',
                        winner: msg.winner,
                        reason: msg.reason
                    }));
                    if (msg.reason === 'draw') {
                        setMessage("It's a draw!");
                    } else if (msg.winner === username) {
                        setMessage('You won!');
                    } else {
                        setMessage(`${msg.winner} won!`);
                    }
                    break;

                case 'error':
                    setMessage(msg.message || 'An error occurred');
                    break;
            }
        });

        return removeHandler;
    }, [addMessageHandler, username, gameState.myPlayer, gameState.opponent]);

    const handleJoin = useCallback(() => {
        if (!inputUsername.trim()) {
            setMessage('Please enter a username');
            return;
        }
        setUsername(inputUsername.trim());
        sendMessage({ type: 'join', username: inputUsername.trim() });
    }, [inputUsername, sendMessage]);

    const handleMove = useCallback((column) => {
        if (gameState.status !== 'playing') return;

        const isMyTurn = gameState.currentPlayer === gameState.myPlayer;
        if (!isMyTurn) {
            setMessage("Not your turn!");
            return;
        }

        if (gameState.board[0][column] !== 0) {
            setMessage("Column is full!");
            return;
        }

        sendMessage({ type: 'move', column });
    }, [gameState, sendMessage]);

    const handlePlayAgain = useCallback(() => {
        setGameState({
            status: 'idle',
            gameId: null,
            board: Array(6).fill(null).map(() => Array(7).fill(0)),
            currentPlayer: 1,
            myPlayer: 1,
            opponent: null,
            isBot: false,
            winner: null,
            reason: null
        });
        setMessage('');
        sendMessage({ type: 'join', username });
    }, [username, sendMessage]);

    const isMyTurn = gameState.status === 'playing' && gameState.currentPlayer === gameState.myPlayer;

    return (
        <div className="app">
            <header className="header">
                <h1>4 in a Row</h1>
                <p>Real-time multiplayer Connect Four</p>
            </header>

            <div className="container">
                <div className="game-section">
                    {gameState.status === 'idle' && !username && (
                        <div className="card">
                            <h2>Join Game</h2>
                            <div className="login-form">
                                <input
                                    type="text"
                                    placeholder="Enter your username"
                                    value={inputUsername}
                                    onChange={(e) => setInputUsername(e.target.value)}
                                    onKeyDown={(e) => e.key === 'Enter' && handleJoin()}
                                    maxLength={20}
                                />
                                <button
                                    className="btn btn-primary"
                                    onClick={handleJoin}
                                    disabled={!isConnected}
                                >
                                    {isConnected ? 'Find Match' : 'Connecting...'}
                                </button>
                            </div>
                            {!isConnected && (
                                <p style={{ marginTop: '10px', color: '#ff6b6b', fontSize: '0.9rem' }}>
                                    Connecting to server...
                                </p>
                            )}
                        </div>
                    )}

                    {gameState.status === 'waiting' && (
                        <div className="card">
                            <div className="status-message status-waiting">
                                <div className="loading">
                                    <div className="spinner"></div>
                                </div>
                                <p>{message}</p>
                                <p style={{ marginTop: '10px', fontSize: '0.9rem', color: '#888' }}>
                                    If no player joins in 10 seconds, you'll play against a bot
                                </p>
                            </div>
                        </div>
                    )}

                    {(gameState.status === 'playing' || gameState.status === 'ended') && (
                        <div className="card">
                            <div className="game-info">
                                <div className="player-info">
                                    <div className={`player-disc player1 ${gameState.currentPlayer === 1 && gameState.status === 'playing' ? 'current-turn' : ''}`}></div>
                                    <span>{gameState.myPlayer === 1 ? `${username} (You)` : gameState.opponent}</span>
                                </div>
                                <div className="player-info">
                                    <div className={`player-disc player2 ${gameState.currentPlayer === 2 && gameState.status === 'playing' ? 'current-turn' : ''}`}></div>
                                    <span>
                                        {gameState.myPlayer === 2 ? `${username} (You)` : gameState.opponent}
                                        {gameState.isBot && gameState.myPlayer !== 2 && ' (Bot)'}
                                    </span>
                                </div>
                            </div>

                            {gameState.status === 'playing' && (
                                <div className={`status-message ${isMyTurn ? 'status-playing' : 'status-waiting'}`}>
                                    {message}
                                </div>
                            )}

                            <GameBoard
                                board={gameState.board}
                                onColumnClick={handleMove}
                                disabled={gameState.status !== 'playing' || !isMyTurn}
                                currentPlayer={gameState.currentPlayer}
                            />

                            {gameState.status === 'ended' && (
                                <>
                                    <div className={`win-message ${gameState.reason === 'draw' ? 'draw' :
                                            gameState.winner === username ? 'winner' : 'loser'
                                        }`}>
                                        {gameState.reason === 'draw'
                                            ? "It's a Draw!"
                                            : gameState.winner === username
                                                ? '🎉 You Won!'
                                                : `${gameState.winner} Wins!`}
                                    </div>
                                    <button
                                        className="btn btn-primary"
                                        onClick={handlePlayAgain}
                                        style={{ width: '100%', marginTop: '15px' }}
                                    >
                                        Play Again
                                    </button>
                                </>
                            )}
                        </div>
                    )}
                </div>

                <div className="sidebar">
                    {username && (
                        <div className="card">
                            <h2>Player Info</h2>
                            <div className="stats">
                                <div className="stat-item">
                                    <div className="stat-value">{username}</div>
                                    <div className="stat-label">Username</div>
                                </div>
                                <div className="stat-item">
                                    <div className="stat-value" style={{ color: isConnected ? '#2ed573' : '#ff6b6b' }}>
                                        {isConnected ? '●' : '○'}
                                    </div>
                                    <div className="stat-label">{isConnected ? 'Connected' : 'Disconnected'}</div>
                                </div>
                            </div>
                        </div>
                    )}

                    <Leaderboard />
                </div>
            </div>
        </div>
    );
}

export default App;
