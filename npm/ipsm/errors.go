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
	SetToBeAddedDeletedTestedDoesNotExist      = 8
	Unknown                                    = 999
)

var (
	ipseterrs = map[int]npmErrorDefinition{
		SetCannotBeDestroyedInUseByKernelComponent: {"Set cannot be destroyed: it is in use by a kernel component", npmErrorRetrySettings{Create: false, Append: false, Delete: false}},
		ElemSeperatorNotSupported:                  {"Syntax error: Elem separator", npmErrorRetrySettings{Create: false, Append: false, Delete: false}},
		SetWithGivenNameDoesNotExist:               {"The set with the given name does not exist", npmErrorRetrySettings{Create: false, Append: false, Delete: false}},
		SecondElementIsMissing:                     {"Second element is missing from", npmErrorRetrySettings{Create: false, Append: false, Delete: false}},
		MissingSecondMandatoryArgument:             {"Missing second mandatory argument to command", npmErrorRetrySettings{Create: false, Append: false, Delete: false}},
		MaximalNumberOfSetsReached:                 {"Kernel error received: maximal number of sets reached", npmErrorRetrySettings{Create: false, Append: false, Delete: false}}, // no ut
		IPSetWithGivenNameAlreadyExists:            {"Set cannot be created: set with the same name already exists", npmErrorRetrySettings{Create: false, Append: false, Delete: false}},
		SetToBeAddedDeletedTestedDoesNotExist:      {"Set to be added/deleted/tested as element does not exist", npmErrorRetrySettings{Create: false, Append: false, Delete: false}},
		Unknown:                                    {"Unknown error", npmErrorRetrySettings{Create: false, Append: false, Delete: false}},
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
				IsRetriable:     errDefinition.IsRetriable(operationFlag),
				FullCmd:         cmd,
				ErrID:           code,
				Err:             err,
			}
		}
	}

	return &NPMError{
		OperationAction: operationstrings[operationFlag],
		IsRetriable:     false,
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

func (n *npmErrorDefinition) IsRetriable(operationAction string) bool {
	switch operationAction {
	case createIPSet:
		return n.isRetriable.Create
	case deleteIPSet:
		return n.isRetriable.Delete
	case appendIPSet:
		return n.isRetriable.Append
	}
	return false
}

type npmErrorDefinition struct {
	description string
	isRetriable npmErrorRetrySettings
}

type npmErrorRetrySettings struct {
	Create bool
	Append bool
	Delete bool
}

func (n *NPMError) Error() string {
	return fmt.Sprintf("Operation [%s] failed with error code [%v], full cmd %v, full error %v", n.OperationAction, n.ErrID, n.FullCmd, n.Err)
}
