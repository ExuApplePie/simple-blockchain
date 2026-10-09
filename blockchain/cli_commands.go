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
		return err
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
			return err
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
		return err
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
	fmt.Println("Done!")

	return nil
}

// getBalance calculates and prints the spendable balance for address.
func (cli *CLI) getBalance(address string) (err error) {
	bc, err := OpenBlockchain()
	if err != nil {
		return err
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
	balance := 0
	pubKeyHash, err := PubKeyHashFromAddress(address)
	if err != nil {
		return err
	}
	utxos, err := bc.FindUTXO(pubKeyHash)
	if err != nil {
		return err
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
		return err
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
	// Constructing the transaction selects inputs, creates change, and signs it;
	// MineBlock then verifies that signed transaction before appending the block.
	tx, err := NewUTXOTransaction(from, to, amount, bc)
	if err != nil {
		return err
	}
	if err := bc.MineBlock([]*Transaction{tx}); err != nil {
		return err
	}
	fmt.Println("Success!")

	return nil
}

func createWallet() error {
	wallets, err := NewWallets()
	if err != nil {
		return err
	}

	address, err := wallets.CreateWallet()
	if err != nil {
		return err
	}
	if err := wallets.SaveToFile(); err != nil {
		return err
	}

	fmt.Printf("Your new address : %s\n", address)
	return nil
}

func listAddresses() error {
	wallets, err := NewWallets()
	if err != nil {
		return err
	}
	for _, address := range wallets.GetAddresses() {
		fmt.Printf("%s\n", address)
	}
	return nil
}
