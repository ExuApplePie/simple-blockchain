package blockchain

import (
	"errors"

	bolt "go.etcd.io/bbolt"
)

// BlockchainIterator walks blocks backward from the current chain tip.
type BlockchainIterator struct {
	currentHash []byte
	db          *bolt.DB
}

// Next returns the current block and advances to its predecessor.
func (i *BlockchainIterator) Next() (*Block, error) {
	var block *Block

	// Read and deserialize one block per BoltDB transaction, then move the
	// iterator to that block's predecessor after the read succeeds.
	err := i.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))
		if b == nil {
			return errors.New("blocks bucket does not exist")
		}
		encodedBlock := b.Get(i.currentHash)
		if encodedBlock == nil {
			return errors.New("block not found")
		}
		var err error
		block, err = DeserializeBlock(encodedBlock)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	i.currentHash = block.PrevBlockHash
	return block, nil
}

// Iterator returns an iterator positioned at the blockchain tip.
func (bc *Blockchain) Iterator() *BlockchainIterator {
	return &BlockchainIterator{bc.tip, bc.db}
}
