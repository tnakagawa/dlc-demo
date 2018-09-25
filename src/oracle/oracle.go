// Package oracle project oracle.go
package oracle

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcutil/hdkeychain"

	"rpc"
)

// OracleTimeLayout is layout of time
const OracleTimeLayout = "20060102"

// Oracle is the oracle dataset.
type Oracle struct {
	name   string                  // oracle name
	rpc    *rpc.BtcRPC             // bitcoin rpc
	extKey *hdkeychain.ExtendedKey // oracle extendedkey
	params chaincfg.Params         // bitcoin network
	digit  int
	value  map[string][]int
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
	// TODO
	oracle.digit = 1
	oracle.value = map[string][]int{}
	oracle.extKey = key
	return oracle, nil
}

// Keys is the keys dataset.
type Keys struct {
	Pubkey string   `json:"pubkey"`
	Keys   []string `json:"keys"`
}

// Keys returns the keys data.
func (oracle *Oracle) Keys(t time.Time) ([]byte, error) {
	_, pub, _ := oracle.getKeys(t.Year(), int(t.Month()), t.Day())
	keys := []string{}
	for i := 0; i < oracle.digit; i++ {
		_, key, _ := oracle.getKeys(t.Year(), int(t.Month()), t.Day(), i)
		keys = append(keys, hex.EncodeToString(key.SerializeCompressed()))
	}
	okeys := &Keys{hex.EncodeToString(pub.SerializeCompressed()), keys}
	bs, _ := json.Marshal(okeys)
	return bs, nil
}

// Signs is signatures data format.
type Signs struct {
	Value string   `json:"value"`
	Msgs  []string `json:"msgs"`
	Signs []string `json:"signs"`
}

// Signs returns the signatures data.
func (oracle *Oracle) Signs(t time.Time) ([]byte, error) {
	key := t.Format(OracleTimeLayout)
	vals, ok := oracle.value[key]
	if !ok {
		return nil, fmt.Errorf("not found value %s", key)
	}
	pri, _, _ := oracle.getKeys(t.Year(), int(t.Month()), t.Day())
	o := pri.D
	msgs := []string{}
	sigs := []string{}
	val := ""
	for i := 0; i < len(vals); i++ {
		key, _, _ := oracle.getKeys(t.Year(), int(t.Month()), t.Day(), i)
		r := key.D
		R := key.PubKey()
		// TODO
		m := big.NewInt(int64(vals[i])).Bytes()
		if len(m) == 0 {
			m = []byte{0x00}
		}
		// TODO string of value
		if len(val) > 0 {
			val += "," + strconv.Itoa(vals[i])
		} else {
			val += strconv.Itoa(vals[i])
		}
		// s = r - H(R,m)o
		// ho = H(R,m) * o
		ho := new(big.Int).Mul(H(R, m), o)
		// s = r - ho
		s := new(big.Int).Mod(new(big.Int).Sub(r, ho), btcec.S256().N)
		sigs = append(sigs, hex.EncodeToString(s.Bytes()))
		msgs = append(msgs, hex.EncodeToString(m))
	}
	osigs := &Signs{val, msgs, sigs}
	bs, _ := json.Marshal(osigs)
	return bs, nil
}

// SetVals sets date and values
func (oracle *Oracle) SetVals(d string, v string) error {
	vs := strings.Split(v, ",")
	if len(vs) != oracle.digit {
		return fmt.Errorf("values size error : %s", v)
	}
	vals := []int{}
	for _, v := range vs {
		val, err := strconv.Atoi(v)
		if err != nil {
			return err
		}
		vals = append(vals, val)
	}
	date, err := time.Parse(OracleTimeLayout, d)
	if err != nil {
		return err
	}
	oracle.value[date.Format(OracleTimeLayout)] = vals
	return nil
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
