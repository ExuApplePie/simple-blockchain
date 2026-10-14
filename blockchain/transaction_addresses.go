package blockchain

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"github.com/btcsuite/btcutil/base58"
)

// NewCoinbaseTX creates the reward transaction for a newly created block.
func NewCoinbaseTX(to, data string) (*Transaction, error) {
	// A coinbase input has no previous transaction; its data field carries the
	// miner/reward message instead.
	if len(data) == 0 {
		// Include a nonce-like timestamp so repeated mining rewards do not reuse
		// the same transaction ID and overwrite one another in the UTXO set.
		data = fmt.Sprintf("Reward to '%s' at %d", to, time.Now().UnixNano())
	}

	txin := TXInput{[]byte{}, -1, nil, []byte(data)}
	txout, err := NewTXOutput(subsidy, to)
	if err != nil {
		return nil, fmt.Errorf("create coinbase output: %w", err)
	}
	tx := Transaction{nil, []TXInput{txin}, []TXOutput{*txout}}
	hash, err := tx.Hash()
	if err != nil {
		return nil, fmt.Errorf("hash coinbase transaction: %w", err)
	}
	// Assign the ID only after hashing the transaction without an ID.
	tx.ID = hash

	return &tx, nil
}

// UsesKey reports whether the input's public key hashes to the supplied key
// hash.
func (in *TXInput) UsesKey(pubKeyHash []byte) bool {
	lockingHash := HashPubKey(in.PubKey)

	return bytes.Equal(lockingHash, pubKeyHash)
}

// IsLockedWithKey reports whether the output is locked to the supplied key
// hash.
func (out *TXOutput) IsLockedWithKey(pubKeyHash []byte) bool {
	return bytes.Equal(out.PubKeyHash, pubKeyHash)
}

// NewTXOutput creates an output locked to the public-key hash encoded by an
// address.
func NewTXOutput(value int, address string) (*TXOutput, error) {
	// Store only the decoded key hash in the output; the human-readable address
	// is not needed when validating ownership later.
	txo := &TXOutput{Value: value, PubKeyHash: nil}
	pubKeyHash, err := PubKeyHashFromAddress(address)
	if err != nil {
		return nil, fmt.Errorf("decode output address %q: %w", address, err)
	}
	txo.PubKeyHash = pubKeyHash
	return txo, nil
}

// PubKeyHashFromAddress validates a Base58Check address and returns its
// public-key hash payload without the version byte or checksum.
func PubKeyHashFromAddress(address string) ([]byte, error) {
	// Base58 decoding gives us version, payload, and checksum bytes to validate
	// independently before returning the payload to transaction code.
	decoded := base58.Decode(address)

	const publicKeyHashLen = sha256.Size
	expectedLen := addressVersionLen + publicKeyHashLen + addressChecksumLen
	if len(decoded) != expectedLen {
		return nil, fmt.Errorf("invalid address length: got %d bytes, want %d", len(decoded), expectedLen)
	}

	if decoded[0] != version {
		return nil, fmt.Errorf("invalid address version: got %d, want %d", decoded[0], version)
	}

	// Separate the checksum from the versioned payload, then recompute and
	// compare it before trusting the address contents.
	payloadEnd := len(decoded) - addressChecksumLen
	payload := decoded[:payloadEnd]

	expectedChecksum := checksum(payload)
	actualChecksum := decoded[payloadEnd:]

	if !bytes.Equal(expectedChecksum, actualChecksum) {
		return nil, errors.New("invalid address checksum")
	}

	return payload[addressVersionLen:], nil
}
