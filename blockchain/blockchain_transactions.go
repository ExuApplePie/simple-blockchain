package blockchain

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"fmt"
)

// FindTransaction searches the chain for a transaction with the supplied ID.
func (bc *Blockchain) FindTransaction(id []byte) (Transaction, error) {
	bci := bc.Iterator()

	// The iterator starts at the tip, so recent transactions are checked first.
	for {
		block, err := bci.Next()
		if err != nil {
			return Transaction{}, fmt.Errorf("scan blockchain for transaction %x: %w", id, err)
		}

		for _, tx := range block.Transactions {
			if bytes.Equal(tx.ID, id) {
				return *tx, nil
			}
		}

		if len(block.PrevBlockHash) == 0 {
			break
		}
	}

	return Transaction{}, errors.New("Transaction not found")
}

// SignTransaction signs tx using privKey and the previous transactions it
// references from the chain.
func (bc *Blockchain) SignTransaction(tx *Transaction, privKey ecdsa.PrivateKey) error {
	// Transaction.Sign needs the full previous transactions, not just their
	// IDs, so resolve every input through the chain before signing.
	prevTXs, err := bc.previousTransactions(tx)
	if err != nil {
		return fmt.Errorf("load previous transactions for signing: %w", err)
	}

	return tx.Sign(privKey, prevTXs)
}

// VerifyTransaction verifies tx against the previous transactions it
// references from the chain.
func (bc *Blockchain) VerifyTransaction(tx *Transaction) (bool, error) {
	if tx.IsCoinbase() {
		return true, nil
	}

	// Verification reconstructs the same previous-transaction context used by
	// signing, then delegates cryptographic checks to Transaction.Verify.
	prevTXs, err := bc.previousTransactions(tx)
	if err != nil {
		return false, fmt.Errorf("load previous transactions for verification: %w", err)
	}

	return tx.Verify(prevTXs), nil
}

func (bc *Blockchain) previousTransactions(tx *Transaction) (map[string]Transaction, error) {
	prevTXs := make(map[string]Transaction)

	// Build a lookup map once so Sign and Verify can safely resolve every input
	// without repeatedly traversing the chain for the same transaction.
	for _, vin := range tx.Vin {
		prevTX, err := bc.FindTransaction(vin.Txid)
		if err != nil {
			return nil, fmt.Errorf("find previous transaction %x: %w", vin.Txid, err)
		}
		prevTXs[hex.EncodeToString(prevTX.ID)] = prevTX
	}

	return prevTXs, nil
}
