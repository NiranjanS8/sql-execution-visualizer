// Package catalog owns the in-memory tables QueryLens can query. It defines the
// typed value model, schemas, tables, and a concurrency-safe registry.
//
// Nothing here knows about HTTP, SQL, or the frontend: the API, planner, and
// execution layers depend on this package, never the reverse.
package catalog

import (
	"fmt"
	"strings"
	"sync"
)

// Catalog is a concurrency-safe registry of tables. Listing order is registration
// order, so results do not depend on Go map iteration order.
type Catalog struct {
	mu     sync.RWMutex
	tables map[TableID]*Table
	names  map[string]TableID
	order  []TableID
}

func New() *Catalog {
	return &Catalog{
		tables: make(map[TableID]*Table),
		names:  make(map[string]TableID),
	}
}

// Create validates and registers a table. IDs are unique, and names are unique
// case-insensitively because SQL identifiers are.
func (c *Catalog) Create(table *Table) error {
	if err := table.Validate(); err != nil {
		return err
	}

	name := normalizeName(table.Name)

	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.tables[table.ID]; exists {
		return fmt.Errorf("%w: id %q", ErrTableExists, table.ID)
	}
	if existing, exists := c.names[name]; exists {
		return fmt.Errorf("%w: name %q is already used by %q", ErrTableExists, table.Name, existing)
	}

	c.tables[table.ID] = table
	c.names[name] = table.ID
	c.order = append(c.order, table.ID)

	return nil
}

// Get returns the table with the given identifier.
func (c *Catalog) Get(id TableID) (*Table, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	table, ok := c.tables[id]
	return table, ok
}

// List returns the registered tables in registration order.
func (c *Catalog) List() []*Table {
	c.mu.RLock()
	defer c.mu.RUnlock()

	tables := make([]*Table, 0, len(c.order))
	for _, id := range c.order {
		tables = append(tables, c.tables[id])
	}
	return tables
}

// Len returns the number of registered tables.
func (c *Catalog) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.order)
}

// Delete removes a table by identifier.
func (c *Catalog) Delete(id TableID) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	table, ok := c.tables[id]
	if !ok {
		return fmt.Errorf("%w: id %q", ErrTableNotFound, id)
	}

	delete(c.tables, id)
	delete(c.names, normalizeName(table.Name))
	for i, existing := range c.order {
		if existing == id {
			c.order = append(c.order[:i], c.order[i+1:]...)
			break
		}
	}

	return nil
}

func normalizeName(name string) string {
	return strings.ToLower(name)
}
