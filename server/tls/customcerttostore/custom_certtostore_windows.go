// +build windows

// The below file was borrowed from this repo
// and modified to our needs per the below license permits
// https://github.com/google/certtostore

// Copyright 2017 Google Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package customcerttostore

import (
	"bytes"
	"crypto"
	"crypto/rsa"
	"crypto/x509"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/big"
	"reflect"
	"strings"
	"syscall"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	// wincrypt.h constants
	acquireCached           = 0x1                                             // CRYPT_ACQUIRE_CACHE_FLAG
	acquireSilent           = 0x40                                            // CRYPT_ACQUIRE_SILENT_FLAG
	acquireOnlyNCryptKey    = 0x40000                                         // CRYPT_ACQUIRE_ONLY_NCRYPT_KEY_FLAG
	encodingX509ASN         = 1                                               // X509_ASN_ENCODING
	encodingPKCS7           = 65536                                           // PKCS_7_ASN_ENCODING
	certStoreProvSystem     = 10                                              // CERT_STORE_PROV_SYSTEM
	certStoreLocalMachine   = uint32(certStoreLocalMachineID << compareShift) // CERT_SYSTEM_STORE_LOCAL_MACHINE
	certStoreLocalMachineID = 2                                               // CERT_SYSTEM_STORE_LOCAL_MACHINE_ID
	compareNameStrW         = 8                                               // CERT_COMPARE_NAME_STR_A
	compareShift            = 16                                              // CERT_COMPARE_SHIFT
	ncryptKeySpec           = 0xFFFFFFFF                                      // CERT_NCRYPT_KEY_SPEC
	infoSubjectFlag         = 7                                               // CERT_INFO_SUBJECT_FLAG
	findSubjectStr          = compareNameStrW<<compareShift | infoSubjectFlag // CERT_FIND_SUBJECT_NAME

	// Legacy CryptoAPI flags
	bCryptPadPKCS1 uintptr = 0x2

	// Magic number for public key blobs.
	rsa1Magic = 0x31415352 // "RSA1" BCRYPT_RSAPUBLIC_MAGIC

	// key creation flag.
	nCryptMachineKey = 0x20 // NCRYPT_MACHINE_KEY_FLAG

	// winerror.h constants
	cryptENotFound = 0x80092004 // CRYPT_E_NOT_FOUND

	// ProviderMSPlatform represents the Microsoft Platform Crypto Provider
	ProviderMSPlatform = "Microsoft Platform Crypto Provider"
	// ProviderMSSoftware represents the Microsoft Software Key Storage Provider
	ProviderMSSoftware = "Microsoft Software Key Storage Provider"
	// ProviderMSLegacy represents the CryptoAPI compatible Enhanced Cryptographic Provider
	ProviderMSLegacy = "Microsoft Enhanced Cryptographic Provider v1.0"
)

