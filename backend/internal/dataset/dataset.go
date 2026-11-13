// Package dataset owns datasets: the grouping of imported tables, and the CSV
// importer that turns uploaded bytes into a catalog table.
//
// A dataset is created with the tables it contains; adding a table to an existing
// dataset later is not exposed yet, though the model supports it.
package dataset

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/catalog"
)

var (
	// ErrNotFound is returned when a dataset ID is not registered.
	ErrNotFound = errors.New("dataset not found")
)

// reservedIDs are never generated, so route literals cannot collide with a
// dataset identifier.
var reservedIDs = map[string]struct{}{"preview": {}}

// ID is a stable dataset identifier derived from the name at creation time.
type ID string

// Source records where a dataset came from.
type Source struct {
	Filename string
	Bytes    int64
}

// Dataset groups the tables imported from one source.
type Dataset struct {
	ID        ID
	Name      string
	CreatedAt time.Time
	Source    Source
	Tables    *catalog.Catalog
}

// RowCount returns the total rows across every table in the dataset.
func (d *Dataset) RowCount() int {
	total := 0
	for _, table := range d.Tables.List() {
		total += table.RowCount()
	}
	return total
}

// Store is a concurrency-safe registry of datasets. Listing order is creation
// order so results are deterministic.
type Store struct {
	mu       sync.RWMutex
	datasets map[ID]*Dataset
	order    []ID
}

func NewStore() *Store {
	return &Store{datasets: make(map[ID]*Dataset)}
}

// Create registers a dataset containing the given tables. Every table is validated
// before any is registered, so a rejected table leaves the store untouched.
func (s *Store) Create(name string, source Source, tables ...*catalog.Table) (*Dataset, error) {
	for _, table := range tables {
		if err := table.Validate(); err != nil {
			return nil, err
		}
	}

	registry := catalog.New()
	for _, table := range tables {
		if err := registry.Create(table); err != nil {
			return nil, err
		}
	}

	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		trimmed = "dataset"
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	created := &Dataset{
		ID:        s.uniqueIDLocked(trimmed),
		Name:      trimmed,
		CreatedAt: time.Now().UTC(),
		Source:    source,
		Tables:    registry,
	}
	s.datasets[created.ID] = created
	s.order = append(s.order, created.ID)

	return created, nil
}

// Get returns the dataset with the given identifier.
func (s *Store) Get(id ID) (*Dataset, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	found, ok := s.datasets[id]
	return found, ok
}

// List returns the registered datasets in creation order.
func (s *Store) List() []*Dataset {
	s.mu.RLock()
	defer s.mu.RUnlock()

	datasets := make([]*Dataset, 0, len(s.order))
	for _, id := range s.order {
		datasets = append(datasets, s.datasets[id])
	}
	return datasets
}

// Len returns the number of registered datasets.
func (s *Store) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.order)
}

// Delete removes a dataset by identifier.
func (s *Store) Delete(id ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.datasets[id]; !ok {
		return fmt.Errorf("%w: id %q", ErrNotFound, id)
	}

	delete(s.datasets, id)
	for i, existing := range s.order {
		if existing == id {
			s.order = append(s.order[:i], s.order[i+1:]...)
			break
		}
	}

	return nil
}

func (s *Store) uniqueIDLocked(name string) ID {
	base := slug(name)
	if base == "" {
		base = "dataset"
	}
	if _, reserved := reservedIDs[base]; reserved {
		base = base + "-dataset"
	}

	candidate := ID(base)
	for suffix := 2; ; suffix++ {
		if _, exists := s.datasets[candidate]; !exists {
			return candidate
		}
		candidate = ID(fmt.Sprintf("%s-%d", base, suffix))
	}
}

// SeedSamples registers the built-in sample datasets. It exists so a fresh install
// has something to inspect before the user imports a file.
func SeedSamples(store *Store) error {
	_, err := store.Create("samples", Source{Filename: "built-in"}, catalog.SampleTables()...)
	return err
}
