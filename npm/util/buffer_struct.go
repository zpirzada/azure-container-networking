package util

import (
	"bytes"
	"fmt"
	"sync"

	"github.com/Azure/azure-container-networking/log"
)

// NpmBuffer is used to build iptables or ipsets buffer
type NpmBuffer struct {
	sync.Mutex
	contents bytes.Buffer
}

func NewNpmBuffer() *NpmBuffer {
	return &NpmBuffer{
		contents: *bytes.NewBuffer(nil),
	}
}

// Reset will reset a given buffer
func (buff *NpmBuffer) Reset() {
	buff.Lock()
	buff.contents.Reset()
	buff.Unlock()
}

// WriteStringLine will write one string line to the buffer
func (buff *NpmBuffer) WriteStringLine(line string) {
	buff.Lock()
	defer buff.Unlock()
	_, err := fmt.Fprintf(&buff.contents, line)
	if err != nil {
		log.Logf("Error: while writing a line to Npmbuffer %s", err.Error())
	}
	buff.contents.WriteString("\n")
}

// WriteByteLine will write one byte line to the buffer
func (buff *NpmBuffer) WriteByteLine(line []byte) error {
	buff.Lock()
	defer buff.Unlock()
	_, err := buff.contents.Write(line)
	if err != nil {
		return fmt.Errorf("error: while writing a line to Npmbuffer with %s", err.Error())
	}
	_, err = buff.contents.WriteString("\n")
	if err != nil {
		return fmt.Errorf("error: while adding newline to Npmbuffer with %s", err.Error())
	}
	return nil
}

// Close will end the buffer with the desired end string
// For iptables this is COMMIT
func (buff *NpmBuffer) Close(closeString string) error {
	buff.Lock()
	defer buff.Unlock()
	_, err := buff.contents.WriteString(closeString)
	if err != nil {
		return fmt.Errorf("error: while adding close string to Npmbuffer with %s", err.Error())
	}
	return nil
}

// Update will replace existing buffer contents
func (buff *NpmBuffer) Update(b bytes.Buffer) {
	buff.Lock()
	buff.contents = b
	buff.Unlock()
}
