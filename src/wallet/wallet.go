// Package wallet project wallet.go
package wallet

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	"math/rand"
	"sort"
	"time"

	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
	"github.com/btcsuite/btcutil/hdkeychain"

	"dlc"
	"rpc"
)

// Wallet is wallet
type Wallet struct {
	extKey *hdkeychain.ExtendedKey
	params chaincfg.Params
	size   int
	rpc    *rpc.BtcRPC
	infos  []*Info
}

// Info is info data.
type Info struct {
	idx uint32
	pub *btcec.PublicKey
	adr string
}

// NewWallet returns a new Wallet
func NewWallet(params chaincfg.Params, rpc *rpc.BtcRPC, seed []byte) (*Wallet, error) {
	wallet := &Wallet{}
	wallet.params = params
	wallet.rpc = rpc
	wallet.size = 16
	mExtKey, err := hdkeychain.NewMaster(seed, &params)
	if err != nil {
		log.Printf("hdkeychain.NewMaster error : %v", err)
		return nil, err
	}
	key := mExtKey
	// m/44'/coin-type'/0'/0
	path := []uint32{44 | hdkeychain.HardenedKeyStart,
		params.HDCoinType | hdkeychain.HardenedKeyStart,
		0 | hdkeychain.HardenedKeyStart, 0}
	for _, i := range path {
		key, err = key.Child(i)
		if err != nil {
			log.Printf("key.Child error : %v", err)
			return nil, err
		}
	}
	wallet.extKey = key
	wallet.infos = []*Info{}
	for i := 0; i < wallet.size; i++ {
		key, _ := wallet.extKey.Child(uint32(i))
		pub, _ := key.ECPubKey()
		adr, _ := btcutil.NewAddressWitnessPubKeyHash(btcutil.Hash160(pub.SerializeCompressed()), &wallet.params)
		info := &Info{uint32(i), pub, adr.EncodeAddress()}
		wallet.infos = append(wallet.infos, info)
		_, err = rpc.Request("importaddress", adr.EncodeAddress(), "", false)
		if err != nil {
			return nil, err
		}
	}
	return wallet, nil
}

// ListUnspent returns utxo list.
func (w *Wallet) ListUnspent() ([]btcjson.ListUnspentResult, error) {
	adrs := []string{}
	for _, info := range w.infos {
		adrs = append(adrs, info.adr)
	}
	res, err := w.rpc.Request("listunspent", 1, 9999999, adrs)
	if err != nil {
		return nil, err
	}
	list := []btcjson.ListUnspentResult{}
	err = res.UnmarshalResult(&list)
	if err != nil {
		return nil, err
	}
	var utxos Utxos = list
	sort.Sort(utxos)
	return list, nil
}

// Utxos is type for sorting.
type Utxos []btcjson.ListUnspentResult

func (u Utxos) Len() int {
	return len(u)
}

func (u Utxos) Less(i, j int) bool {
	if u[i].Confirmations == u[j].Confirmations {
		return u[i].Amount < u[j].Amount
	}
	return u[i].Confirmations > u[j].Confirmations
}

func (u Utxos) Swap(i, j int) {
	u[i], u[j] = u[j], u[i]
}

// GetPublicKey returns public key for random.
func (w *Wallet) GetPublicKey() *btcec.PublicKey {
	rand.Seed(time.Now().UnixNano())
	i := rand.Intn(len(w.infos))
	info := w.infos[i]
	return info.pub
}

// GetAddress returns bech32 address for random.
func (w *Wallet) GetAddress() string {
	rand.Seed(time.Now().UnixNano())
	i := rand.Intn(len(w.infos))
	info := w.infos[i]
	return info.adr
}

// GetBalance returns amounts (satoshi).
func (w *Wallet) GetBalance() int64 {
	total := int64(0)
	list, err := w.ListUnspent()
	if err != nil {
		log.Printf("Error : %+v", err)
		return total
	}
	for _, utxo := range list {
		a, _ := btcutil.NewAmount(utxo.Amount)
		total += int64(a)
	}
	return total
}

