// Copyright Microsoft Corp.
// All rights reserved.

package store

import (
	"fmt"
	"time"
)

// KeyValueStore represents a persistent store of (key,value) pairs.
type KeyValueStore interface {
	Read(key string, value interface{}) error
	Write(key string, value interface{}) error
	Flush() error
	Lock(block bool) error
	Unlock() error
	GetModificationTime() (time.Time, error)
}

var (
	// Errors returned by KeyValueStore methods.
	ErrKeyNotFound    = fmt.Errorf("Key not found")
	ErrStoreLocked    = fmt.Errorf("Store is locked")
	ErrStoreNotLocked = fmt.Errorf("Store is not locked")
)
