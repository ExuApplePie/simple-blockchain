package blockchain

import (
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

const (
	walletFile = "wallets.dat"
)

// Wallets stores wallets indexed by their addresses.
type Wallets struct {
	Wallets map[string]*Wallet
}

type storedWallets struct {
	Wallets map[string]storedWallet `json:"wallets"`
}

// NewWallets loads the wallet collection from disk, or creates an empty
// collection when the wallet file does not exist yet.
func NewWallets() (*Wallets, error) {
	wallets := Wallets{}
	wallets.Wallets = make(map[string]*Wallet)

	if err := wallets.LoadFromFile(); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("load wallets: %w", err)
		}
		// Just use empty map if file doesn't exist for first run
	}
	return &wallets, nil
}

// CreateWallet generates and stores a wallet, returning its address.
func (wl *Wallets) CreateWallet() (string, error) {
	wallet, err := NewWallet()
	if err != nil {
		return "", fmt.Errorf("generate wallet: %w", err)
	}
	address := wallet.GetAddress()

	wl.Wallets[address] = wallet

	return address, nil
}

// GetWallet returns the wallet stored under address.
func (wl *Wallets) GetWallet(address string) (*Wallet, error) {
	wallet, ok := wl.Wallets[address]
	if !ok {
		return nil, errors.New("wallet not found")
	}
	return wallet, nil
}

// GetAddresses returns all addresses stored in the collection.
func (wl *Wallets) GetAddresses() []string {
	var addresses []string

	for address := range wl.Wallets {
		addresses = append(addresses, address)
	}

	return addresses
}

// SaveToFile serializes all wallets to the wallet file.
func (wl *Wallets) SaveToFile() error {
	// Store key material in serializable forms rather than marshaling
	// elliptic.Curve directly, which JSON cannot encode.
	stored := storedWallets{
		Wallets: make(map[string]storedWallet, len(wl.Wallets)),
	}

	for address, wallet := range wl.Wallets {
		privateKey, err := x509.MarshalECPrivateKey(&wallet.PrivateKey)
		if err != nil {
			return fmt.Errorf("serialize private key for wallet %q: %w", address, err)
		}

		stored.Wallets[address] = storedWallet{
			PrivateKey: privateKey,
			PublicKey:  wallet.PublicKey,
		}
	}

	data, err := json.MarshalIndent(stored, "", "\t")
	if err != nil {
		return fmt.Errorf("serialize wallets: %w", err)
	}
	if err := os.WriteFile(walletFile, data, 0o0600); err != nil {
		return fmt.Errorf("write wallet file %q: %w", walletFile, err)
	}
	return nil
}

// LoadFromFile replaces the in-memory wallets with wallets read from disk.
func (wl *Wallets) LoadFromFile() error {
	fileContent, err := os.ReadFile(walletFile)
	if err != nil {
		return fmt.Errorf("read wallet file %q: %w", walletFile, err)
	}
	var stored storedWallets
	if err := json.Unmarshal(fileContent, &stored); err != nil {
		return fmt.Errorf("parse wallet file %q: %w", walletFile, err)
	}

	// Reconstruct each private key from its standard X.509 representation and
	// pair it with the saved serialized public key.
	for address, saved := range stored.Wallets {
		privateKey, err := x509.ParseECPrivateKey(saved.PrivateKey)
		if err != nil {
			return fmt.Errorf("parse private key for wallet %q: %w", address, err)
		}

		wl.Wallets[address] = &Wallet{
			PrivateKey: *privateKey,
			PublicKey:  saved.PublicKey,
		}
	}

	return nil
}
