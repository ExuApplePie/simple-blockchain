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

// NewProofOfWork creates a proof-of-work calculator for b.
func NewProofOfWork(b *Block) *ProofOfWork {
	target := big.NewInt(1)
	target.Lsh(target, uint(256-targetBits))

	pow := &ProofOfWork{b, target}

	return pow
}

func (pow *ProofOfWork) prepareMiningData(nonce int) []byte {
	data := append([]byte{}, pow.block.PrevBlockHash...)
	data = append(data, pow.block.HashTransactions()...)

	data = fmt.Appendf(data, "%x", pow.block.Timestamp)
	data = fmt.Appendf(data, "%x", targetBits)
	data = fmt.Appendf(data, "%x", nonce)

	return data
}

// Run searches for a nonce whose hash satisfies the configured target.
func (pow *ProofOfWork) Run() (int, []byte, error) {
	var hashInt big.Int
	var hash [32]byte
	nonce := 0
	maxNonce := math.MaxInt64

	// Try nonces until the hash is below the target; changing only the nonce
	// lets us search without changing the transactions being committed.
	fmt.Printf("Mining the block containing \"%v\"\n", pow.block.Transactions)
	for nonce < maxNonce {
		data := pow.prepareMiningData(nonce)
		hash = sha256.Sum256(data)
		fmt.Printf("\r%x", hash)
		hashInt.SetBytes(hash[:])

		if hashInt.Cmp(pow.target) == -1 {
			return nonce, hash[:], nil
		} else {
			nonce++
		}
	}
	fmt.Print("\n\n")

	return 0, nil, errors.New("proof of work exhausted nonce range")
}

// Validate checks that the stored nonce and hash satisfy the proof-of-work
// target and match the block's contents.
func (pow *ProofOfWork) Validate() bool {
	var hashInt big.Int

	data := pow.prepareMiningData(pow.block.Nonce)
	hash := sha256.Sum256(data)
	hashInt.SetBytes(hash[:])

	isValid := hashInt.Cmp(pow.target) == -1 && bytes.Equal(pow.block.Hash, hash[:])

	return isValid
}
