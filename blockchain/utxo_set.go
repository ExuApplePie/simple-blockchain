package blockchain

import (
	"encoding/hex"
	"errors"
	"fmt"

	bolt "go.etcd.io/bbolt"
	boltErrors "go.etcd.io/bbolt/errors"
)

const (
	utxoBucket = "utxo"
)

// UTXOSet stores and queries the currently unspent transaction outputs.
type UTXOSet struct {
	Blockchain *Blockchain
}

// SpendableOutputs contains the value and output references selected for a spend.
type SpendableOutputs struct {
	Amount  int
	Outputs map[string][]int
}

// Reindex rebuilds the UTXO bucket by scanning the entire blockchain.
func (u UTXOSet) Reindex() error {
	db := u.Blockchain.db
	bucketName := []byte(utxoBucket)

	err := db.Update(func(tx *bolt.Tx) error {
		err := tx.DeleteBucket(bucketName)
		// The first reindex has no bucket to delete; that is an expected state.
		if err != nil && !errors.Is(err, boltErrors.ErrBucketNotFound) {
			return fmt.Errorf("delete UTXO bucket: %w", err)
		}

		_, err = tx.CreateBucket(bucketName)
		if err != nil {
			return fmt.Errorf("create UTXO bucket: %w", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("reset UTXO bucket: %w", err)
	}

	UTXO, err := u.Blockchain.FindUTXO()
	if err != nil {
		return fmt.Errorf("collect unspent transaction outputs: %w", err)
	}

	err = db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketName)

		for txID, outs := range UTXO {
			key, err := hex.DecodeString(txID)
			if err != nil {
				return fmt.Errorf("decode UTXO transaction ID %q: %w", txID, err)
			}
			data, err := outs.Serialize()
			if err != nil {
				return fmt.Errorf("serialize UTXO outputs for transaction %q: %w", txID, err)
			}
			if err = b.Put(key, data); err != nil {
				return fmt.Errorf("store UTXO outputs for transaction %q: %w", txID, err)
			}
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("write UTXO set: %w", err)
	}
	return nil
}

// FindSpendableOutputs selects enough owned outputs to cover amount.
func (u UTXOSet) FindSpendableOutputs(pubKeyHash []byte, amount int) (SpendableOutputs, error) {
	unspentOutputs := make(map[string][]int)
	accumulated := 0
	db := u.Blockchain.db

	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(utxoBucket))
		// The cursor walks each transaction's stored outputs and selects only
		// outputs locked to the requested public-key hash.
		c := b.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			txID := hex.EncodeToString(k)
			outs, err := deserializeUTXOOutputs(k, v)
			if err != nil {
				return err
			}

			for outIdx, out := range outs.Outputs {
				if out.IsLockedWithKey(pubKeyHash) && accumulated < amount {
					accumulated += out.Value
					unspentOutputs[txID] = append(unspentOutputs[txID], outIdx)
				}
			}
		}
		return nil
	})
	if err != nil {
		return SpendableOutputs{}, fmt.Errorf("read spendable outputs: %w", err)
	}

	return SpendableOutputs{Amount: accumulated, Outputs: unspentOutputs}, nil
}

// FindUTXO returns all stored outputs locked to pubKeyHash.
func (u UTXOSet) FindUTXO(pubKeyHash []byte) ([]TXOutput, error) {
	var UTXOs []TXOutput
	db := u.Blockchain.db

	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(utxoBucket))
		// UTXO entries are grouped by transaction ID, but callers receive a
		// flat list filtered by ownership.
		c := b.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			outs, err := deserializeUTXOOutputs(k, v)
			if err != nil {
				return err
			}

			for _, out := range outs.Outputs {
				if out.IsLockedWithKey(pubKeyHash) {
					UTXOs = append(UTXOs, out)
				}
			}
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("read UTXO set: %w", err)
	}

	return UTXOs, nil
}

// Update removes outputs spent by block and adds its newly created outputs.
func (u UTXOSet) Update(block *Block) error {
	db := u.Blockchain.db

	err := db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(utxoBucket))

		for _, tx := range block.Transactions {
			if err := removeSpentOutputs(b, tx); err != nil {
				return err
			}
			if err := addTransactionOutputs(b, tx); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("update UTXO set: %w", err)
	}
	return nil
}

// deserializeUTXOOutputs decodes one transaction's stored UTXO entry.
func deserializeUTXOOutputs(txID, data []byte) (TXOutputs, error) {
	var outs TXOutputs
	if err := DeserializeOutputs(data, &outs); err != nil {
		return TXOutputs{}, fmt.Errorf("deserialize UTXOs for transaction %x: %w", txID, err)
	}
	return outs, nil
}

// removeSpentOutputs applies every input in tx to the UTXO bucket.
func removeSpentOutputs(bucket *bolt.Bucket, tx *Transaction) error {
	if tx.IsCoinbase() {
		return nil
	}

	for _, input := range tx.Vin {
		if err := removeSpentOutput(bucket, input); err != nil {
			return err
		}
	}
	return nil
}

// removeSpentOutput removes one referenced output and preserves its siblings.
func removeSpentOutput(bucket *bolt.Bucket, input TXInput) error {
	outs, err := deserializeUTXOOutputs(input.Txid, bucket.Get(input.Txid))
	if err != nil {
		return fmt.Errorf("load spent outputs for input %x:%d: %w", input.Txid, input.Vout, err)
	}

	remaining := TXOutputs{}
	for outIdx, out := range outs.Outputs {
		if outIdx != input.Vout {
			remaining.Outputs = append(remaining.Outputs, out)
		}
	}

	if len(remaining.Outputs) == 0 {
		if err := bucket.Delete(input.Txid); err != nil {
			return fmt.Errorf("delete exhausted UTXOs for transaction %x: %w", input.Txid, err)
		}
		return nil
	}

	data, err := remaining.Serialize()
	if err != nil {
		return fmt.Errorf("serialize remaining UTXOs for transaction %x: %w", input.Txid, err)
	}
	if err := bucket.Put(input.Txid, data); err != nil {
		return fmt.Errorf("store remaining UTXOs for transaction %x: %w", input.Txid, err)
	}
	return nil
}

// addTransactionOutputs stores all outputs created by tx as one UTXO entry.
func addTransactionOutputs(bucket *bolt.Bucket, tx *Transaction) error {
	outputs := TXOutputs{Outputs: tx.Vout}
	data, err := outputs.Serialize()
	if err != nil {
		return fmt.Errorf("serialize new UTXOs for transaction %x: %w", tx.ID, err)
	}
	if err := bucket.Put(tx.ID, data); err != nil {
		return fmt.Errorf("store new UTXOs for transaction %x: %w", tx.ID, err)
	}
	return nil
}
