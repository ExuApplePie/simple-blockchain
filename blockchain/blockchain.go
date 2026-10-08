package blockchain

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"sync"

	bolt "go.etcd.io/bbolt"
)

const (
	dbFile              = "blockchain.db"
	blocksBucket        = "blocks"
	genesisCoinbaseData = "Exusiai loves apple pie"
)

type Blockchain struct {
	tip []byte
	db  *bolt.DB
	mu  sync.Mutex
}

func (bc *Blockchain) Close() error {
	return bc.db.Close()
}

func (bc *Blockchain) MineBlock(transactions []*Transaction) error {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	for _, tx := range transactions {
		validTransaction, err := bc.VerifyTransaction(tx)
		if err != nil {
			return err
		}
		if validTransaction != true {
			return errors.New("Invalid Transaction")
		}
	}

	newBlock := NewBlock(transactions, bc.tip)

	err := bc.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))
		if b == nil {
			return errors.New("blocks bucket does not exist")
		}
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

func dbExists() bool {
	if _, err := os.Stat(dbFile); err != nil {
		return false
	}
	return true
}

func CreateBlockchain(address string) (*Blockchain, error) {
	if dbExists() {
		return nil, fmt.Errorf("blockchain already exists")
	}
	wallets, err := NewWallets()
	if err != nil {
		return nil, err
	}
	_, err = wallets.GetWallet(address)
	if err != nil {
		return nil, err
	}

	var tip []byte
	db, err := bolt.Open(dbFile, 0o600, nil)
	if err != nil {
		return nil, err
	}
	err = db.Update(func(tx *bolt.Tx) error {
		cbtx, err := NewCoinbaseTX(address, genesisCoinbaseData)
		if err != nil {
			return err
		}
		genesis := NewGenesisBlock(cbtx)

		b, err := tx.CreateBucket([]byte(blocksBucket))
		if err != nil {
			return err
		}
		data, err := genesis.Serialize()
		if err != nil {
			return err
		}
		if err := b.Put(genesis.Hash, data); err != nil {
			return err
		}
		if err := b.Put([]byte("l"), genesis.Hash); err != nil {
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

func OpenBlockchain() (*Blockchain, error) {
	if dbExists() == false {
		return nil, fmt.Errorf("no existing blockchain found. Create one first")
	}

	var tip []byte
	db, err := bolt.Open(dbFile, 0o600, nil)
	if err != nil {
		return nil, err
	}
	err = db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))
		if b == nil {
			return fmt.Errorf("blocks bucket not found")
		}

		// Copy l key since once the transaction is closed, the data will be inaccessible. This is a workaround for the fact that BoltDB does not allow reading data outside of a transaction.
		// tip = append([]byte{}, b.Get([]byte("l"))...)
		stored := b.Get([]byte("l"))
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

func (bc *Blockchain) FindTransaction(ID []byte) (Transaction, error) {
	bci := bc.Iterator()

	for {
		block, err := bci.Next()
		if err != nil {
			return Transaction{}, err
		}

		for _, tx := range block.Transactions {
			if bytes.Compare(tx.ID, ID) == 0 {
				return *tx, nil
			}
		}

		if len(block.PrevBlockHash) == 0 {
			break
		}
	}

	return Transaction{}, errors.New("Transaction not found")
}

func (bc *Blockchain) SignTransaction(tx *Transaction, privKey ecdsa.PrivateKey) error {
	prevTXs := make(map[string]Transaction)

	for _, vin := range tx.Vin {
		prevTX, err := bc.FindTransaction(vin.Txid)
		if err != nil {
			return err
		}
		prevTXs[hex.EncodeToString(prevTX.ID)] = prevTX
	}

	return tx.Sign(privKey, prevTXs)
}

func (bc *Blockchain) VerifyTransaction(tx *Transaction) (bool, error) {
	prevTXs := make(map[string]Transaction)

	for _, vin := range tx.Vin {
		prevTX, err := bc.FindTransaction(vin.Txid)
		if err != nil {
			return false, err
		}
		prevTXs[hex.EncodeToString(prevTX.ID)] = prevTX
	}

	return tx.Verify(prevTXs), nil
}
