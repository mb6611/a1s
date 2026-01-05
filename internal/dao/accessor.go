package dao

import (
	"fmt"
	"reflect"
)

// Accessors maps resource ID strings to their accessor implementations.
type Accessors map[string]Accessor

// accessors holds all registered DAOs.
var accessors = make(Accessors)

// RegisterAccessor adds an accessor to the global registry.
func RegisterAccessor(rid *ResourceID, accessor Accessor) {
	accessors[rid.String()] = accessor
}

// AccessorFor returns a new initialized accessor instance for the given resource ID.
func AccessorFor(f Factory, rid *ResourceID) (Accessor, error) {
	accessor, ok := accessors[rid.String()]
	if !ok {
		return nil, fmt.Errorf("no accessor for: %s", rid.String())
	}

	// Create new instance using reflection
	accessorType := reflect.TypeOf(accessor)
	if accessorType.Kind() == reflect.Ptr {
		accessorType = accessorType.Elem()
	}
	newInstance := reflect.New(accessorType).Interface()

	// Type assert to Accessor
	acc, ok := newInstance.(Accessor)
	if !ok {
		return nil, fmt.Errorf("failed to create accessor for: %s", rid.String())
	}

	// Initialize and return
	acc.Init(f, rid)
	return acc, nil
}

// ListAccessors returns all registered resource IDs.
func ListAccessors() []*ResourceID {
	rids := make([]*ResourceID, 0, len(accessors))
	for key := range accessors {
		// Parse the key back to ResourceID
		rid := &ResourceID{}
		if err := rid.Parse(key); err == nil {
			rids = append(rids, rid)
		}
	}
	return rids
}
