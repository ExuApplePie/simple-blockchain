package blockchain

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"

	"github.com/btcsuite/btcutil/base58"
)

const (
	addressChecksumLen      = 4
	version            byte = 0x01
	addressVersionLen       = 1
)

// Wallet contains an ECDSA key pair and its serialized public key.
type Wallet struct {
	PrivateKey ecdsa.PrivateKey
	PublicKey  []byte
}

type storedWallet struct {
	PrivateKey []byte `json:"privateKey"`
	PublicKey  []byte `json:"publicKey"`
}

func newKeyPair() (ecdsa.PrivateKey, []byte, error) {
	curve := elliptic.P256()
	private, err := ecdsa.GenerateKey(curve, rand.Reader)
	if err != nil {
		return ecdsa.PrivateKey{}, nil, err
	}
	pubKey, err := private.PublicKey.Bytes()
	if err != nil {
		return ecdsa.PrivateKey{}, nil, err
	}

	return *private, pubKey, nil
}

// NewWallet generates a new P-256 ECDSA wallet.
func NewWallet() (*Wallet, error) {
	private, public, err := newKeyPair()
	if err != nil {
		return nil, err
	}
	wallet := Wallet{PrivateKey: private, PublicKey: public}

	return &wallet, nil
}

// HashPubKey hashes a serialized public key for use in an address and output
// lock.
func HashPubKey(pubKey []byte) []byte {
	// Given that RIPEMD160 is deprecated and I have no legacy addresses that need public key shortening I will just use sha256
	sum := sha256.Sum256(pubKey)
	return sum[:]
}

func checksum(payload []byte) []byte {
	firstSHA := sha256.Sum256(payload)
	secondSHA := sha256.Sum256(firstSHA[:])

	return secondSHA[:addressChecksumLen]
}

// GetPubKeyHash returns the hash of the wallet's public key.
func (w Wallet) GetPubKeyHash() []byte {
	return HashPubKey(w.PublicKey)
}

// GetAddress returns the wallet's Base58Check-encoded address.
func (w Wallet) GetAddress() string {
	// Addresses contain a versioned public-key hash and a short checksum so they
	// are readable while still allowing malformed addresses to be rejected.
	pubKeyHash := w.GetPubKeyHash()

	versionedPayload := append([]byte{version}, pubKeyHash...)
	checksum := checksum(versionedPayload)

	fullPayload := append(versionedPayload, checksum...)
	address := base58.Encode(fullPayload)

	return address
}