var (
	// Key blob type constants.
	bCryptRSAPublicBlob = wide("RSAPUBLICBLOB")
	bCryptECCPublicBlob = wide("ECCPUBLICBLOB")

	// Key storage properties
	nCryptAlgorithmGroupProperty = wide("Algorithm Group") // NCRYPT_ALGORITHM_GROUP_PROPERTY
	nCryptUniqueNameProperty     = wide("Unique Name")     // NCRYPT_UNIQUE_NAME_PROPERTY

	// algIDs maps crypto.Hash values to bcrypt.h constants.
	algIDs = map[crypto.Hash]*uint16{
		crypto.SHA1:   wide("SHA1"),   // BCRYPT_SHA1_ALGORITHM
		crypto.SHA256: wide("SHA256"), // BCRYPT_SHA256_ALGORITHM
		crypto.SHA384: wide("SHA384"), // BCRYPT_SHA384_ALGORITHM
		crypto.SHA512: wide("SHA512"), // BCRYPT_SHA512_ALGORITHM
	}

	// MY, CA and ROOT are well-known system stores that holds certificates.
	// The store that is opened (system or user) depends on the system call used.
	// see https://msdn.microsoft.com/en-us/library/windows/desktop/aa376560(v=vs.85).aspx)
	my   = wide("MY")
	ca   = wide("CA")
	root = wide("ROOT")

	crypt32 = windows.MustLoadDLL("crypt32.dll")
	nCrypt  = windows.MustLoadDLL("ncrypt.dll")

	certDeleteCertificateFromStore    = crypt32.MustFindProc("CertDeleteCertificateFromStore")
	certFindCertificateInStore        = crypt32.MustFindProc("CertFindCertificateInStore")
	certFreeCertificateChain          = crypt32.MustFindProc("CertFreeCertificateChain")
	certGetCertificateChain           = crypt32.MustFindProc("CertGetCertificateChain")
	certGetIntendedKeyUsage           = crypt32.MustFindProc("CertGetIntendedKeyUsage")
	cryptAcquireCertificatePrivateKey = crypt32.MustFindProc("CryptAcquireCertificatePrivateKey")
	cryptFindCertificateKeyProvInfo   = crypt32.MustFindProc("CryptFindCertificateKeyProvInfo")
	nCryptCreatePersistedKey          = nCrypt.MustFindProc("NCryptCreatePersistedKey")
	nCryptDecrypt                     = nCrypt.MustFindProc("NCryptDecrypt")
	nCryptExportKey                   = nCrypt.MustFindProc("NCryptExportKey")
	nCryptFinalizeKey                 = nCrypt.MustFindProc("NCryptFinalizeKey")
	nCryptOpenKey                     = nCrypt.MustFindProc("NCryptOpenKey")
	nCryptOpenStorageProvider         = nCrypt.MustFindProc("NCryptOpenStorageProvider")
	nCryptGetProperty                 = nCrypt.MustFindProc("NCryptGetProperty")
	nCryptSetProperty                 = nCrypt.MustFindProc("NCryptSetProperty")
	nCryptSignHash                    = nCrypt.MustFindProc("NCryptSignHash")
)

// Credential provides access to a certificate and is a crypto.Signer and crypto.Decrypter.
type Credential interface {
	// Public returns the public key corresponding to the leaf certificate.
	Public() crypto.PublicKey
	// Sign signs digest with the private key.
	Sign(rand io.Reader, digest []byte, opts crypto.SignerOpts) (signature []byte, err error)
	// Decrypt decrypts msg. Returns an error if not implemented.
	Decrypt(rand io.Reader, msg []byte, opts crypto.DecrypterOpts) (plaintext []byte, err error)
}

// paddingInfo is the BCRYPT_PKCS1_PADDING_INFO struct in bcrypt.h.
type paddingInfo struct {
	pszAlgID *uint16
}

// wide returns a pointer to a a uint16 representing the equivalent
// to a Windows LPCWSTR.
func wide(s string) *uint16 {
	w := utf16.Encode([]rune(s))
	w = append(w, 0)
	return &w[0]
}

func openProvider(provider string) (uintptr, error) {
	var err error
	var hProv uintptr
	pname := wide(provider)
	// Open the provider, the last parameter is not used
	r, _, err := nCryptOpenStorageProvider.Call(uintptr(unsafe.Pointer(&hProv)), uintptr(unsafe.Pointer(pname)), 0)
	if r == 0 {
		return hProv, nil
	}
	return hProv, fmt.Errorf("NCryptOpenStorageProvider returned %X: %v", r, err)
}

// findCert wraps the CertFindCertificateInStore call. Note that any cert context passed
// into prev will be freed. If no certificate was found, nil will be returned.
func findCert(store windows.Handle, enc, findFlags, findType uint32, para *uint16, prev *windows.CertContext) (*windows.CertContext, error) {
	h, _, err := certFindCertificateInStore.Call(
		uintptr(store),
		uintptr(enc),
		uintptr(findFlags),
		uintptr(findType),
		uintptr(unsafe.Pointer(para)),
		uintptr(unsafe.Pointer(prev)),
	)
	if h == 0 {
		// Actual error, or simply not found?
		if errno, ok := err.(syscall.Errno); ok && errno == cryptENotFound {
			return nil, nil
		}
		return nil, err
	}
	return (*windows.CertContext)(unsafe.Pointer(h)), nil
}

