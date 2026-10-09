package blockchain

import (
	"os"
	"testing"
)

func TestCLICoreWalletAndTransactionFlow(t *testing.T) {
	t.Chdir(t.TempDir())
	originalTargetBits := targetBits
	targetBits = 8
	defer func() { targetBits = originalTargetBits }()

	cli := NewCLI()
	run := func(args ...string) error {
		originalArgs := os.Args
		originalStdout := os.Stdout
		sink, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
		if err != nil {
			return err
		}
		defer func() {
			os.Args = originalArgs
			os.Stdout = originalStdout
			_ = sink.Close()
		}()

		os.Args = append([]string{"simple-blockchain"}, args...)
		os.Stdout = sink
		return cli.Run()
	}

	// Create the first wallet and use it as the genesis recipient.
	if err := run("createwallet"); err != nil {
		t.Fatalf("createwallet failed: %v", err)
	}
	owner := onlyAddress(t)
	if _, err := os.Stat(walletFile); err != nil {
		t.Fatalf("expected test wallet file: %v", err)
	}

	if err := run("createblockchain", "-address", owner); err != nil {
		t.Fatalf("createblockchain failed: %v", err)
	}
	if _, err := os.Stat(dbFile); err != nil {
		t.Fatalf("expected test blockchain database: %v", err)
	}

	if got := balanceFor(t, owner); got != subsidy {
		t.Fatalf("genesis balance = %d, want %d", got, subsidy)
	}

	// Create a recipient and send part of the genesis reward.
	if err := run("createwallet"); err != nil {
		t.Fatalf("second createwallet failed: %v", err)
	}
	addresses := loadAddresses(t)
	if len(addresses) != 2 {
		t.Fatalf("wallet count = %d, want 2", len(addresses))
	}
	recipient := addresses[0]
	if recipient == owner {
		recipient = addresses[1]
	}

	if err := run("send", "-from", owner, "-to", recipient, "-amount", "3"); err != nil {
		t.Fatalf("send failed: %v", err)
	}
	if got := balanceFor(t, owner); got != subsidy-3 {
		t.Fatalf("owner balance after send = %d, want %d", got, subsidy-3)
	}
	if got := balanceFor(t, recipient); got != 3 {
		t.Fatalf("recipient balance after send = %d, want 3", got)
	}

	// The owner has 7 coins after the transfer, so spending 8 must fail.
	err := run("send", "-from", owner, "-to", recipient, "-amount", "8")
	if err == nil {
		t.Fatal("send succeeded with insufficient funds")
	}
	if err.Error() != "Not enough funds" {
		t.Fatalf("insufficient-funds error = %v, want Not enough funds", err)
	}
	if got := balanceFor(t, owner); got != subsidy-3 {
		t.Fatalf("owner balance changed after rejected send = %d, want %d", got, subsidy-3)
	}
}

func onlyAddress(t *testing.T) string {
	t.Helper()
	addresses := loadAddresses(t)
	if len(addresses) != 1 {
		t.Fatalf("wallet count = %d, want 1", len(addresses))
	}
	return addresses[0]
}

func loadAddresses(t *testing.T) []string {
	t.Helper()
	wallets, err := NewWallets()
	if err != nil {
		t.Fatalf("load wallets: %v", err)
	}
	return wallets.GetAddresses()
}

func balanceFor(t *testing.T, address string) int {
	t.Helper()
	bc, err := OpenBlockchain()
	if err != nil {
		t.Fatalf("open blockchain: %v", err)
	}
	defer func() {
		if err := bc.Close(); err != nil {
			t.Errorf("close blockchain: %v", err)
		}
	}()

	pubKeyHash, err := PubKeyHashFromAddress(address)
	if err != nil {
		t.Fatalf("decode address: %v", err)
	}
	utxos, err := bc.FindUTXO(pubKeyHash)
	if err != nil {
		t.Fatalf("find UTXOs: %v", err)
	}

	balance := 0
	for _, utxo := range utxos {
		balance += utxo.Value
	}
	return balance
}
