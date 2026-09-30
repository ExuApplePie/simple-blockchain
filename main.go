package main

import (
	"fmt"
	"strconv"

	"github.com/ExuApplePie/simple-blockchain/blockchain"
)

func main() {
	bc := blockchain.NewBlockchain()

	bc.AddBlock("Send 1 BTC to Exusiai")
	bc.AddBlock("Send 2 more BTC to Exusiai")

	for _, block := range bc.Blocks() {
		fmt.Printf("Prev. Hash: %x\n", block.PrevBlockHash)
		fmt.Printf("Data: %s\n", block.Data)
		fmt.Printf("Hash: %x\n", block.Hash)
		pow := blockchain.NewProofOfWork(block)
		fmt.Printf("PoW: %s\n", strconv.FormatBool(pow.Validate()))
		fmt.Println()
	}
}
