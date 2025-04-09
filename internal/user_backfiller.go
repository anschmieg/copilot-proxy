package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// UserBackfiller periodically fetches additional user data from GitHub
// to enrich our database with more user information.
type UserBackfiller struct {
	client        *http.Client
	accessToken   string
	db            Database
	interval      time.Duration
	requestsMutex sync.Mutex
	requestsHour  time.Time
	requestsCount int
	maxRequests   int
}

// Database interface defines methods needed by the user backfiller
type Database interface {
	GetUsersNeedingBackfill(limit int) ([]UserToBackfill, error)
	UpdateUserFromGitHub(userID uint64, userData GitHubUserData) error
}

// UserToBackfill represents a user that needs additional data
type UserToBackfill struct {
	ID         uint64
	GitHubID   int
	GitHubName string
}

// GitHubUserData represents user data from GitHub
type GitHubUserData struct {
	Name        string    `json:"name"`
	Email       string    `json:"email"`
	Company     string    `json:"company"`
	Blog        string    `json:"blog"`
	Location    string    `json:"location"`
	Bio         string    `json:"bio"`
	TwitterUser string    `json:"twitter_username"`
	CreatedAt   time.Time `json:"created_at"`
}

// NewUserBackfiller creates a new user backfiller
func NewUserBackfiller(db Database, accessToken string) *UserBackfiller {
	return &UserBackfiller{
		client:      &http.Client{Timeout: 10 * time.Second},
		accessToken: accessToken,
		db:          db,
		interval:    time.Hour,
		maxRequests: 5000, // GitHub API limit (5000 requests/hour)
	}
}

// Start begins the backfilling process
func (b *UserBackfiller) Start(ctx context.Context) {
	ticker := time.NewTicker(b.interval)
	defer ticker.Stop()

	// Process immediately on start
	b.processBackfill(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			b.processBackfill(ctx)
		}
	}
}

// processBackfill processes a batch of users that need backfilling
func (b *UserBackfiller) processBackfill(ctx context.Context) {
	users, err := b.db.GetUsersNeedingBackfill(50)
	if err != nil {
		fmt.Printf("Error getting users to backfill: %v\n", err)
		return
	}

	if len(users) == 0 {
		return // No users to backfill
	}

	for _, user := range users {
		if !b.canMakeRequest() {
			fmt.Println("GitHub API rate limit reached, waiting until next hour")
			return
		}

		userData, err := b.fetchGitHubUserData(ctx, user.GitHubID)
		if err != nil {
			fmt.Printf("Error fetching data for user %d: %v\n", user.ID, err)
			continue
		}

		if err := b.db.UpdateUserFromGitHub(user.ID, userData); err != nil {
			fmt.Printf("Error updating user %d: %v\n", user.ID, err)
		}
	}
}

// fetchGitHubUserData fetches user data from GitHub API
func (b *UserBackfiller) fetchGitHubUserData(ctx context.Context, githubID int) (GitHubUserData, error) {
	url := fmt.Sprintf("https://api.github.com/user/%d", githubID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return GitHubUserData{}, err
	}

	req.Header.Set("Authorization", "Bearer "+b.accessToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := b.client.Do(req)
	if err != nil {
		return GitHubUserData{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return GitHubUserData{}, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var userData GitHubUserData
	if err := json.NewDecoder(resp.Body).Decode(&userData); err != nil {
		return GitHubUserData{}, err
	}

	return userData, nil
}

// canMakeRequest checks if we can make another GitHub API request based on rate limits
func (b *UserBackfiller) canMakeRequest() bool {
	b.requestsMutex.Lock()
	defer b.requestsMutex.Unlock()

	now := time.Now()
	currentHour := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location())

	if b.requestsHour != currentHour {
		// Reset counter for a new hour
		b.requestsHour = currentHour
		b.requestsCount = 0
	}

	if b.requestsCount >= b.maxRequests {
		return false
	}

	b.requestsCount++
	return true
}
