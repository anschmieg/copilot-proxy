package rpc

import (
	"fmt"
	"sync"
	"time"
)

// ConnectionID represents a unique identifier for a connection
type ConnectionID uint64

// UserID represents a unique identifier for a user
type UserID uint64

// ZedVersion stores client version information
type ZedVersion struct {
	Version string
}

// Connection represents an active client connection
type Connection struct {
	ID         ConnectionID
	UserID     UserID
	Admin      bool
	ZedVersion ZedVersion
	CreatedAt  time.Time
}

// ConnectionPool manages active connections between clients and the server
type ConnectionPool struct {
	nextConnectionID   ConnectionID
	connections        map[ConnectionID]*Connection
	connectionsByUser  map[UserID]map[ConnectionID]struct{}
	channelSubscribers map[uint64]map[ConnectionID]struct{}
	mu                 sync.RWMutex
}

// NewConnectionPool creates a new connection pool
func NewConnectionPool() *ConnectionPool {
	return &ConnectionPool{
		nextConnectionID:   1,
		connections:        make(map[ConnectionID]*Connection),
		connectionsByUser:  make(map[UserID]map[ConnectionID]struct{}),
		channelSubscribers: make(map[uint64]map[ConnectionID]struct{}),
	}
}

// AddConnection adds a new connection to the pool
func (p *ConnectionPool) AddConnection(userID UserID, admin bool, zedVersion ZedVersion) ConnectionID {
	p.mu.Lock()
	defer p.mu.Unlock()

	connID := p.nextConnectionID
	p.nextConnectionID++

	conn := &Connection{
		ID:         connID,
		UserID:     userID,
		Admin:      admin,
		ZedVersion: zedVersion,
		CreatedAt:  time.Now(),
	}

	p.connections[connID] = conn

	// Track connections by user
	if _, exists := p.connectionsByUser[userID]; !exists {
		p.connectionsByUser[userID] = make(map[ConnectionID]struct{})
	}
	p.connectionsByUser[userID][connID] = struct{}{}

	return connID
}

// RemoveConnection removes a connection from the pool
func (p *ConnectionPool) RemoveConnection(connID ConnectionID) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	conn, exists := p.connections[connID]
	if !exists {
		return fmt.Errorf("connection %d not found", connID)
	}

	// Remove from connections by user
	if userConns, exists := p.connectionsByUser[conn.UserID]; exists {
		delete(userConns, connID)
		if len(userConns) == 0 {
			delete(p.connectionsByUser, conn.UserID)
		}
	}

	// Remove from connections
	delete(p.connections, connID)

	// Remove from channel subscribers
	for channelID, subscribers := range p.channelSubscribers {
		if _, exists := subscribers[connID]; exists {
			delete(subscribers, connID)
			if len(subscribers) == 0 {
				delete(p.channelSubscribers, channelID)
			}
		}
	}

	return nil
}

// GetConnection returns a connection by ID
func (p *ConnectionPool) GetConnection(connID ConnectionID) (*Connection, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	conn, exists := p.connections[connID]
	return conn, exists
}

// UserConnectionIDs returns all connection IDs for a given user
func (p *ConnectionPool) UserConnectionIDs(userID UserID) []ConnectionID {
	p.mu.RLock()
	defer p.mu.RUnlock()

	userConns, exists := p.connectionsByUser[userID]
	if !exists {
		return nil
	}

	connIDs := make([]ConnectionID, 0, len(userConns))
	for connID := range userConns {
		connIDs = append(connIDs, connID)
	}
	return connIDs
}

// SubscribeToChannel subscribes a connection to a channel
func (p *ConnectionPool) SubscribeToChannel(userID UserID, channelID uint64, role int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for connID := range p.connectionsByUser[userID] {
		if _, exists := p.channelSubscribers[channelID]; !exists {
			p.channelSubscribers[channelID] = make(map[ConnectionID]struct{})
		}
		p.channelSubscribers[channelID][connID] = struct{}{}
	}
}

// Reset clears all connections from the pool
func (p *ConnectionPool) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.connections = make(map[ConnectionID]*Connection)
	p.connectionsByUser = make(map[UserID]map[ConnectionID]struct{})
	p.channelSubscribers = make(map[uint64]map[ConnectionID]struct{})
}