// intendedKeyUsage wraps CertGetIntendedKeyUsage. If there are key usage bytes they will be returned,
// otherwise 0 will be returned. The final parameter (2) represents the size in bytes of &usage.
func intendedKeyUsage(enc uint32, cert *windows.CertContext) (usage uint16) {
	certGetIntendedKeyUsage.Call(uintptr(enc), uintptr(unsafe.Pointer(cert.CertInfo)), uintptr(unsafe.Pointer(&usage)), 2)
	return
}

// WinCertStore is a CertStorage implementation for the Windows Certificate Store.
type WinCertStore struct {
	Prov                uintptr
	ProvName            string
	issuers             []string
	intermediateIssuers []string
	container           string
	keyStorageFlags     uintptr
	certChains          [][]*x509.Certificate
}

// OpenWinCertStore creates a WinCertStore.
// when using openStoreWithHandle with handle, it is the responsbility of the caller to
// call Cstore.Close from the returned object
func OpenWinCertStore(provider, container string, issuers, intermediateIssuers []string, legacyKey, openStoreWithHandle bool) (*WinCertStore, error) {
	// Open a handle to the crypto provider we will use for private key operations
	cngProv, err := openProvider(provider)
	if err != nil {
		return nil, fmt.Errorf("unable to open crypto provider or provider not available: %v", err)
	}

	wcs := &WinCertStore{
		Prov:                cngProv,
		ProvName:            provider,
		issuers:             issuers,
		intermediateIssuers: intermediateIssuers,
		container:           container,
	}

	return wcs, nil
}

// certContextToX509 creates an x509.Certificate from a Windows cert context.
func certContextToX509(ctx *windows.CertContext) (*x509.Certificate, error) {
	var der []byte
	slice := (*reflect.SliceHeader)(unsafe.Pointer(&der))
	slice.Data = uintptr(unsafe.Pointer(ctx.EncodedCert))
	slice.Len = int(ctx.Length)
	slice.Cap = int(ctx.Length)
	return x509.ParseCertificate(der)
}

// cert is a function to lookup certificates based on a subject name.
func (w *WinCertStore) CertBySubjectName(subjectName string) (*x509.Certificate, *windows.CertContext, error) {
	var certContext *windows.CertContext
	var cert *x509.Certificate

	// Open a handle to the system cert store
	certStore, err := windows.CertOpenStore(
		certStoreProvSystem,
		0,
		0,
		certStoreLocalMachine,
		uintptr(unsafe.Pointer(my)))
	defer windows.CertCloseStore(certStore, 0)
	if err != nil {
		return nil, nil , fmt.Errorf("CertOpenStore returned: %v", err)
	}

	searchString, err := windows.UTF16PtrFromString(subjectName)
	if err != nil {
		return nil, nil, err
	}

	certContext, err = findCert(certStore, encodingX509ASN|encodingPKCS7, 0, findSubjectStr, searchString, certContext)

	if err != nil {
		return nil, nil, err
	}

	if certContext == nil{
		return nil, nil, fmt.Errorf("Certificate not found")
	}

	cert, err = certContextToX509(certContext)
	if err != nil {
		return nil, nil, err
	}
	return cert, certContext, nil
}

// Key implements crypto.Signer and crypto.Decrypter for key based operations.
type Key struct {
	handle          uintptr
	pub             crypto.PublicKey
	Container       string
	LegacyContainer string
	AlgorithmGroup  string
}

// Public exports a public key to implement crypto.Signer
func (k Key) Public() crypto.PublicKey {
	return k.pub
}

