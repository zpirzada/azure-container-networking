// Copyright 2020 Microsoft. All rights reserved.

package tls

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"software.sslmate.com/src/go-pkcs12"
	"testing"
	"time"
)

const (
	validFor   = time.Duration(365 * 24 * time.Hour)
	rsaBits    = 2048
	commonName = "dnc.azure.com"
)

func TestPfxConsumption(t *testing.T) {
	pfxContent := createPfxCertificate(t)
	currentDirectory, _ := os.Getwd()
	pfxLocation := fmt.Sprintf("%s/%s.pfx", currentDirectory, commonName)

	ioutil.WriteFile(pfxLocation, pfxContent, 0)
	defer os.Remove(pfxLocation)

	config := TlsSettings{
		TLSCertificatePath:    pfxLocation,
		TLSSubjectName: commonName,
	}
	fileCertRetriever, err := NewFileTlsCertificateRetriever(config)
	if err != nil {
		t.Fatalf("Failed to open file certificate retriever %+v", err)
	}
	certificate, err := fileCertRetriever.GetCertificate()
	if err != nil {
		t.Fatalf("Failed to get certificate %+v", err)
	}
	if certificate.Subject.CommonName != commonName {
		t.Fatalf("Recieved a unexpected subject name %+v", err)
	}
	_, err = fileCertRetriever.GetPrivateKey()
	if err != nil {
		t.Fatalf("Failed to get private key %+v", err)
	}
}

func createPfxCertificate(t *testing.T) []byte {
	priv, err := rsa.GenerateKey(rand.Reader, rsaBits)
	if err != nil {
		t.Fatalf("Failed to generate private key: %v", err)
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Acme Co"},
			CommonName:   commonName,
		},
	}
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("Failed to create certificate: %v", err)
	}
	certificate, err := x509.ParseCertificate(derBytes)
	if err != nil {
		t.Fatalf("Could not parse certificate")
	}
	pfxcontent, err := pkcs12.Encode(rand.Reader, priv, certificate, []*x509.Certificate{}, "")
	if err != nil {
		t.Fatalf("Could not encode certificate to pkcs12")
	}
	return pfxcontent
}
