package main

import (
	"encoding/json"
	"reflect"
	"sync"
	"time"

	"github.com/youtube/vitess/go/vt/topo"
)

// This file includes the support for serving topo data to an ajax-based
// front-end. There are three ways we collect data:
// - reading topology records that don't change too often, caching them
//   with a somewhat big TTL, and have a 'flush' command in case it's needed.
//   (list of cells, tablets in a cell/shard, ...)
// - subscribing to topology change channels (serving graph)
// - establishing streaming connections to vttablets to get up-to-date
//   health reports.

// KnownCells is the toplevel stuct we convert to json to return to clients
type KnownCells struct {
	// Version is the version number for that object. If it hasn't changed,
	// the content is the same.
	Version int

	// Cells is the list of Known Cells for this topology
	Cells []string
}

type knownCellsCache struct {
	ts topo.Server

	mu         sync.Mutex
	timestamp  time.Time
	knownCells KnownCells
	result     []byte
}

func newKnownCellsCache(ts topo.Server) *knownCellsCache {
	return &knownCellsCache{
		ts: ts,
	}
}

func (kcc *knownCellsCache) get() ([]byte, error) {
	kcc.mu.Lock()
	defer kcc.mu.Unlock()

	now := time.Now()
	if now.Sub(kcc.timestamp) < 5*time.Minute {
		return kcc.result, nil
	}

	cells, err := kcc.ts.GetKnownCells()
	if err != nil {
		return nil, err
	}
	if !reflect.DeepEqual(cells, kcc.knownCells.Cells) {
		kcc.knownCells.Cells = cells
		kcc.knownCells.Version++
	}
	kcc.result, err = json.MarshalIndent(&kcc.knownCells, "", "  ")
	if err != nil {
		return nil, err
	}
	kcc.timestamp = now

	return kcc.result, nil
}

func (kcc *knownCellsCache) flush() {
	kcc.mu.Lock()
	defer kcc.mu.Unlock()

	// we reset timestamp and content, so the Version will increase again
	// and force a client refresh, even if the data is the same.
	kcc.timestamp = time.Time{}
	kcc.knownCells.Cells = nil
}