// Sign returns the signature of a hash to implement crypto.Signer
func (k Key) Sign(_ io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
	switch k.AlgorithmGroup {
	case "ECDSA":
		return nil, fmt.Errorf("ECDSA not supported in this implmentation")
	case "RSA":
		hf := opts.HashFunc()
		algID, ok := algIDs[hf]
		if !ok {
			return nil, fmt.Errorf("unsupported RSA hash algorithm %v", hf)
		}
		return signRSA(k.handle, digest, algID)
	default:
		return nil, fmt.Errorf("unsupported algorithm group %v", k.AlgorithmGroup)
	}
}

func signRSA(kh uintptr, digest []byte, algID *uint16) ([]byte, error) {
	padInfo := paddingInfo{pszAlgID: algID}
	var size uint32
	// Obtain the size of the signature
	r, _, err := nCryptSignHash.Call(
		kh,
		uintptr(unsafe.Pointer(&padInfo)),
		uintptr(unsafe.Pointer(&digest[0])),
		uintptr(len(digest)),
		0,
		0,
		uintptr(unsafe.Pointer(&size)),
		bCryptPadPKCS1)
	if r != 0 {
		return nil, fmt.Errorf("NCryptSignHash returned %X during size check: %v", r, err)
	}

	// Obtain the signature data
	sig := make([]byte, size)
	r, _, err = nCryptSignHash.Call(
		kh,
		uintptr(unsafe.Pointer(&padInfo)),
		uintptr(unsafe.Pointer(&digest[0])),
		uintptr(len(digest)),
		uintptr(unsafe.Pointer(&sig[0])),
		uintptr(size),
		uintptr(unsafe.Pointer(&size)),
		bCryptPadPKCS1)
	if r != 0 {
		return nil, fmt.Errorf("NCryptSignHash returned %X during signing: %v", r, err)
	}

	return sig[:size], nil
}

// DecrypterOpts implements crypto.DecrypterOpts and contains the
// flags required for the NCryptDecrypt system call.
type DecrypterOpts struct {
	// Hashfunc represents the hashing function that was used during
	// encryption and is mapped to the Microsoft equivalent LPCWSTR.
	Hashfunc crypto.Hash
	// Flags represents the dwFlags parameter for NCryptDecrypt
	Flags uint32
}

// oaepPaddingInfo is the BCRYPT_OAEP_PADDING_INFO struct in bcrypt.h.
// https://msdn.microsoft.com/en-us/library/windows/desktop/aa375526(v=vs.85).aspx
type oaepPaddingInfo struct {
	pszAlgID *uint16 // pszAlgId
	pbLabel  *uint16 // pbLabel
	cbLabel  uint32  // cbLabel
}

// Decrypt returns the decrypted contents of the encrypted blob, and implements
// crypto.Decrypter for Key.
func (k Key) Decrypt(rand io.Reader, blob []byte, opts crypto.DecrypterOpts) ([]byte, error) {
	decrypterOpts, ok := opts.(DecrypterOpts)
	if !ok {
		return nil, errors.New("opts was not certtostore.DecrypterOpts")
	}

	algID, ok := algIDs[decrypterOpts.Hashfunc]
	if !ok {
		return nil, fmt.Errorf("unsupported hash algorithm %v", decrypterOpts.Hashfunc)
	}

	padding := oaepPaddingInfo{
		pszAlgID: algID,
		pbLabel:  wide(""),
		cbLabel:  0,
	}

	return decrypt(k.handle, blob, padding, decrypterOpts.Flags)
}

