package ipsm

import (
	"fmt"
	"strings"

	"github.com/Azure/azure-container-networking/npm/util"
)

const (
	createIPSet  = "CreateIPSet"
	appendIPSet  = "AppendIPSet"
	deleteIPSet  = "DeleteIPSet"
	destroyIPSet = "DestroyIPSet"
	testIPSet    = "TestIPSet"

	SetCannotBeDestroyedInUseByKernelComponent = 1
	ElemSeperatorNotSupported                  = 2
	SetWithGivenNameDoesNotExist               = 3
	SecondElementIsMissing                     = 4
	MissingSecondMandatoryArgument             = 5
	MaximalNumberOfSetsReached                 = 6
	IPSetWithGivenNameAlreadyExists            = 7
	Unknown                                    = 999
)

var (
	ipseterrs = map[int]npmErrorDefiniion{
		SetCannotBeDestroyedInUseByKernelComponent: {"Set cannot be destroyed: it is in use by a kernel component", false},
		ElemSeperatorNotSupported:                  {"Syntax error: Elem separator", false},
		SetWithGivenNameDoesNotExist:               {"The set with the given name does not exist", false},
		SecondElementIsMissing:                     {"Second element is missing from", false},
		MissingSecondMandatoryArgument:             {"Missing second mandatory argument to command", false},
		MaximalNumberOfSetsReached:                 {"Kernel error received: maximal number of sets reached", false}, // no ut
		IPSetWithGivenNameAlreadyExists:            {"Set cannot be created: set with the same name already exists", false},
		Unknown:                                    {"Unknown error", false},
	}

	operationstrings = map[string]string{
		util.IpsetAppendFlag:   appendIPSet,
		util.IpsetCreationFlag: createIPSet,
		util.IpsetDeletionFlag: deleteIPSet,
		util.IpsetDestroyFlag:  destroyIPSet,
		util.IpsetTestFlag:     testIPSet,
	}
)

func ConvertToNPMErrorWithEntry(operationFlag string, err error, cmd []string) *NPMError {
	for code, errDefinition := range ipseterrs {
		errstr := err.Error()
		if strings.Contains(errstr, errDefinition.description) {
			return &NPMError{
				OperationAction: operationstrings[operationFlag],
				IsRetriable:     errDefinition.isRetriable,
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
	IsRetriable     bool
	FullCmd         []string
	ErrID           int
	Err             error
}

type npmErrorDefiniion struct {
	description string
	isRetriable bool
}

func (n *NPMError) Error() string {
	return fmt.Sprintf("Operation [%s] failed with error code [%v], full cmd %v, full error %v", n.OperationAction, n.ErrID, n.FullCmd, n.Err)
}
