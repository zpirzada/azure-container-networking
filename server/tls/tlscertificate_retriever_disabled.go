// Copyright 2020 Microsoft. All rights reserved.

// +build linux

// This file is to ensure a implementation for NewTlsCertificateRetriever exists
// so we avoid a compilation error

package tls

import (
	"fmt"
)

// NewTlsCertificateRetriever should not be called
// Linux currently uses tls file certificate retriever
// this indicates the caller has not set the Tls Certificate Path in the server settings
func NewTlsCertificateRetriever(settings TlsSettings) (TlsCertificateRetriever, error) {
	if settings.TLSCertificatePath == "" {
		return nil, fmt.Errorf("TLS certificate file path not set")
	}
	return nil, fmt.Errorf("Not implemented, only windows and linux is supported")
}
