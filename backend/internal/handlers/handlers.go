package handlers

import (
	"four-in-a-row/internal/database"
	"net/http"

	"github.com/gin-gonic/gin"
)

type Handlers struct {
	DB *database.Database
}

func NewHandlers(db *database.Database) *Handlers {
	return &Handlers{DB: db}
}

func (h *Handlers) GetLeaderboard(c *gin.Context) {
	entries, err := h.DB.GetLeaderboard(20)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch leaderboard"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"leaderboard": entries,
	})
}

func (h *Handlers) GetPlayerStats(c *gin.Context) {
	username := c.Param("username")
	if username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Username is required"})
		return
	}

	stats, err := h.DB.GetPlayerStats(username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch player stats"})
		return
	}

	c.JSON(http.StatusOK, stats)
}

func (h *Handlers) GetRecentGames(c *gin.Context) {
	games, err := h.DB.GetRecentGames(10)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch recent games"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"games": games,
	})
}

func (h *Handlers) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "healthy",
	})
}
