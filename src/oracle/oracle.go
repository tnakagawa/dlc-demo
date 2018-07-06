// Package oracle project oracle.go
package oracle

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math/big"

	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcutil/hdkeychain"

	"rpc"
)

// Oracle is the oracle dataset.
type Oracle struct {
	name   string                  // oracle name
	rpc    *rpc.BtcRPC             // bitcoin rpc
	extKey *hdkeychain.ExtendedKey // oracle extendedkey
	params chaincfg.Params         // bitcoin network
}

// NewOracle returns a new Oracle.
func NewOracle(name string, params chaincfg.Params, rpc *rpc.BtcRPC) (*Oracle, error) {
	oracle := new(Oracle)
	oracle.name = name
	oracle.params = params
	oracle.rpc = rpc
	// TODO
	seed := chainhash.DoubleHashB([]byte(oracle.name))
	mExtKey, err := hdkeychain.NewMaster(seed, &params)
	if err != nil {
		log.Printf("hdkeychain.NewMaster error : %v", err)
		return nil, err
	}
	key := mExtKey
	// TODO m/1/2/3/4/5
	path := []uint32{1, 2, 3, 4, 5}
	for _, i := range path {
		key, err = key.Child(i)
		if err != nil {
			log.Printf("key.Child error : %v", err)
			return nil, err
		}
	}
	oracle.extKey = key
	return oracle, nil
}

// Keys is the keys dataset.
type Keys struct {
	Pubkey string   `json:"pubkey"`
	Keys   []string `json:"keys"`
}

// Keys returns the keys data.
func (oracle *Oracle) Keys(height int) ([]byte, error) {
	if height < 0 {
		return nil, fmt.Errorf("invalid params height:%d", height)
	}
	_, pub, _ := oracle.getKeys(height)
	keys := []string{}
	for i := 0; i < chainhash.HashSize; i++ {
		_, key, _ := oracle.getKeys(height, i)
		keys = append(keys, hex.EncodeToString(key.SerializeCompressed()))
	}
	okeys := &Keys{hex.EncodeToString(pub.SerializeCompressed()), keys}
	bs, _ := json.Marshal(okeys)
	return bs, nil
}

// Signs is signatures data format.
type Signs struct {
	Hash  string   `json:"hash"`
	Msgs  []string `json:"msgs"`
	Signs []string `json:"signs"`
}

// Signs returns the signatures data.
func (oracle *Oracle) Signs(height int) ([]byte, error) {
	if height < 0 {
		return nil, fmt.Errorf("invalid params height:%d", height)
	}
	res, err := oracle.rpc.Request("getblockcount")
	if err != nil {
		return nil, err
	}
	count, _ := res.Result.(float64)
	if int(count) < height {
		return nil, fmt.Errorf("block height out of range / %d, %d / diff %d",
			int(count), height, height-int(count))
	}
	res, err = oracle.rpc.Request("getblockhash", height)
	if err != nil {
		return nil, err
	}
	result, _ := res.Result.(string)
	hash, _ := chainhash.NewHashFromStr(result)
	pri, _, _ := oracle.getKeys(height)
	o := pri.D
	msgs := []string{}
	sigs := []string{}
	for i := 0; i < chainhash.HashSize; i++ {
		key, _, _ := oracle.getKeys(height, i)
		r := key.D
		R := key.PubKey()
		m := []byte{hash[i]}
		// s = r - H(R,m)o
		// ho = H(R,m) * o
		ho := new(big.Int).Mul(H(R, m), o)
		// s = r - ho
		s := new(big.Int).Mod(new(big.Int).Sub(r, ho), btcec.S256().N)
		sigs = append(sigs, hex.EncodeToString(s.Bytes()))
		msgs = append(msgs, hex.EncodeToString(m))
	}
	osigs := &Signs{hash.String(), msgs, sigs}
	bs, _ := json.Marshal(osigs)
	return bs, nil
}

func (oracle *Oracle) getKeys(path ...int) (*btcec.PrivateKey, *btcec.PublicKey, error) {
	key := oracle.extKey
	var err error
	for _, i := range path {
		key, err = key.Child(uint32(i))
		if err != nil {
			return nil, nil, err
		}
	}
	prvKey, err := key.ECPrivKey()
	if err != nil {
		return nil, nil, err
	}
	pubKey, err := key.ECPubKey()
	if err != nil {
		return nil, nil, err
	}
	return prvKey, pubKey, nil
}

// Commit returns a message publickey.
func Commit(R, O *btcec.PublicKey, m []byte) *btcec.PublicKey {
	// H(R,m)
	h := H(R, m)
	// - H(R,m)
	h = new(big.Int).Mod(new(big.Int).Neg(h), btcec.S256().N)
	hO := new(btcec.PublicKey)
	// - H(R,m)O
	hO.X, hO.Y = btcec.S256().ScalarMult(O.X, O.Y, h.Bytes())
	// R - H(R,m)O
	P := new(btcec.PublicKey)
	P.X, P.Y = btcec.S256().Add(R.X, R.Y, hO.X, hO.Y)
	return P
}

// H is a hash function.
func H(R *btcec.PublicKey, m []byte) *big.Int {
	s := sha256.New()
	s.Write(R.SerializeUncompressed())
	s.Write(m)
	hash := s.Sum(nil)
	h := new(big.Int).SetBytes(hash)
	h = new(big.Int).Mod(h, btcec.S256().N)
	return h
}
