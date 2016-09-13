// Copyright Microsoft Corp.
// All rights reserved.

package store

import (
	"encoding/json"
	"os"
	"sync"
)

const (
	// Default file name for backing persistent store.
	defaultFileName = "aqua.json"
)

// jsonFileStore is an implementation of KeyValueStore using a local JSON file.
type jsonFileStore struct {
	fileName string
	data     map[string]*json.RawMessage
	sync.Mutex
}

// NewJsonFileStore creates a new jsonFileStore object, accessed as a KeyValueStore.
func NewJsonFileStore(fileName string) (KeyValueStore, error) {
	if fileName == "" {
		fileName = defaultFileName
	}

	kvs := &jsonFileStore{
		fileName: fileName,
		data:     make(map[string]*json.RawMessage),
	}

	// Open and parse the file if it exists.
	file, err := os.Open(fileName)
	if err != nil {
		if os.IsNotExist(err) {
			return kvs, nil
		}
		return nil, err
	}

	// Decode to raw JSON messages. Object instantiation happens on read.
	err = json.NewDecoder(file).Decode(&kvs.data)
	if err != nil {
		return nil, err
	}

	err = file.Close()
	if err != nil {
		return nil, err
	}

	return kvs, nil
}

// Read restores the value for the given key from persistent store.
func (kvs *jsonFileStore) Read(key string, value interface{}) error {
	kvs.Lock()
	defer kvs.Unlock()

	raw := kvs.data[key]
	if raw == nil {
		return ErrKeyNotFound
	}

	return json.Unmarshal(*raw, value)
}

// Write saves the given key value pair to persistent store.
func (kvs *jsonFileStore) Write(key string, value interface{}) error {
	kvs.Lock()
	defer kvs.Unlock()

	var raw json.RawMessage
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}

	kvs.data[key] = &raw

	return kvs.flush()
}

// Flush commits in-memory state to backing store.
func (kvs *jsonFileStore) Flush() error {
	kvs.Lock()
	defer kvs.Unlock()

	return kvs.flush()
}

// Lock-free flush for internal callers.
func (kvs *jsonFileStore) flush() error {
	file, err := os.Create(kvs.fileName)
	if err != nil {
		return err
	}

	buf, err := json.MarshalIndent(&kvs.data, "", "\t")
	if err != nil {
		return err
	}

	_, err = file.Write(buf)
	if err != nil {
		return err
	}

	return file.Close()
}
