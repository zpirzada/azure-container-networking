// Copyright Microsoft Corp.
// All rights reserved.

package store

// KeyValueStore represents a persistent store of (key,value) pairs.
type KeyValueStore interface {
	Read(key string, value interface{}) error
	Write(key string, value interface{}) error
	Flush() error
}
