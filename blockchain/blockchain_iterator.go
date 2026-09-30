package blockchain

import (
	"errors"

	bolt "go.etcd.io/bbolt"
)

type BlockchainIterator struct {
	currentHash []byte
	db          *bolt.DB
}

func (i *BlockchainIterator) Next() (*Block, error) {
	var block *Block

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

func (bc *Blockchain) Iterator() *BlockchainIterator {
	return &BlockchainIterator{bc.tip, bc.db}
}