// FundTx adds inputs to a transaction until amount.
func (w *Wallet) FundTx(tx *wire.MsgTx, amount, efee int64) error {
	list, err := w.ListUnspent()
	if err != nil {
		return err
	}
	outs := []*wire.OutPoint{}
	total := int64(0)
	addfee := int64(0)
	for _, utxo := range list {
		txid, _ := chainhash.NewHashFromStr(utxo.TxID)
		outs = append(outs, wire.NewOutPoint(txid, utxo.Vout))
		a, _ := btcutil.NewAmount(utxo.Amount)
		total += int64(a)
		addfee = int64(len(outs)) * dlc.DlcTxInSize * efee
		if amount+addfee <= total {
			if amount+addfee == total {
				break
			}
			addfee += dlc.DlcTxOutSize * efee
			if amount+addfee <= total {
				break
			}
		}
	}
	if amount+addfee > total {
		return fmt.Errorf("short of bitcoin")
	}
	for _, out := range outs {
		tx.AddTxIn(wire.NewTxIn(out, nil, nil))
	}
	if amount+addfee == total {
		return nil
	}
	change := total - (amount + addfee)
	pkScript := w.P2WPKHpkScript(w.GetPublicKey())
	tx.AddTxOut(wire.NewTxOut(change, pkScript))
	return nil
}

// SignTx signs the transaction inputs of known utxo.
func (w *Wallet) SignTx(tx *wire.MsgTx) error {
	list, err := w.ListUnspent()
	if err != nil {
		return err
	}
	for idx, txin := range tx.TxIn {
		txid := txin.PreviousOutPoint.Hash.String()
		vout := txin.PreviousOutPoint.Index
		var utxo *btcjson.ListUnspentResult
		for _, item := range list {
			if item.TxID == txid && item.Vout == vout {
				utxo = &item
				break
			}
		}
		if utxo == nil {
			continue
		}
		var pri *btcec.PrivateKey
		var pub *btcec.PublicKey
		for _, info := range w.infos {
			if info.adr == utxo.Address {
				key, _ := w.extKey.Child(info.idx)
				pri, _ = key.ECPrivKey()
				pub = info.pub
				break
			}
		}
		sighash := txscript.NewTxSigHashes(tx)
		script := w.P2WPKHpkScript(pub)
		amt, _ := btcutil.NewAmount(utxo.Amount)
		sign, err := txscript.RawTxInWitnessSignature(tx, sighash, idx, int64(amt),
			script, txscript.SigHashAll, pri)
		if err != nil {
			return err
		}
		var witness [][]byte
		witness = append(witness, sign)
		witness = append(witness, pub.SerializeCompressed())
		txin.Witness = witness
	}
	return nil
}

// GetWitnessSignature returns signature
func (w *Wallet) GetWitnessSignature(tx *wire.MsgTx, idx int, amt int64,
	script []byte, pub *btcec.PublicKey) ([]byte, error) {
	return w.GetWitnessSignaturePlus(tx, idx, amt, script, pub, nil)
}

// GetWitnessSignaturePlus returns signature for added private key
func (w *Wallet) GetWitnessSignaturePlus(tx *wire.MsgTx, idx int, amt int64,
	script []byte, pub *btcec.PublicKey, add *big.Int) ([]byte, error) {
	var pri *btcec.PrivateKey
	for _, info := range w.infos {
		if info.pub.IsEqual(pub) {
			key, _ := w.extKey.Child(info.idx)
			pri, _ = key.ECPrivKey()
		}
	}
	if pri == nil {
		return nil, fmt.Errorf("unknown public key %x", pub.SerializeCompressed())
	}
	if add != nil {
		num := new(big.Int).Mod(new(big.Int).Add(pri.D, add), btcec.S256().N)
		pri, _ = btcec.PrivKeyFromBytes(btcec.S256(), num.Bytes())
	}
	sighash := txscript.NewTxSigHashes(tx)
	sign, err := txscript.RawTxInWitnessSignature(tx, sighash, idx, amt, script, txscript.SigHashAll, pri)
	if err != nil {
		return nil, err
	}
	return sign, nil
}

// SendTx submits transaction to local node and network.
func (w *Wallet) SendTx(tx *wire.MsgTx) (*chainhash.Hash, error) {
	buf := &bytes.Buffer{}
	err := tx.Serialize(buf)
	if err != nil {
		return nil, err
	}
	res, err := w.rpc.Request("sendrawtransaction", hex.EncodeToString(buf.Bytes()))
	if err != nil {
		return nil, err
	}
	txid, _ := res.Result.(string)
	return chainhash.NewHashFromStr(txid)
}

// P2WPKHpkScript creates P2WPKH pkScript
func (w *Wallet) P2WPKHpkScript(pub *btcec.PublicKey) []byte {
	// P2WPKH is OP_0 + HASH160(<public key>)
	builder := txscript.NewScriptBuilder()
	builder.AddOp(txscript.OP_0)
	builder.AddData(btcutil.Hash160(pub.SerializeCompressed()))
	pkScript, _ := builder.Script()
	return pkScript
}
