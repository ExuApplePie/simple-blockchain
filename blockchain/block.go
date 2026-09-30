package blockchain

import (
	"encoding/json"
	"time"
)

type Block struct {
	Timestamp     int64
	Data          []byte
	PrevBlockHash []byte
	Hash          []byte
	Nonce         int
}

func NewBlock(data string, prevBlockHash []byte) *Block {
	block := &Block{time.Now().Unix(), []byte(data), prevBlockHash, []byte{}, 0}
	pow := NewProofOfWork(block)
	nonce, hash := pow.Run()

	block.Hash = hash[:]
	block.Nonce = nonce

	return block
}

func (b *Block) Serialize() ([]byte, error) {
	return json.Marshal(b)
}

func DeserializeBlock(d []byte) (*Block, error) {
	var block Block

	if err := json.Unmarshal(d, &block); err != nil {
		return nil, err
	}
	return &block, nil
}
