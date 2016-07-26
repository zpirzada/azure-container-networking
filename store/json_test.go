// Copyright Microsoft Corp.
// All rights reserved.

package store

import (
	"os"
	"testing"
)

const (
	// File name used for test store.
	testFileName = "test.json"

	// Keys used during tests.
	testKey1 = "key1"
	testKey2 = "key2"
)

// Type for testing aggregate encoding.
type testType1 struct {
	Field1 string
	Field2 int
}

// Tests that the key value pairs are reinstantiated correctly from a pre-existing JSON encoded file.
func TestKeyValuePairsAreReinstantiatedFromJSONFile(t *testing.T) {
	var encodedPair = `{"key1":{"Field1":"test","Field2":42}}`
	var expectedValue = testType1{"test", 42}
	var actualValue testType1

	// Create a JSON file containing the encoded pair.
	file, err := os.Create(testFileName)
	if err != nil {
		t.Fatalf("Failed to create file %v", err)
	}

	_, err = file.WriteString(encodedPair)
	if err != nil {
		t.Fatalf("Failed to write to file %v", err)
	}

	file.Close()
	defer os.Remove(testFileName)

	// Create the store, initialized using the JSON file.
	kvs, err := NewJsonFileStore(testFileName)
	if err != nil {
		t.Fatalf("Failed to create KeyValueStore %v\n", err)
	}

	// Read the pair.
	err = kvs.Read(testKey1, &actualValue)
	if err != nil {
		t.Fatalf("Failed to read from store %v", err)
	}

	// Fail if the read pair does not match the expected pair.
	if actualValue != expectedValue {
		t.Errorf("Read pair (%v, %v) does not match the expected pair (%v, %v)",
			testKey1, actualValue, testKey1, expectedValue)
	}
}

// Tests that the key value pairs written to the store are persisted correctly in JSON encoded file.
func TestKeyValuePairsArePersistedToJSONFile(t *testing.T) {
	var writtenValue = testType1{"test", 42}
	var expectedPair = `{"key1":{"Field1":"test","Field2":42}}`
	var actualPair string

	// Create the store.
	kvs, err := NewJsonFileStore(testFileName)
	if err != nil {
		t.Fatalf("Failed to create KeyValueStore %v\n", err)
	}

	// Write the value.
	err = kvs.Write(testKey1, &writtenValue)
	if err != nil {
		t.Fatalf("Failed to write to store %v", err)
	}

	// Read the persisted file contents.
	file, err := os.Open(testFileName)
	if err != nil {
		t.Fatalf("Failed to open file %v", err)
	}

	data := make([]byte, 100)
	n, err := file.Read(data)
	if err != nil {
		t.Fatalf("Failed to read from file %v", err)
	}
	actualPair = string(data[:n-1])

	file.Close()
	defer os.Remove(testFileName)

	// Fail if the contents do not match expected JSON encoding.
	if actualPair != expectedPair {
		t.Errorf("Read pair (%v, %v) does not match the expected pair (%v, %v)",
			testKey1, actualPair, testKey1, expectedPair)
	}
}

// Tests that key value pairs are written and read back correctly.
func TestKeyValuePairsAreWrittenAndReadCorrectly(t *testing.T) {
	var writtenValue = testType1{"test", 42}
	var anotherValue = testType1{"any", 14}
	var readValue testType1

	// Create the store.
	kvs, err := NewJsonFileStore(testFileName)
	if err != nil {
		t.Fatalf("Failed to create KeyValueStore %v\n", err)
	}

	// Write a key value pair.
	err = kvs.Write(testKey1, &writtenValue)
	if err != nil {
		t.Fatalf("Failed to write to store %v", err)
	}

	// Write a second key value pair.
	err = kvs.Write(testKey2, &anotherValue)
	if err != nil {
		t.Fatalf("Failed to write to store %v", err)
	}

	// Read the first pair back.
	err = kvs.Read(testKey1, &readValue)
	if err != nil {
		t.Fatalf("Failed to read from store %v", err)
	}

	// Fail if the read pair does not match the written pair.
	if readValue != writtenValue {
		t.Errorf("Read pair (%v, %v) does not match the written pair (%v, %v)",
			testKey1, readValue, testKey1, writtenValue)
	}

	// Cleanup.
	os.Remove(testFileName)
}
