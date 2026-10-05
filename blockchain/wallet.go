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

func NewWallet() (*Wallet, error) {
	private, public, err := newKeyPair()
	if err != nil {
		return nil, err
	}
	wallet := Wallet{PrivateKey: private, PublicKey: public}

	return &wallet, nil
}

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

// return string as it's meant to be human readable
func (w Wallet) GetAddress() string {
	pubKeyHash := HashPubKey(w.PublicKey)

	versionedPayload := append([]byte{version}, pubKeyHash...)
	checksum := checksum(versionedPayload)

	fullPayload := append(versionedPayload, checksum...)
	address := base58.Encode(fullPayload)

	return address
}
