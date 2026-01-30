import { useState, useEffect } from 'react';

const API_URL = `http://${window.location.hostname}:8080`;

function Leaderboard() {
    const [leaderboard, setLeaderboard] = useState([]);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState(null);

    const fetchLeaderboard = async () => {
        try {
            setLoading(true);
            const response = await fetch(`${API_URL}/api/leaderboard`);
            if (!response.ok) throw new Error('Failed to fetch');
            const data = await response.json();
            setLeaderboard(data.leaderboard || []);
            setError(null);
        } catch (err) {
            console.error('Failed to fetch leaderboard:', err);
            setError('Unable to load leaderboard');
        } finally {
            setLoading(false);
        }
    };

    useEffect(() => {
        fetchLeaderboard();

        const interval = setInterval(fetchLeaderboard, 10000);
        return () => clearInterval(interval);
    }, []);

    return (
        <div className="card">
            <h2>🏆 Leaderboard</h2>

            {loading && leaderboard.length === 0 && (
                <div className="loading">
                    <div className="spinner"></div>
                </div>
            )}

            {error && leaderboard.length === 0 && (
                <p style={{ color: '#888', textAlign: 'center' }}>{error}</p>
            )}

            {leaderboard.length === 0 && !loading && !error && (
                <p style={{ color: '#888', textAlign: 'center' }}>No games played yet</p>
            )}

            {leaderboard.length > 0 && (
                <ul className="leaderboard-list">
                    {leaderboard.slice(0, 10).map((entry, index) => (
                        <li key={entry.username} className="leaderboard-item">
                            <span className="rank">#{index + 1}</span>
                            <span className="player-name">{entry.username}</span>
                            <span className="wins">{entry.wins} W</span>
                            <span style={{ color: '#ff6b6b', marginLeft: '8px' }}>{entry.losses} L</span>
                        </li>
                    ))}
                </ul>
            )}

            <button
                className="btn btn-secondary"
                onClick={fetchLeaderboard}
                style={{ width: '100%', marginTop: '10px' }}
            >
                Refresh
            </button>
        </div>
    );
}

export default Leaderboard;
