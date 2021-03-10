package util

import (
	"bytes"
	"fmt"
	"sync"

	"github.com/Azure/azure-container-networking/log"
)

// NpmBuffer is used to build iptables or ipsets buffer
type NpmBuffer struct {
	contents bytes.Buffer
	mu       sync.Mutex
}

// Reset will reset a given buffer
func (buff *NpmBuffer) Reset() {
	buff.mu.Lock()
	buff.contents.Reset()
	buff.mu.Unlock()
}

// WriteLine will write one line to the buffer
func (buff *NpmBuffer) WriteLine(line string, args ...interface{}) {
	buff.mu.Lock()
	_, err := fmt.Fprintf(&buff.contents, line, args...)
	if err != nil {
		log.Logf("Error: while writing a line to Npmbuffer %s", err.Error())
	}
	buff.contents.WriteString("\n")
	buff.mu.Unlock()
}

// Update will replace existing buffer contents
func (buff *NpmBuffer) Update(b bytes.Buffer) {
	buff.mu.Lock()
	buff.contents = b
	buff.mu.Unlock()
}
