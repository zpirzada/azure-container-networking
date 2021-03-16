package ipsm

import (
	"fmt"
	"strings"

	"github.com/Azure/azure-container-networking/npm/util"
)

const (
	createIPset  = "CreateIPSet"
	deleteIPset  = "DeleteIPSet"
	destroyIPset = "DestroyIPSet"
	testIPSet    = "TestIPSet"

	SetCannotBeDestroyedInUseByKernelComponent = 1
	ElemSeperatorNotSupported                  = 2
	SetWithGivenNameDoesNotExist               = 3
	Unknown                                    = 999
)

var (
	ipseterrs = map[int]string{
		SetCannotBeDestroyedInUseByKernelComponent: "Set cannot be destroyed: it is in use by a kernel component",
		ElemSeperatorNotSupported:                  "Syntax error: Elem separator",
		SetWithGivenNameDoesNotExist:               "The set with the given name does not exist",
		Unknown:                                    "Unknown error",
	}

	operationstrings = map[string]string{
		util.IpsetCreationFlag: createIPset,
		util.IpsetDeletionFlag: deleteIPset,
		util.IpsetDestroyFlag:  destroyIPset,
		util.IpsetTestFlag:     testIPSet,
	}
)

func ConvertToNPMErrorWithEntry(operationFlag string, err error, cmd []string) *NPMError {
	for code, description := range ipseterrs {
		errstr := err.Error()
		if strings.Contains(errstr, description) {
			return &NPMError{
				OperationAction: operationstrings[operationFlag],
				FullCmd:         cmd,
				ErrID:           code,
				Err:             err,
			}
		}
	}

	return &NPMError{
		OperationAction: operationstrings[operationFlag],
		FullCmd:         cmd,
		ErrID:           Unknown,
		Err:             err,
	}
}

type NPMError struct {
	OperationAction string
	FullCmd         []string
	ErrID           int
	Err             error
}

func (n *NPMError) Error() string {
	return fmt.Sprintf("Operation [%s] failed with error code [%v], full cmd %v, full error %v", n.OperationAction, n.ErrID, n.FullCmd, n.Err)
}
