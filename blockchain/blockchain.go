package blockchain

import (
	"errors"
	"fmt"
	"sync"

	bolt "go.etcd.io/bbolt"
)

const (
	dbFile              = "blockchain.db"
	blocksBucket        = "blocks"
	lastHashKey         = "l"
	genesisCoinbaseData = "Exusiai loves apple pie"
)

// Blockchain stores the chain tip and the BoltDB handle used to persist blocks.
type Blockchain struct {
	tip []byte
	db  *bolt.DB
	mu  sync.Mutex
}

// Close releases the blockchain's database handle.
func (bc *Blockchain) Close() error {
	return bc.db.Close()
}

// MineBlock validates transactions, mines a block, and persists it as the new
// chain tip.
func (bc *Blockchain) MineBlock(transactions []*Transaction) (*Block, error) {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	// Validate every transaction before mining so invalid data cannot become
	// part of the chain merely because it has a valid proof-of-work hash.
	for _, tx := range transactions {
		validTransaction, err := bc.VerifyTransaction(tx)
		if err != nil {
			return nil, fmt.Errorf("verify transaction: %w", err)
		}
		if validTransaction != true {
			return nil, errors.New("Invalid Transaction")
		}
	}

	newBlock, err := NewBlock(transactions, bc.tip)
	if err != nil {
		return nil, fmt.Errorf("mine new block: %w", err)
	}

	// Store the block and update the tip in one BoltDB write transaction.
	err = bc.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))
		if b == nil {
			return errors.New("blocks bucket does not exist")
		}
		serializedData, err := newBlock.Serialize()
		if err != nil {
			return fmt.Errorf("serialize mined block: %w", err)
		}
		if err := b.Put(newBlock.Hash, serializedData); err != nil {
			return fmt.Errorf("store mined block: %w", err)
		}
		if err := b.Put([]byte(lastHashKey), newBlock.Hash); err != nil {
			return fmt.Errorf("update blockchain tip: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("persist mined block: %w", err)
	}
	bc.tip = newBlock.Hash

	return newBlock, nil
}
