package blockchain

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
)

// Hash returns the SHA-256 hash of the transaction without including its own
// ID, preventing the ID from hashing itself recursively.
func (tx *Transaction) Hash() ([]byte, error) {
	var hash [32]byte

	// The ID is derived from the transaction contents, so exclude the current
	// ID before serializing; otherwise changing the ID would change its hash.
	txCopy := *tx
	txCopy.ID = []byte{}

	data, err := txCopy.Serialize()
	if err != nil {
		return nil, err
	}
	hash = sha256.Sum256(data)

	return hash[:], nil
}

// TrimmedCopy returns a transaction copy prepared for signing. Input
// signatures and public keys are removed, while the output values and locking
// hashes remain part of the data being signed.
func (tx *Transaction) TrimmedCopy() Transaction {
	var inputs []TXInput
	var outputs []TXOutput

	// Keep references and output values, but remove signature material. This
	// creates the common transaction shape used to calculate signing data.
	for _, vin := range tx.Vin {
		inputs = append(inputs, TXInput{vin.Txid, vin.Vout, nil, nil})
	}
	// Copy each output into a new slice so the trimmed transaction has
	// independent TXOutput structs. The PubKeyHash byte slices remain shared.
	for _, vout := range tx.Vout {
		outputs = append(outputs, TXOutput{vout.Value, vout.PubKeyHash})
	}

	txCopy := Transaction{tx.ID, inputs, outputs}

	return txCopy
}

// Sign signs every input using the corresponding previous output's public-key
// hash as part of the signing data.
func (tx *Transaction) Sign(privKey ecdsa.PrivateKey, prevTXs map[string]Transaction) error {
	if tx.IsCoinbase() {
		return nil
	}

	// Work on a copy so the real transaction remains unsigned until each
	// signature has been successfully generated.
	txCopy := tx.TrimmedCopy()

	for inID, vin := range txCopy.Vin {
		// Each input signs a different message because its referenced output is
		// inserted into the copy before hashing.
		prevOutput, err := previousOutput(vin, prevTXs)
		if err != nil {
			return err
		}
		// Include the previous locking hash while constructing the message, but
		// never include the private key or the final public key in that message.
		txCopy.Vin[inID].Signature = nil
		txCopy.Vin[inID].PubKey = prevOutput.PubKeyHash
		hash, err := txCopy.Hash()
		if err != nil {
			return err
		}

		txCopy.ID = hash
		txCopy.Vin[inID].PubKey = nil

		signature, err := ecdsa.SignASN1(rand.Reader, &privKey, txCopy.ID)
		if err != nil {
			return err
		}

		// Store the signature only on the original transaction after signing the
		// copy has succeeded.
		tx.Vin[inID].Signature = signature
	}
	return nil
}

// Verify checks that every input references a valid output, uses the matching
// public key, and contains a valid ECDSA signature.
// TODO: Handle coinbase transactions separately before applying normal input
// ownership and signature checks.
func (tx *Transaction) Verify(prevTXs map[string]Transaction) bool {
	// Recreate the same signing data used by Sign, then compare each supplied
	// signature against that data and the supplied public key.
	txCopy := tx.TrimmedCopy()

	for inID, vin := range tx.Vin {
		// Resolve the referenced output first so malformed input indexes fail
		// safely instead of indexing outside the previous transaction's outputs.
		prevOutput, err := previousOutput(vin, prevTXs)
		if err != nil {
			return false
		}

		// The public key must hash to the key hash that locked the old output;
		// otherwise a valid signature from another wallet would still be invalid.
		if !slices.Equal(HashPubKey(vin.PubKey), prevOutput.PubKeyHash) {
			return false
		}
		txCopy.Vin[inID].Signature = nil
		txCopy.Vin[inID].PubKey = prevOutput.PubKeyHash
		data, err := txCopy.Hash()
		if err != nil {
			return false
		}
		txCopy.ID = data
		txCopy.Vin[inID].PubKey = nil

		// Parse the serialized public key before asking ECDSA to verify it.
		rawPubKey, err := ecdsa.ParseUncompressedPublicKey(elliptic.P256(), vin.PubKey)
		if err != nil {
			return false
		}

		if !ecdsa.VerifyASN1(rawPubKey, txCopy.ID, vin.Signature) {
			return false
		}
	}

	return true
}

func previousOutput(vin TXInput, prevTXs map[string]Transaction) (TXOutput, error) {
	// Look up by transaction ID, then validate the output index before accessing
	// the slice. Both checks are needed because inputs are untrusted data.
	prevTx, ok := prevTXs[hex.EncodeToString(vin.Txid)]
	if !ok {
		return TXOutput{}, fmt.Errorf("previous transaction %x not found", vin.Txid)
	}
	if vin.Vout < 0 || vin.Vout >= len(prevTx.Vout) {
		return TXOutput{}, fmt.Errorf("previous output index %d out of range", vin.Vout)
	}

	return prevTx.Vout[vin.Vout], nil
}
