package blockchain

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"math"
	"math/big"
)

// targetBits is the production proof-of-work difficulty. Tests may temporarily
// lower it so integration tests do not spend most of their time mining.
var targetBits = 20

// ProofOfWork mines and validates a block's nonce and hash.
type ProofOfWork struct {
	block  *Block
	target *big.Int
}

// MiningResult contains the nonce and hash that satisfy the proof-of-work target.
type MiningResult struct {
	Nonce int
	Hash  []byte
}

// NewProofOfWork creates a proof-of-work calculator for b.
func NewProofOfWork(b *Block) *ProofOfWork {
	target := big.NewInt(1)
	target.Lsh(target, uint(256-targetBits))

	pow := &ProofOfWork{b, target}

	return pow
}

func (pow *ProofOfWork) prepareMiningData(nonce int) ([]byte, error) {
	if pow == nil || pow.block == nil {
		return nil, fmt.Errorf("prepare proof-of-work data: proof of work or block is nil")
	}

	data := append([]byte{}, pow.block.PrevBlockHash...)
	txHash, err := pow.block.HashTransactions()
	if err != nil {
		return nil, fmt.Errorf("prepare proof-of-work data: hash block transactions: %w", err)
	}
	data = append(data, txHash...)

	data = fmt.Appendf(data, "%x", pow.block.Timestamp)
	data = fmt.Appendf(data, "%x", targetBits)
	data = fmt.Appendf(data, "%x", nonce)

	return data, nil
}

// Run searches for a nonce whose hash satisfies the configured target.
func (pow *ProofOfWork) Run() (MiningResult, error) {
	if pow == nil || pow.block == nil {
		return MiningResult{}, fmt.Errorf("mine block: proof of work or block is nil")
	}

	var hashInt big.Int
	var hash [32]byte
	nonce := 0
	maxNonce := math.MaxInt64

	// Try nonces until the hash is below the target; changing only the nonce
	// lets us search without changing the transactions being committed.
	fmt.Printf("Mining the block containing \"%v\"\n", pow.block.Transactions)
	for nonce < maxNonce {
		data, err := pow.prepareMiningData(nonce)
		if err != nil {
			return MiningResult{}, fmt.Errorf("mine block at nonce %d: %w", nonce, err)
		}
		hash = sha256.Sum256(data)
		fmt.Printf("\r%x", hash)
		hashInt.SetBytes(hash[:])

		if hashInt.Cmp(pow.target) == -1 {
			return MiningResult{Nonce: nonce, Hash: hash[:]}, nil
		} else {
			nonce++
		}
	}
	fmt.Print("\n\n")

	return MiningResult{}, errors.New("proof of work exhausted nonce range")
}

// Validate checks that the stored nonce and hash satisfy the proof-of-work
// target and match the block's contents.
func (pow *ProofOfWork) Validate() (bool, error) {
	if pow == nil || pow.block == nil {
		return false, fmt.Errorf("validate proof of work: proof of work or block is nil")
	}

	var hashInt big.Int

	data, err := pow.prepareMiningData(pow.block.Nonce)
	if err != nil {
		return false, fmt.Errorf("validate proof of work: %w", err)
	}
	hash := sha256.Sum256(data)
	hashInt.SetBytes(hash[:])

	isValid := hashInt.Cmp(pow.target) == -1 && bytes.Equal(pow.block.Hash, hash[:])

	return isValid, nil
}
