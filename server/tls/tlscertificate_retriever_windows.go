// Copyright 2020 Microsoft. All rights reserved.

package tls

import (
	"crypto"
	"crypto/x509"
	"fmt"
	certtostore "github.com/Azure/azure-container-networking/server/tls/customcerttostore"
	"golang.org/x/sys/windows"
)

type windowsTlsCertificateRetriever struct {
	certStore   *certtostore.WinCertStore
	certContext *windows.CertContext
	settings    TlsSettings
}

// Get certificate reads from the windows cert store
// it depends on the TlsCertificateSubjectName being set
// in the server settings to retrieve the cert
func (wtls *windowsTlsCertificateRetriever) GetCertificate() (*x509.Certificate, error) {
	if wtls.settings.TLSSubjectName == "" {
		return nil, fmt.Errorf("Certificate subject name is empty in the settings")
	}
	cert, certContext, err := wtls.certStore.CertBySubjectName(wtls.settings.TLSSubjectName)
	if err != nil {
		return nil, fmt.Errorf("Retrieving certificate with subject name %s from cert store returned error %+v",wtls.settings.TLSSubjectName, err)
	}
	if cert == nil {
		return nil, fmt.Errorf("Call to cert store succeeded but gave a empty certificate")
	}
	if certContext == nil {
		return nil, fmt.Errorf("Cert context returned empty")
	}
	wtls.certContext = certContext
	return cert, nil
}

// Get private key retrieves the private key from the windows cert store
// it returns a private key that implements crypto.Signer with an RSA based key
func (wtls *windowsTlsCertificateRetriever) GetPrivateKey() (crypto.PrivateKey, error) {
	certKey, err := wtls.certStore.CertKey(wtls.certContext)

	if err != nil {
		return nil, fmt.Errorf("Retrieving private key returned error %+v ", err)
	}
	if certKey == nil {
		return nil, fmt.Errorf("Empty private key returned")
	}
	return certKey, nil
}

// Open cert store opens the cert store
func (wtls *windowsTlsCertificateRetriever) openCertStore() error {
	certStore, err := certtostore.OpenWinCertStore(certtostore.ProviderMSSoftware, "0", nil, nil, false, true)
	if err != nil {
		return fmt.Errorf("Error opening cert store %+v", err)
	}
	if certStore == nil {
		return fmt.Errorf("Empty cert store recieved %+v", err)
	}
	wtls.certStore = certStore
	return nil
}

// NewWindowsTlsCertificateRetriever creates a TlsCertificateRetriever
// NewFileTlsCertificateRetriever depends on the pfx being available on the windows cert store
func NewTlsCertificateRetriever(settings TlsSettings) (TlsCertificateRetriever, error) {
	windowsCertStoreRetriever := &windowsTlsCertificateRetriever{
		settings: settings,
	}
	if err := windowsCertStoreRetriever.openCertStore(); err != nil {
		return nil, fmt.Errorf("Failed to open cert store with %+v:", err)
	}
	return windowsCertStoreRetriever, nil
}
