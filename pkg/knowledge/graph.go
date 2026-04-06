package knowledge

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/biswas-dev/aiagents/pkg/agent"
)

// Entity represents a node in the knowledge graph
type Entity struct {
	ID         string            `json:"id"`
	Type       string            `json:"type"`
	Properties map[string]string `json:"properties"`
}

// Relationship represents an edge in the knowledge graph
type Relationship struct {
	SourceID   string            `json:"source_id"`
	TargetID   string            `json:"target_id"`
	Relation   string            `json:"relation"`
	Properties map[string]string `json:"properties,omitempty"`
}

// Graph is a thread-safe in-memory knowledge graph
type Graph struct {
	entities map[string]*Entity
	rels     []Relationship
	mu       sync.RWMutex
}

// NewGraph creates a new empty knowledge graph
func NewGraph() *Graph {
	return &Graph{entities: make(map[string]*Entity)}
}

// AddEntity adds or updates an entity
func (g *Graph) AddEntity(e *Entity) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.entities[e.ID] = e
}

// GetEntity returns an entity by ID
func (g *Graph) GetEntity(id string) (*Entity, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	e, ok := g.entities[id]
	return e, ok
}

// AddRelationship adds a relationship between entities
func (g *Graph) AddRelationship(r Relationship) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.rels = append(g.rels, r)
}

// Neighbors returns entities connected from the given entity
func (g *Graph) Neighbors(id, relation string) []*Entity {
	g.mu.RLock()
	defer g.mu.RUnlock()
	var out []*Entity
	for _, r := range g.rels {
		if r.SourceID == id && (relation == "" || r.Relation == relation) {
			if e, ok := g.entities[r.TargetID]; ok {
				out = append(out, e)
			}
		}
	}
	return out
}

// IncomingNeighbors returns entities that point TO the given entity
func (g *Graph) IncomingNeighbors(id, relation string) []*Entity {
	g.mu.RLock()
	defer g.mu.RUnlock()
	var out []*Entity
	for _, r := range g.rels {
		if r.TargetID == id && (relation == "" || r.Relation == relation) {
			if e, ok := g.entities[r.SourceID]; ok {
				out = append(out, e)
			}
		}
	}
	return out
}

// Query finds entities by type and optional property filters
func (g *Graph) Query(entityType string, filters map[string]string) []*Entity {
	g.mu.RLock()
	defer g.mu.RUnlock()
	var out []*Entity
	for _, e := range g.entities {
		if e.Type != entityType {
			continue
		}
		match := true
		for k, v := range filters {
			if e.Properties[k] != v {
				match = false
				break
			}
		}
		if match {
			out = append(out, e)
		}
	}
	return out
}

// AllEntities returns all entities in the graph
func (g *Graph) AllEntities() []*Entity {
	g.mu.RLock()
	defer g.mu.RUnlock()
	out := make([]*Entity, 0, len(g.entities))
	for _, e := range g.entities {
		out = append(out, e)
	}
	return out
}

// ContextString serializes an entity and its relationships for LLM consumption
func (g *Graph) ContextString(id string) string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	e, ok := g.entities[id]
	if !ok {
		return "entity not found: " + id
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "Entity: %s (type: %s)\n", e.ID, e.Type)
	if len(e.Properties) > 0 {
		fmt.Fprintf(&sb, "Properties:\n")
		for k, v := range e.Properties {
			fmt.Fprintf(&sb, "  %s: %s\n", k, v)
		}
	}
	outgoing := false
	for _, r := range g.rels {
		if r.SourceID == id {
			if !outgoing {
				fmt.Fprintf(&sb, "Outgoing relationships:\n")
				outgoing = true
			}
			targetType := ""
			if t, ok := g.entities[r.TargetID]; ok {
				targetType = t.Type
			}
			fmt.Fprintf(&sb, "  → %s %s (%s)\n", r.Relation, r.TargetID, targetType)
		}
	}
	incoming := false
	for _, r := range g.rels {
		if r.TargetID == id {
			if !incoming {
				fmt.Fprintf(&sb, "Incoming relationships:\n")
				incoming = true
			}
			sourceType := ""
			if s, ok := g.entities[r.SourceID]; ok {
				sourceType = s.Type
			}
			fmt.Fprintf(&sb, "  ← %s from %s (%s)\n", r.Relation, r.SourceID, sourceType)
		}
	}
	return sb.String()
}

// RegisterTools exposes the knowledge graph as agent tools
func (g *Graph) RegisterTools(r *agent.ToolRegistry) {
	r.Register("kg_lookup", "Look up an entity and its relationships in the knowledge graph",
		json.RawMessage(`{"type":"object","properties":{"entity_id":{"type":"string","description":"ID of the entity to look up"}},"required":["entity_id"]}`),
		func(input json.RawMessage) (string, error) {
			var p struct{ EntityID string `json:"entity_id"` }
			json.Unmarshal(input, &p)
			return g.ContextString(p.EntityID), nil
		},
	)

	r.Register("kg_search", "Search for entities by type and optional property filters",
		json.RawMessage(`{"type":"object","properties":{"type":{"type":"string","description":"Entity type to search for"},"filters":{"type":"object","description":"Property key-value filters"}},"required":["type"]}`),
		func(input json.RawMessage) (string, error) {
			var p struct {
				Type    string            `json:"type"`
				Filters map[string]string `json:"filters"`
			}
			json.Unmarshal(input, &p)
			entities := g.Query(p.Type, p.Filters)
			if len(entities) == 0 {
				return "No entities found", nil
			}
			var lines []string
			for _, e := range entities {
				lines = append(lines, fmt.Sprintf("%s (%s): %v", e.ID, e.Type, e.Properties))
			}
			return strings.Join(lines, "\n"), nil
		},
	)

	r.Register("kg_dependencies", "Find all entities that a given entity depends on",
		json.RawMessage(`{"type":"object","properties":{"entity_id":{"type":"string","description":"Entity ID"},"relation":{"type":"string","description":"Relationship type to follow (default: all)"}},"required":["entity_id"]}`),
		func(input json.RawMessage) (string, error) {
			var p struct {
				EntityID string `json:"entity_id"`
				Relation string `json:"relation"`
			}
			json.Unmarshal(input, &p)
			deps := g.Neighbors(p.EntityID, p.Relation)
			if len(deps) == 0 {
				return "No dependencies found", nil
			}
			var lines []string
			for _, d := range deps {
				lines = append(lines, fmt.Sprintf("%s (%s)", d.ID, d.Type))
			}
			return strings.Join(lines, "\n"), nil
		},
	)
}

// Import loads entities and relationships from JSON
func (g *Graph) Import(data []byte) error {
	var payload struct {
		Entities      []Entity       `json:"entities"`
		Relationships []Relationship `json:"relationships"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}
	for i := range payload.Entities {
		g.AddEntity(&payload.Entities[i])
	}
	for _, r := range payload.Relationships {
		g.AddRelationship(r)
	}
	return nil
}

// Export serializes the graph to JSON
func (g *Graph) Export() ([]byte, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	payload := struct {
		Entities      []*Entity      `json:"entities"`
		Relationships []Relationship `json:"relationships"`
	}{
		Entities:      make([]*Entity, 0, len(g.entities)),
		Relationships: g.rels,
	}
	for _, e := range g.entities {
		payload.Entities = append(payload.Entities, e)
	}
	return json.MarshalIndent(payload, "", "  ")
}
