package blockchain

import (
	bolt "go.etcd.io/bbolt"
)

const (
	dbFile       = "blockchain.db"
	blocksBucket = "blocks"
)

type Blockchain struct {
	tip []byte
	db  *bolt.DB
}

func (bc *Blockchain) AddBlock(data string) error {
	var lastHash []byte

	err := bc.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))
		lastHash = append([]byte(nil), b.Get([]byte("l"))...)

		return nil
	})
	if err != nil {
		return err
	}

	newBlock := NewBlock(data, lastHash)

	err = bc.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))
		serializedData, err := newBlock.Serialize()
		if err != nil {
			return err
		}
		if err := b.Put(newBlock.Hash, serializedData); err != nil {
			return err
		}
		if err := b.Put([]byte("l"), newBlock.Hash); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	bc.tip = newBlock.Hash

	return nil
}

func NewGenesisBlock() *Block {
	return NewBlock("Genesis Block", []byte{})
}

func NewBlockchain() (*Blockchain, error) {
	var tip []byte
	db, err := bolt.Open(dbFile, 0o600, nil)
	if err != nil {
		return nil, err
	}
	err = db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))

		if b == nil {
			genesis := NewGenesisBlock()
			b, err := tx.CreateBucket([]byte(blocksBucket))
			if err != nil {
				return err
			}
			serializedData, err := genesis.Serialize()
			if err != nil {
				return err
			}
			if err := b.Put(genesis.Hash, serializedData); err != nil {
				return err
			}
			if err := b.Put([]byte("l"), genesis.Hash); err != nil {
				return err
			}
			tip = genesis.Hash
		} else {
			// tip = append([]byte{}, b.Get([]byte("l"))...)
			stored := b.Get([]byte("l"))
			tip = make([]byte, len(stored))
			copy(tip, stored)
		}

		return nil
	})
	if err != nil {
		db.Close()
		return nil, err
	}

	bc := Blockchain{tip: tip, db: db}
	return &bc, nil
}

func (bc *Blockchain) Blocks() []*Block {
	return bc.blocks
}
