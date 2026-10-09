package blockchain

import (
	"errors"
	"fmt"

	bolt "go.etcd.io/bbolt"
)

// CreateBlockchain creates a new blockchain database with a genesis block
// assigning the initial reward to address.
func CreateBlockchain(address string) (*Blockchain, error) {
	var tip []byte
	// BoltDB takes an exclusive file lock while open, which prevents another
	// process from creating or modifying the same blockchain concurrently.
	db, err := bolt.Open(dbFile, 0o600, nil)
	if err != nil {
		return nil, err
	}
	err = db.Update(func(tx *bolt.Tx) error {
		// Build and mine the genesis transaction inside the same initialization
		// operation so the bucket is created only for a complete chain.
		cbtx, err := NewCoinbaseTX(address, genesisCoinbaseData)
		if err != nil {
			return err
		}
		genesis, err := NewGenesisBlock(cbtx)
		if err != nil {
			return err
		}

		b, err := tx.CreateBucket([]byte(blocksBucket))
		if err != nil {
			if errors.Is(err, bolt.ErrBucketExists) {
				return fmt.Errorf("blockchain already exists: %w", err)
			}
			return err
		}
		data, err := genesis.Serialize()
		if err != nil {
			return err
		}
		if err := b.Put(genesis.Hash, data); err != nil {
			return err
		}
		if err := b.Put([]byte(lastHashKey), genesis.Hash); err != nil {
			return err
		}
		tip = genesis.Hash

		return nil
	})
	if err != nil {
		if closeErr := db.Close(); closeErr != nil {
			return nil, fmt.Errorf("initialize blockchain: %w (also failed to close database: %v)", err, closeErr)
		}
		return nil, err
	}
	return &Blockchain{tip: tip, db: db}, nil
}

// OpenBlockchain opens an existing blockchain database and loads its latest
// block hash.
func OpenBlockchain() (*Blockchain, error) {
	var tip []byte
	db, err := bolt.Open(dbFile, 0o600, nil)
	if err != nil {
		return nil, err
	}
	err = db.View(func(tx *bolt.Tx) error {
		// Read the persisted tip while the BoltDB read transaction is open, then
		// copy it below because BoltDB owns the returned byte slice.
		b := tx.Bucket([]byte(blocksBucket))
		if b == nil {
			return fmt.Errorf("blocks bucket not found")
		}

		// Copy the tip because BoltDB's transaction-owned bytes are invalid
		// after the read transaction closes.
		stored := b.Get([]byte(lastHashKey))
		if stored == nil {
			return fmt.Errorf("blockchain tip not found")
		}
		tip = make([]byte, len(stored))
		copy(tip, stored)

		return nil
	})
	if err != nil {
		if closeErr := db.Close(); closeErr != nil {
			return nil, fmt.Errorf("read blockchain tip: %w (also failed to close database: %v)", err, closeErr)
		}
		return nil, err
	}

	return &Blockchain{tip: tip, db: db}, nil
}
