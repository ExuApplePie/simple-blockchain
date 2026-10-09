package blockchain

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"time"
)

// Block groups transactions, links to the previous block, and stores its
// proof-of-work result.
type Block struct {
	Timestamp     int64
	Transactions  []*Transaction
	PrevBlockHash []byte
	Hash          []byte
	Nonce         int
}

// NewBlock creates and mines a block that follows prevBlockHash.
func NewBlock(transactions []*Transaction, prevBlockHash []byte) (*Block, error) {
	// Start with the block contents, then let proof-of-work fill in the nonce
	// and hash that make this block computationally valid.
	block := &Block{time.Now().Unix(), transactions, prevBlockHash, []byte{}, 0}
	pow := NewProofOfWork(block)
	nonce, hash, err := pow.Run()
	if err != nil {
		return nil, err
	}

	block.Hash = hash[:]
	block.Nonce = nonce

	return block, nil
}

// NewGenesisBlock creates the first block in a blockchain.
func NewGenesisBlock(coinbase *Transaction) (*Block, error) {
	return NewBlock([]*Transaction{coinbase}, []byte{})
}

// Serialize encodes the block as JSON for database storage.
func (b *Block) Serialize() ([]byte, error) {
	return json.Marshal(b)
}

// DeserializeBlock decodes a JSON-encoded block.
func DeserializeBlock(d []byte) (*Block, error) {
	var block Block

	if err := json.Unmarshal(d, &block); err != nil {
		return nil, err
	}
	return &block, nil
}

// HashTransactions returns the hash of all transaction IDs in the block.
func (b *Block) HashTransactions() []byte {
	var txHashes [][]byte
	var txHash [32]byte
	// The block commits to every transaction ID through one combined hash.
	for _, tx := range b.Transactions {
		txHashes = append(txHashes, tx.ID)
	}
	txHash = sha256.Sum256(bytes.Join(txHashes, []byte{}))

	return txHash[:]
}
