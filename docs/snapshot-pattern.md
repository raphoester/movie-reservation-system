# Snapshot Pattern

Domain objects keep their fields private to enforce invariants, but persistence layers and wire serialization need access to those fields. The Snapshot pattern solves both problems without breaking encapsulation.

**Three moving parts:**

1. `Snapshot` — a plain public struct that mirrors every private field of the domain object. No behaviour, no validation; it is just data.
2. `TakeSnapshot()` — a method on the domain object that copies its state into a `Snapshot`. Used by the storage layer before writing to the DB, and by serializers before sending on the wire.
3. `Restore()` — a method on `Snapshot` that validates the data and reconstructs the domain object. This is the single place where all invariants are enforced — both on the read path (DB → domain) and on the write path (constructors delegate to it).

```go
// Domain object — all fields private
type Screening struct {
    id    string
    seats map[string]string
    // ...
}

// Constructor delegates to Restore() — validation lives in one place only
func NewScreening(id string, seats map[string]string, ...) (*Screening, error) {
    return Snapshot{ID: id, Seats: seats, ...}.Restore()
}

// Expose state for persistence / serialization
func (s Screening) TakeSnapshot() Snapshot {
    return Snapshot{ID: s.id, Seats: s.seats, ...}
}

// Validate and reconstruct — called on both the create and read paths
func (s Snapshot) Restore() (*Screening, error) {
    if s.ID == "" {
        return nil, errors.New("id is required")
    }
    // ... all other invariants ...
    return &Screening{id: s.ID, seats: s.Seats, ...}, nil
}
```

**Storage layer usage:**

```go
// Write: domain → snapshot → DB row
func (s *Store) Insert(ctx context.Context, sc *Screening) error {
    r := fromSnapshot(sc.TakeSnapshot())
    return s.db.Create(&r).Error
}

// Read: DB row → snapshot → domain (validation re-runs automatically)
func (s *Store) FindByID(ctx context.Context, id string) (*Screening, error) {
    var r row
    // ... query ...
    return Snapshot{ID: r.ID, Seats: r.Seats, ...}.Restore()
}
```

Apply this pattern to any domain object whose fields are private.
