package iptm

import (
	"bytes"
	"fmt"
)

var (
	commitBytes = []byte("COMMIT")
	spaceBytes  = []byte(" ")
)

// Below section is taken from https://github.com/kubernetes/kubernetes/blob/master/pkg/util/iptables
// and modified as required for this pkg
// MakeChainLine return an iptables-save/restore formatted chain line given a Chain
func MakeChainLine(chain string) string {
	return fmt.Sprintf(":%s - [0:0]", chain)
}

// GetChainLines parses a table's iptables-save data to find chains in the table.
// It returns a map of iptables.Chain to []byte where the []byte is the chain line
// from save (with counters etc.).
// Note that to avoid allocations memory is SHARED with save.
func GetChainLines(table string, save []byte) map[string]*IptableChain {
	chainsMap := make(map[string]*IptableChain)
	tablePrefix := []byte("*" + string(table))
	readIndex := 0
	// find beginning of table
	for readIndex < len(save) {
		line, n := readLine(readIndex, save)
		readIndex = n
		if bytes.HasPrefix(line, tablePrefix) {
			break
		}
	}
	// parse table lines
	for readIndex < len(save) {
		line, n := readLine(readIndex, save)
		readIndex = n
		if len(line) == 0 {
			continue
		}
		if bytes.HasPrefix(line, commitBytes) || line[0] == '*' {
			break
		} else if line[0] == '#' {
			continue
		} else if line[0] == ':' && len(line) > 1 {
			// We assume that the <line> contains space - chain lines have 3 fields,
			// space delimited. If there is no space, this line will panic.
			spaceIndex := bytes.Index(line, spaceBytes)
			if spaceIndex == -1 {
				panic(fmt.Sprintf("Unexpected chain line in iptables-save output: %v", string(line)))
			}
			chain := string(line[1:spaceIndex])
			if val, ok := chainsMap[chain]; ok {
				val.Data = line
				chainsMap[chain] = val
			} else {
				chainsMap[chain] = &IptableChain{
					Chain: chain,
					Data:  line,
					Rules: make([][]byte, 0),
				}
			}
		} else if line[0] == '-' && len(line) > 1 {
			chain := getChainNameFromRule(line)
			val, ok := chainsMap[chain]
			if !ok {
				val = &IptableChain{
					Chain: chain,
					Data:  []byte{},
					Rules: make([][]byte, 0),
				}
			}
			val.Rules = append(chainsMap[chain].Rules, line)
			chainsMap[chain] = val
		}
	}
	return chainsMap
}

func getChainNameFromRule(byteArray []byte) string {
	spaceIndex1 := bytes.Index(byteArray, spaceBytes)
	if spaceIndex1 == -1 {
		panic(fmt.Sprintf("Unexpected chain line in iptables-save output: %v", string(byteArray)))
	}
	start := spaceIndex1 + 1
	spaceIndex2 := bytes.Index(byteArray[start:], spaceBytes)
	if spaceIndex2 == -1 {
		panic(fmt.Sprintf("Unexpected chain line in iptables-save output: %v", string(byteArray)))
	}
	end := start + spaceIndex2
	return string(byteArray[start:end])
}

func readLine(readIndex int, byteArray []byte) ([]byte, int) {
	currentReadIndex := readIndex

	// consume left spaces
	for currentReadIndex < len(byteArray) {
		if byteArray[currentReadIndex] == ' ' {
			currentReadIndex++
		} else {
			break
		}
	}

	// leftTrimIndex stores the left index of the line after the line is left-trimmed
	leftTrimIndex := currentReadIndex

	// rightTrimIndex stores the right index of the line after the line is right-trimmed
	// it is set to -1 since the correct value has not yet been determined.
	rightTrimIndex := -1

	for ; currentReadIndex < len(byteArray); currentReadIndex++ {
		if byteArray[currentReadIndex] == ' ' {
			// set rightTrimIndex
			if rightTrimIndex == -1 {
				rightTrimIndex = currentReadIndex
			}
		} else if (byteArray[currentReadIndex] == '\n') || (currentReadIndex == (len(byteArray) - 1)) {
			// end of line or byte buffer is reached
			if currentReadIndex <= leftTrimIndex {
				return nil, currentReadIndex + 1
			}
			// set the rightTrimIndex
			if rightTrimIndex == -1 {
				rightTrimIndex = currentReadIndex
				if currentReadIndex == (len(byteArray)-1) && (byteArray[currentReadIndex] != '\n') {
					// ensure that the last character is part of the returned string,
					// unless the last character is '\n'
					rightTrimIndex = currentReadIndex + 1
				}
			}
			// Avoid unnecessary allocation.
			return byteArray[leftTrimIndex:rightTrimIndex], currentReadIndex + 1
		} else {
			// unset rightTrimIndex
			rightTrimIndex = -1
		}
	}
	return nil, currentReadIndex
}
