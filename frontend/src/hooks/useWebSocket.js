import { useState, useEffect, useCallback, useRef } from 'react';

const getWsUrl = () => {
    if (import.meta.env.VITE_WS_URL) {
        return import.meta.env.VITE_WS_URL;
    }
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const host = window.location.host;
    return `${protocol}//${host}/ws`;
};
const WS_URL = getWsUrl();

export function useWebSocket() {
    const [isConnected, setIsConnected] = useState(false);
    const [lastMessage, setLastMessage] = useState(null);
    const wsRef = useRef(null);
    const reconnectTimeoutRef = useRef(null);
    const messageHandlersRef = useRef(new Set());

    const connect = useCallback(() => {
        if (wsRef.current?.readyState === WebSocket.OPEN) {
            return;
        }

        try {
            wsRef.current = new WebSocket(WS_URL);

            wsRef.current.onopen = () => {
                console.log('WebSocket connected');
                setIsConnected(true);
            };

            wsRef.current.onclose = () => {
                console.log('WebSocket disconnected');
                setIsConnected(false);

                reconnectTimeoutRef.current = setTimeout(() => {
                    console.log('Attempting to reconnect...');
                    connect();
                }, 3000);
            };

            wsRef.current.onerror = (error) => {
                console.error('WebSocket error:', error);
            };

            wsRef.current.onmessage = (event) => {
                try {
                    const message = JSON.parse(event.data);
                    setLastMessage(message);

                    messageHandlersRef.current.forEach(handler => {
                        handler(message);
                    });
                } catch (e) {
                    console.error('Failed to parse message:', e);
                }
            };
        } catch (error) {
            console.error('Failed to connect:', error);
        }
    }, []);

    const disconnect = useCallback(() => {
        if (reconnectTimeoutRef.current) {
            clearTimeout(reconnectTimeoutRef.current);
        }
        if (wsRef.current) {
            wsRef.current.close();
            wsRef.current = null;
        }
    }, []);

    const sendMessage = useCallback((message) => {
        if (wsRef.current?.readyState === WebSocket.OPEN) {
            wsRef.current.send(JSON.stringify(message));
        } else {
            console.error('WebSocket is not connected');
        }
    }, []);

    const addMessageHandler = useCallback((handler) => {
        messageHandlersRef.current.add(handler);
        return () => {
            messageHandlersRef.current.delete(handler);
        };
    }, []);

    useEffect(() => {
        connect();
        return () => disconnect();
    }, [connect, disconnect]);

    return {
        isConnected,
        lastMessage,
        sendMessage,
        addMessageHandler,
        connect,
        disconnect
    };
}