// decrypt wraps the NCryptDecrypt function and returns the decrypted bytes
// that were previously encrypted by NCryptEncrypt or another compatible
// function such as rsa.EncryptOAEP.
// https://msdn.microsoft.com/en-us/library/windows/desktop/aa376249(v=vs.85).aspx
func decrypt(kh uintptr, blob []byte, padding oaepPaddingInfo, flags uint32) ([]byte, error) {
	var size uint32
	// Obtain the size of the decrypted data
	r, _, err := nCryptDecrypt.Call(
		kh,
		uintptr(unsafe.Pointer(&blob[0])),
		uintptr(len(blob)),
		uintptr(unsafe.Pointer(&padding)),
		0, // Must be null on first run.
		0, // Ignored on first run.
		uintptr(unsafe.Pointer(&size)),
		uintptr(flags))
	if r != 0 {
		return nil, fmt.Errorf("NCryptDecrypt returned %X during size check: %v", r, err)
	}

	// Decrypt the message
	plainText := make([]byte, size)
	r, _, err = nCryptDecrypt.Call(
		kh,
		uintptr(unsafe.Pointer(&blob[0])),
		uintptr(len(blob)),
		uintptr(unsafe.Pointer(&padding)),
		uintptr(unsafe.Pointer(&plainText[0])),
		uintptr(size),
		uintptr(unsafe.Pointer(&size)),
		uintptr(flags))
	if r != 0 {
		return nil, fmt.Errorf("NCryptDecrypt returned %X during decryption: %v", r, err)
	}

	return plainText[:size], nil
}

// Key opens a handle to an existing private key and returns key.
// Key implements both crypto.Signer and crypto.Decrypter
func (w *WinCertStore) Key() (Credential, error) {
	var kh uintptr
	r, _, err := nCryptOpenKey.Call(
		uintptr(w.Prov),
		uintptr(unsafe.Pointer(&kh)),
		uintptr(unsafe.Pointer(wide(w.container))),
		0,
		nCryptMachineKey)
	if r != 0 {
		return nil, fmt.Errorf("NCryptOpenKey for container %q returned %X: %v", w.container, r, err)
	}

	return keyMetadata(kh, w)
}

// CertKey wraps CryptAcquireCertificatePrivateKey. It obtains the CNG private
// key of a known certificate and returns a pointer to a Key which implements
// both crypto.Signer and crypto.Decrypter. When a nil cert context is passed
// a nil key is intentionally returned, to model the expected behavior of a
// non-existent cert having no private key.
// https://docs.microsoft.com/en-us/windows/win32/api/wincrypt/nf-wincrypt-cryptacquirecertificateprivatekey
func (w *WinCertStore) CertKey(cert *windows.CertContext) (*Key, error) {
	// Return early if a nil cert was passed.
	if cert == nil {
		return nil, nil
	}
	var (
		kh       uintptr
		spec     uint32
		mustFree int
	)
	r, _, err := cryptAcquireCertificatePrivateKey.Call(
		uintptr(unsafe.Pointer(cert)),
		acquireCached|acquireSilent|acquireOnlyNCryptKey,
		0, // Reserved, must be null.
		uintptr(unsafe.Pointer(&kh)),
		uintptr(unsafe.Pointer(&spec)),
		uintptr(unsafe.Pointer(&mustFree)),
	)
	// If the function succeeds, the return value is nonzero (TRUE).
	if r == 0 {
		return nil, fmt.Errorf("cryptAcquireCertificatePrivateKey returned %X: %v", r, err)
	}
	if mustFree != 0 {
		return nil, fmt.Errorf("wrong mustFree [%d != 0]", mustFree)
	}
	if spec != ncryptKeySpec {
		return nil, fmt.Errorf("wrong keySpec [%d != %d]", spec, ncryptKeySpec)
	}

	return keyMetadata(kh, w)
}

