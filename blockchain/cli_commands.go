package blockchain

import (
	"fmt"
	"strconv"
)

// printChain displays blocks from the tip back to the genesis block.
func (cli *CLI) printChain() (err error) {
	// Keep the database open for the complete iteration and close it even when
	// printing or deserializing a block fails.
	bc, err := OpenBlockchain()
	if err != nil {
		return fmt.Errorf("open blockchain for printing: %w", err)
	}
	defer func() {
		if closeErr := bc.Close(); closeErr != nil {
			if err == nil {
				err = closeErr
			} else {
				err = fmt.Errorf("%w (also failed to close database: %v)", err, closeErr)
			}
		}
	}()
	bci := bc.Iterator()

	for {
		// Each call returns the current block and advances toward genesis.
		block, err := bci.Next()
		if err != nil {
			return fmt.Errorf("read next block while printing chain: %w", err)
		}

		fmt.Printf("Prev. hash: %x\n", block.PrevBlockHash)
		fmt.Printf("Transaction: %v\n", block.Transactions)
		fmt.Printf("Hash: %x\n", block.Hash)
		pow := NewProofOfWork(block)
		fmt.Printf("PoW: %s\n", strconv.FormatBool(pow.Validate()))
		fmt.Println()

		if len(block.PrevBlockHash) == 0 {
			break
		}
	}
	return nil
}

// createBlockchain creates a blockchain and closes its database handle after
// initialization.
func (cli *CLI) createBlockchain(address string) (err error) {
	bc, err := CreateBlockchain(address)
	if err != nil {
		return fmt.Errorf("create blockchain for CLI: %w", err)
	}
	defer func() {
		if closeErr := bc.Close(); closeErr != nil {
			if err == nil {
				err = closeErr
			} else {
				err = fmt.Errorf("%w (also failed to close database: %v)", err, closeErr)
			}
		}
	}()
	UTXOSet := UTXOSet{bc}
	if err := UTXOSet.Reindex(); err != nil {
		return fmt.Errorf("reindex UTXO set: %w", err)
	}
	fmt.Println("Done!")

	return nil
}

// getBalance calculates and prints the spendable balance for address.
func (cli *CLI) getBalance(address string) (err error) {
	bc, err := OpenBlockchain()
	if err != nil {
		return fmt.Errorf("open blockchain for balance: %w", err)
	}
	defer func() {
		if closeErr := bc.Close(); closeErr != nil {
			if err == nil {
				err = closeErr
			} else {
				err = fmt.Errorf("%w (also failed to close database: %v)", err, closeErr)
			}
		}
	}()

	// Addresses are converted to key hashes at the CLI boundary; storage only
	// compares the resulting hashes against output locks.
	UTXOSet := UTXOSet{bc}
	balance := 0
	pubKeyHash, err := PubKeyHashFromAddress(address)
	if err != nil {
		return fmt.Errorf("decode balance address: %w", err)
	}
	utxos, err := UTXOSet.FindUTXO(pubKeyHash)
	if err != nil {
		return fmt.Errorf("find balance UTXOs: %w", err)
	}

	for _, out := range utxos {
		balance += out.Value
	}

	fmt.Printf("Balance of '%s': %d\n", address, balance)
	return nil
}

// send creates and mines a signed transaction from one address to another.
func (cli *CLI) send(from, to string, amount int) (err error) {
	bc, err := OpenBlockchain()
	if err != nil {
		return fmt.Errorf("open blockchain for send: %w", err)
	}
	UTXOSet := UTXOSet{bc}
	defer func() {
		if closeErr := bc.Close(); closeErr != nil {
			if err == nil {
				err = closeErr
			} else {
				err = fmt.Errorf("%w (also failed to close database: %v)", err, closeErr)
			}
		}
	}()
	// Constructing the transaction selects inputs, creates change, and signs it;
	// MineBlock then verifies that signed transaction before appending the block.
	tx, err := NewUTXOTransaction(from, to, amount, &UTXOSet)
	if err != nil {
		return fmt.Errorf("create transaction: %w", err)
	}
	cbTx, err := NewCoinbaseTX(from, "")
	if err != nil {
		return fmt.Errorf("create mining reward: %w", err)
	}
	txs := []*Transaction{cbTx, tx}

	newBlock, err := bc.MineBlock(txs)
	if err != nil {
		return fmt.Errorf("mine block for send: %w", err)
	}
	// TODO: Persist the block and update the UTXO set in the same BoltDB
	// transaction so a successful block write cannot leave the UTXO set stale.
	if err := UTXOSet.Update(newBlock); err != nil {
		return fmt.Errorf("update UTXO set after send: %w", err)
	}
	fmt.Println("Success!")

	return nil
}

func createWallet() error {
	wallets, err := NewWallets()
	if err != nil {
		return fmt.Errorf("load wallets for creation: %w", err)
	}

	address, err := wallets.CreateWallet()
	if err != nil {
		return fmt.Errorf("create wallet: %w", err)
	}
	if err := wallets.SaveToFile(); err != nil {
		return fmt.Errorf("save new wallet: %w", err)
	}

	fmt.Printf("Your new address : %s\n", address)
	return nil
}

func listAddresses() error {
	wallets, err := NewWallets()
	if err != nil {
		return fmt.Errorf("load wallets for address listing: %w", err)
	}
	for _, address := range wallets.GetAddresses() {
		fmt.Printf("%s\n", address)
	}
	return nil
}
