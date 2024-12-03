package main

import (
	"log/slog"
	"sync"
)

type Stats struct {
	connectionsOpened int
	connections       int
	clientBytesRead   int64
	serverBytesRead   int64
	mutex             sync.Mutex
}

func NewStats() *Stats {
	return &Stats{}
}

func (s *Stats) IncrementConnectionsOpened() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.connectionsOpened++
}

func (s *Stats) IncrementConnections() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.connections++
}

func (s *Stats) DecrementConnections() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.connections > 0 {
		s.connections--
	}
}

func (s *Stats) AddClientBytesRead(n int64) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.clientBytesRead += n
}

func (s *Stats) AddServerBytesRead(n int64) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.serverBytesRead += n
}

func (s *Stats) Print() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	slog.Info(
		"stats",
		slog.Int("connections_opened", s.connectionsOpened),
		slog.Int("connections_established", s.connections),
		slog.Int64("client_transferred_bytes", s.clientBytesRead),
		slog.Int64("server_transferred_bytes", s.serverBytesRead),
	)
}

func (s *Stats) Reset() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.connectionsOpened = 0
	s.connections = 0
	s.clientBytesRead = 0
	s.serverBytesRead = 0
}