func keyMetadata(kh uintptr, store *WinCertStore) (*Key, error) {
	// uc is used to populate the unique container name attribute of the private key
	uc, err := getProperty(kh, nCryptUniqueNameProperty)
	if err != nil {
		return nil, fmt.Errorf("unable to determine key unique name: %v", err)
	}

	alg, err := getProperty(kh, nCryptAlgorithmGroupProperty)
	if err != nil {
		return nil, fmt.Errorf("unable to determine key algorithm: %v", err)
	}
	var pub crypto.PublicKey
	switch alg {
	case "RSA":
		buf, err := export(kh, bCryptRSAPublicBlob)
		if err != nil {
			return nil, fmt.Errorf("failed to export %v public key: %v", alg, err)
		}
		pub, err = unmarshalRSA(buf)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal %v public key: %v", alg, err)
		}

	default:
		return nil, fmt.Errorf("Only RSA based keys are supported")
	}

	return &Key{handle: kh, pub: pub, Container: uc, LegacyContainer: "", AlgorithmGroup: alg}, nil
}

func getProperty(kh uintptr, property *uint16) (string, error) {
	var strSize uint32
	r, _, err := nCryptGetProperty.Call(
		kh,
		uintptr(unsafe.Pointer(property)),
		0,
		0,
		uintptr(unsafe.Pointer(&strSize)),
		0,
		0)
	if r != 0 {
		return "", fmt.Errorf("NCryptGetProperty(%v) returned %X during size check: %v", property, r, err)
	}

	buf := make([]byte, strSize)
	r, _, err = nCryptGetProperty.Call(
		kh,
		uintptr(unsafe.Pointer(property)),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(strSize),
		uintptr(unsafe.Pointer(&strSize)),
		0,
		0)
	if r != 0 {
		return "", fmt.Errorf("NCryptGetProperty %v returned %X during export: %v", property, r, err)
	}

	uc := strings.Replace(string(buf), string(0x00), "", -1)
	return uc, nil
}

func export(kh uintptr, blobType *uint16) ([]byte, error) {
	var size uint32
	// When obtaining the size of a public key, most parameters are not required
	r, _, err := nCryptExportKey.Call(
		kh,
		0,
		uintptr(unsafe.Pointer(blobType)),
		0,
		0,
		0,
		uintptr(unsafe.Pointer(&size)),
		0)
	if r != 0 {
		return nil, fmt.Errorf("NCryptExportKey returned %X during size check: %v", r, err)
	}

	// Place the exported key in buf now that we know the size required
	buf := make([]byte, size)
	r, _, err = nCryptExportKey.Call(
		kh,
		0,
		uintptr(unsafe.Pointer(blobType)),
		0,
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(size),
		uintptr(unsafe.Pointer(&size)),
		0)
	if r != 0 {
		return nil, fmt.Errorf("NCryptExportKey returned %X during export: %v", r, err)
	}
	return buf, nil
}

func unmarshalRSA(buf []byte) (*rsa.PublicKey, error) {
	// BCRYPT_RSA_BLOB from bcrypt.h
	header := struct {
		Magic         uint32
		BitLength     uint32
		PublicExpSize uint32
		ModulusSize   uint32
		UnusedPrime1  uint32
		UnusedPrime2  uint32
	}{}

	r := bytes.NewReader(buf)
	if err := binary.Read(r, binary.LittleEndian, &header); err != nil {
		return nil, err
	}

	if header.Magic != rsa1Magic {
		return nil, fmt.Errorf("invalid header magic %x", header.Magic)
	}

	if header.PublicExpSize > 8 {
		return nil, fmt.Errorf("unsupported public exponent size (%d bits)", header.PublicExpSize*8)
	}

	exp := make([]byte, 8)
	if n, err := r.Read(exp[8-header.PublicExpSize:]); n != int(header.PublicExpSize) || err != nil {
		return nil, fmt.Errorf("failed to read public exponent (%d, %v)", n, err)
	}

	mod := make([]byte, header.ModulusSize)
	if n, err := r.Read(mod); n != int(header.ModulusSize) || err != nil {
		return nil, fmt.Errorf("failed to read modulus (%d, %v)", n, err)
	}

	pub := &rsa.PublicKey{
		N: new(big.Int).SetBytes(mod),
		E: int(binary.BigEndian.Uint64(exp)),
	}
	return pub, nil
}

// Verify interface conformance.
var _ Credential = &Key{}
