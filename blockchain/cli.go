package blockchain

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
)

type CLI struct{}

func NewCLI() *CLI {
	return &CLI{}
}

func (cli *CLI) printChain() error {
	bc, err := OpenBlockchain()
	if err != nil {
		return err
	}
	defer func() {
		if err := bc.Close(); err != nil {
			fmt.Printf("failed to close database: %v", err)
		}
	}()
	bci := bc.Iterator()

	for {
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

func (cli *CLI) createBlockchain(address string) error {
	_, err := CreateBlockchain(address)
	if err != nil {
		return err
	}
	fmt.Println("Done!")

	return nil
}

func (cli *CLI) getBalance(address string) error {
	bc, err := OpenBlockchain()
	if err != nil {
		return err
	}
	defer func() {
		if err := bc.Close(); err != nil {
			fmt.Printf("failed to close database: %v", err)
		}
	}()

	balance := 0
	pubKeyHash, err := PubKeyHashFromAddress(address)
	if err != nil {
		return err
	}
	UTXOs, err := bc.FindUTXO(pubKeyHash)
	if err != nil {
		return err
	}

	for _, out := range UTXOs {
		balance += out.Value
	}

	fmt.Printf("Balance of '%s': %d\n", address, balance)
	return nil
}

func (cli *CLI) send(from, to string, amount int) error {
	bc, err := OpenBlockchain()
	if err != nil {
		return err
	}
	defer func() {
		if err := bc.Close(); err != nil {
			fmt.Printf("failed to close database: %v", err)
		}
	}()
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

func (cli *CLI) validateArgs() error {
	if len(os.Args) < 2 {
		return errors.New("command required")
	}
	return nil
}

func (cli *CLI) printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  createblockchain -address ADDRESS - Create a blockchain and send genesis block reward to ADDRESS")
	fmt.Println("  createwallet - Generates a new key-pair and saves it into the wallet file")
	fmt.Println("  getbalance -address ADDRESS - Get balance of ADDRESS")
	fmt.Println("  listaddresses - Lists all addresses from the wallet file")
	fmt.Println("  printchain - Print all the blocks of the blockchain")
	fmt.Println("  send -from FROM -to TO -amount AMOUNT - Send AMOUNT of coins from FROM address to TO")
}

func (cli *CLI) Run() error {
	if err := cli.validateArgs(); err != nil {
		cli.printUsage()
		return err
	}

	createBlockchainCmd := flag.NewFlagSet("createblockchain", flag.ContinueOnError)
	printChainCmd := flag.NewFlagSet("printchain", flag.ContinueOnError)
	getBalanceCmd := flag.NewFlagSet("getbalance", flag.ContinueOnError)
	sendCmd := flag.NewFlagSet("send", flag.ContinueOnError)

	createBlockchainAddress := createBlockchainCmd.String("address", "", "The address to send genesis block reward to")
	getBalanceAddress := getBalanceCmd.String("address", "", "The address to check the balance of")
	sendAmountSender := sendCmd.String("from", "", "sender address")
	sendAmountReceiver := sendCmd.String("to", "", "recipient address")
	sendAmountValue := sendCmd.Int("amount", 0, "amount to send")

	switch os.Args[1] {
	case "printchain":
		if err := printChainCmd.Parse(os.Args[2:]); err != nil {
			return err
		}
	case "createblockchain":
		if err := createBlockchainCmd.Parse(os.Args[2:]); err != nil {
			return err
		}
	case "getbalance":
		if err := getBalanceCmd.Parse(os.Args[2:]); err != nil {
			return err
		}
	case "send":
		if err := sendCmd.Parse(os.Args[2:]); err != nil {
			return err
		}
	case "createwallet":
		if len(os.Args[2:]) != 0 {
			return errors.New("createwallet takes no arguments")
		}
		return createWallet()
	case "listaddresses":
		if len(os.Args[2:]) != 0 {
			return errors.New("listaddresses takes no arguments")
		}
		return listAddresses()
	default:
		cli.printUsage()
		return fmt.Errorf("unknown command: %s", os.Args[1])
	}

	if createBlockchainCmd.Parsed() {
		if *createBlockchainAddress == "" {
			createBlockchainCmd.Usage()
			return fmt.Errorf("address cannot be empty")
		}
		if err := cli.createBlockchain(*createBlockchainAddress); err != nil {
			return err
		}
	}
	if printChainCmd.Parsed() {
		return cli.printChain()
	}
	if getBalanceCmd.Parsed() {
		if *getBalanceAddress == "" {
			getBalanceCmd.Usage()
			return fmt.Errorf("address cannot be empty")
		}
		if err := cli.getBalance(*getBalanceAddress); err != nil {
			return err
		}
	}
	if sendCmd.Parsed() {
		if *sendAmountReceiver == "" {
			sendCmd.Usage()
			return fmt.Errorf("receiver cannot be empty")
		}
		if *sendAmountSender == "" {
			sendCmd.Usage()
			return fmt.Errorf("sender cannot be empty")
		}
		if *sendAmountReceiver == *sendAmountSender {
			sendCmd.Usage()
			return fmt.Errorf("sender and receiver cannot be the same person")
		}
		if *sendAmountValue <= 0 {
			sendCmd.Usage()
			return fmt.Errorf("value to send must be greater than 0")
		}
		if err := cli.send(*sendAmountSender, *sendAmountReceiver, *sendAmountValue); err != nil {
			return err
		}
	}

	return nil
}
