// Copyright 2020 Microsoft. All rights reserved.

package tls

// TlsCertificateSettins - Details related to the TLS certificate.
type TlsSettings struct {
	TLSSubjectName     string
	TLSCertificatePath string
	TLSEndpoint        string
}

func GetTlsCertificateRetriever(settings TlsSettings) (TlsCertificateRetriever, error) {
	if settings.TLSCertificatePath != "" {
		return NewFileTlsCertificateRetriever(settings)
	}
	// if Windows build flag is set, the below will return a windows implementation
	// otherwise it will return a error as caller should of already received a
	// tls certificate parsed from disk.
	return NewTlsCertificateRetriever(settings)
}
