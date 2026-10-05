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

type Wallets struct {
	Wallets map[string]*Wallet
}

type storedWallets struct {
	Wallets map[string]storedWallet `json:"wallets"`
}

func NewWallets() (*Wallets, error) {
	wallets := Wallets{}
	wallets.Wallets = make(map[string]*Wallet)

	if err := wallets.LoadFromFile(); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		// Just use empty map if file doesn't exist for first run
	}
	return &wallets, nil
}

func (wl *Wallets) CreateWallet() (string, error) {
	wallet, err := NewWallet()
	if err != nil {
		return "", err
	}
	address := fmt.Sprintf("%s", wallet.GetAddress())

	wl.Wallets[address] = wallet

	return address, nil
}

func (wl *Wallets) GetAddresses() []string {
	var addresses []string

	for address := range wl.Wallets {
		addresses = append(addresses, address)
	}

	return addresses
}

func (wl *Wallets) SaveToFile() error {
	stored := storedWallets{
		Wallets: make(map[string]storedWallet, len(wl.Wallets)),
	}

	for address, wallet := range wl.Wallets {
		privateKey, err := x509.MarshalECPrivateKey(&wallet.PrivateKey)
		if err != nil {
			return err
		}

		stored.Wallets[address] = storedWallet{
			PrivateKey: privateKey,
			PublicKey:  wallet.PublicKey,
		}
	}

	data, err := json.MarshalIndent(stored, "", "\t")
	if err != nil {
		return err
	}
	return os.WriteFile(walletFile, data, 0o0600)
}

func (wl *Wallets) LoadFromFile() error {
	fileContent, err := os.ReadFile(walletFile)
	if err != nil {
		return err
	}
	var stored storedWallets
	if err := json.Unmarshal(fileContent, &stored); err != nil {
		return err
	}

	for address, saved := range stored.Wallets {
		privateKey, err := x509.ParseECPrivateKey(saved.PrivateKey)
		if err != nil {
			return err
		}

		wl.Wallets[address] = &Wallet{
			PrivateKey: *privateKey,
			PublicKey:  saved.PublicKey,
		}
	}

	return nil
}
