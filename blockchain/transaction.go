package blockchain

import "encoding/json"

const subsidy = 10 // This is the reward given, just leaving as constant instead of dividing in half every X amount of blocks

// TXOutput represents coins locked to a public-key hash.
type TXOutput struct {
	Value      int
	PubKeyHash []byte
}

// TXOutputs collects transaction outputs for a single transaction ID.
type TXOutputs struct {
	Outputs []TXOutput
}

// TXInput references an output from an earlier transaction and proves ownership
// of that output with a signature and public key.
type TXInput struct {
	Txid      []byte
	Vout      int
	Signature []byte
	PubKey    []byte
}

// Transaction transfers value from referenced inputs to new outputs.
type Transaction struct {
	ID   []byte
	Vin  []TXInput
	Vout []TXOutput
}

// Serialize encodes the transaction as JSON for storage and hashing.
func (tx *Transaction) Serialize() ([]byte, error) {
	return json.Marshal(tx)
}

// Serialize encodes transaction outputs as JSON for UTXO storage.
func (outs *TXOutputs) Serialize() ([]byte, error) {
	return json.Marshal(outs)
}

// DeserializeOutputs decodes serialized transaction outputs into output.
func DeserializeOutputs(data []byte, output *TXOutputs) error {
	return json.Unmarshal(data, output)
}

// IsCoinbase reports whether the transaction creates the initial block reward
// rather than spending an earlier transaction output.
func (tx *Transaction) IsCoinbase() bool {
	return len(tx.Vin) == 1 && len(tx.Vin[0].Txid) == 0 && tx.Vin[0].Vout == -1
}
