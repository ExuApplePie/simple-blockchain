package blockchain

import (
	"encoding/json"
	"fmt"
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
	result, err := pow.Run()
	if err != nil {
		return nil, fmt.Errorf("run proof of work: %w", err)
	}

	block.Hash = result.Hash
	block.Nonce = result.Nonce

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
		return nil, fmt.Errorf("deserialize block JSON: %w", err)
	}
	return &block, nil
}

// HashTransactions returns the hash of all transaction IDs in the block.
func (b *Block) HashTransactions() ([]byte, error) {
	if b == nil {
		return nil, fmt.Errorf("hash block transactions: block is nil")
	}
	if len(b.Transactions) == 0 {
		return nil, fmt.Errorf("hash block transactions: block contains no transactions")
	}

	var transactions [][]byte

	for index, tx := range b.Transactions {
		if tx == nil {
			return nil, fmt.Errorf("hash block transactions: transaction at index %d is nil", index)
		}
		data, err := tx.Serialize()
		if err != nil {
			return nil, fmt.Errorf("serialize transaction at index %d: %w", index, err)
		}
		transactions = append(transactions, data)
	}
	mTree := NewMerkleTree(transactions)

	if mTree == nil || mTree.RootNode == nil {
		return nil, fmt.Errorf("hash block transactions: Merkle tree has no root node")
	}

	return mTree.RootNode.Data, nil
}
