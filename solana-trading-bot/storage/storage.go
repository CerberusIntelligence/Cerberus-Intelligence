package storage

import (
	"encoding/json"
	"os"
	"sync"

	"solana-trading-bot/types"

	log "github.com/sirupsen/logrus"
)

type State struct {
	Balance   float64                    `json:"balance"`
	Positions map[string]*types.Position `json:"positions"`
	History   []types.Trade              `json:"history"`
}

type Store struct {
	mu   sync.Mutex
	path string
}

func New(path string) *Store {
	return &Store{path: path}
}

func (s *Store) Load() (*State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path)
	if err != nil {
		return nil, err
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}

	if state.Positions == nil {
		state.Positions = make(map[string]*types.Position)
	}
	if state.History == nil {
		state.History = []types.Trade{}
	}

	return &state, nil
}

func (s *Store) Save(state *State) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		log.WithError(err).Error("Failed to marshal state")
		return
	}

	if err := os.WriteFile(s.path, data, 0600); err != nil {
		log.WithError(err).Error("Failed to save state")
	}
}
