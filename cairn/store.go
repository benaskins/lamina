package cairn

import (
	"context"
	"fmt"
	"sync"
)

// CharacterStore is the read interface for character data.
type CharacterStore interface {
	GetCharacter(ctx context.Context, id string) (*Sheet, error)
	ListCharacters(ctx context.Context) ([]*Sheet, error)
}

// CharacterWriter is the write interface, used by projectors only.
type CharacterWriter interface {
	SaveCharacter(ctx context.Context, s *Sheet) error
}

// MemoryCharacterStore is an in-memory CharacterStore and CharacterWriter.
// Use in tests and development.
type MemoryCharacterStore struct {
	mu     sync.RWMutex
	sheets map[string]*Sheet
}

func NewMemoryCharacterStore() *MemoryCharacterStore {
	return &MemoryCharacterStore{sheets: make(map[string]*Sheet)}
}

func (m *MemoryCharacterStore) SaveCharacter(_ context.Context, s *Sheet) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sheets[s.ID] = s
	return nil
}

func (m *MemoryCharacterStore) GetCharacter(_ context.Context, id string) (*Sheet, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sheets[id]
	if !ok {
		return nil, fmt.Errorf("character not found: %s", id)
	}
	return s, nil
}

func (m *MemoryCharacterStore) ListCharacters(_ context.Context) ([]*Sheet, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*Sheet, 0, len(m.sheets))
	for _, s := range m.sheets {
		result = append(result, s)
	}
	return result, nil
}
