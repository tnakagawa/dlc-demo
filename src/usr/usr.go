// Package usr project usr.go
package usr

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/big"
	"time"

	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"

	"dlc"
	"oracle"
	"rpc"
	"wallet"
)

// User is the User dataset.
type User struct {
	name   string          // user name
	rpc    *rpc.BtcRPC     // bitcoin rpc
	wallet *wallet.Wallet  // wallet
	params chaincfg.Params // bitcoin network
	dlc    *dlc.Dlc        // dlc
	status int             // status for dlc
}

// Status
const (
	StatusNone                = 0
	StatusWaitForAccept       = 1
	StatusCanGetSign          = 2
	StatusCanGetAccept        = 10
	StatusWaitForSign         = 20
	StatusWaitSendTx          = 30
	StatusCanSendSettlementTx = 31
)

// NewUser returns a new User.
func NewUser(name string, params chaincfg.Params, rpc *rpc.BtcRPC) (*User, error) {
	user := &User{}
	user.name = name
	user.params = params
	user.rpc = rpc
	user.status = StatusNone
	// TODO
	seed := chainhash.DoubleHashB([]byte(user.name))
	var err error
	user.wallet, err = wallet.NewWallet(params, rpc, seed)
	if err != nil {
		return nil, err
	}
	return user, nil
}

// Name returns user name.
func (u *User) Name() string {
	return u.name
}

// GetBalance returns a balance(satoshi).
func (u *User) GetBalance() int64 {
	return u.wallet.GetBalance()
}

// GetAddress returns a bech32 address.
func (u *User) GetAddress() string {
	return u.wallet.GetAddress()
}

// OfferData is the offer dataset.
type OfferData struct {
	High   bool     `json:"high"`   // bet high?
	Amount int64    `json:"amount"` // amount of fund transaction
	Fefee  int64    `json:"fefee"`  // estimate fee of fund transaction (satoshi/byte)
	Sefee  int64    `json:"sefee"`  // estimate fee of settlement transaction (satoshi/byte)
	Date   string   `json:"date"`   // date of target block
	Length int      `json:"length"` // length of target message
	Pubkey string   `json:"pubkey"` // public key
	Inputs []string `json:"inputs"` // inputs of fund transaction
	Output string   `json:"output"` // inputs of fund transaction
}

// GetOfferData returns Serialized OfferData.
func (u *User) GetOfferData(d *dlc.Dlc) ([]byte, error) {
	if u.status != StatusNone {
		return nil, fmt.Errorf("illegal status : %d", u.status)
	}
	if d == nil {
		return nil, fmt.Errorf("parameter is nil")
	}
	u.dlc = d
	pub := u.wallet.GetPublicKey()
	u.dlc.SetPublicKey(pub, u.dlc.IsA())
	// find inputs(utxo) and output of fund transaction
	tx := wire.NewMsgTx(2)
	amt := half(u.dlc.FundAmount()) + half(u.dlc.SettlementFee()) +
		half(dlc.DlcFundTxBaseSize*u.dlc.FundEstimateFee())
	err := u.wallet.FundTx(tx, amt, u.dlc.FundEstimateFee())
	if err != nil {
		return nil, err
	}
	inputs := []string{}
	txins := []*wire.TxIn{}
	for _, txin := range tx.TxIn {
		op := &txin.PreviousOutPoint
		inputs = append(inputs, hex.EncodeToString(OpToBs(op)))
		txins = append(txins, txin)
	}
	var txout *wire.TxOut
	output := ""
	if len(tx.TxOut) > 0 {
		txout = tx.TxOut[0]
		output = hex.EncodeToString(TxOutToBs(txout))
	}
	u.dlc.SetTxInsAndTxOut(txins, txout, u.dlc.IsA())
	// serialize
	odata := &OfferData{}
	odata.High = d.IsA()
	odata.Amount = d.FundAmount()
	odata.Fefee = d.FundEstimateFee()
	odata.Sefee = d.SettlementEstimateFee()
	odata.Date = d.GameDate().Format(oracle.OracleTimeLayout)
	odata.Length = d.GameLength()
	odata.Pubkey = hex.EncodeToString(pub.SerializeCompressed())
	odata.Inputs = inputs
	odata.Output = output
	bs, _ := json.Marshal(odata)
	u.status = StatusWaitForAccept
	return bs, nil
}

// SetOfferData sets Serialized OfferData.
func (u *User) SetOfferData(data []byte) error {
	if u.status != StatusNone {
		return fmt.Errorf("illegal status : %d", u.status)
	}
	// deserialize
	var odata OfferData
	err := json.Unmarshal(data, &odata)
	if err != nil {
		return err
	}
	pub, err := StrToPub(odata.Pubkey)
	if err != nil {
		return err
	}
	txins, txout, err := StrToInputsOutput(odata.Inputs, odata.Output)
	if err != nil {
		return err
	}
	sfee := odata.Sefee * dlc.DlcSettlementTxSize

	// create Dlc
	u.dlc, err = dlc.NewDlc(half(odata.Amount), half(odata.Amount),
		odata.Fefee, odata.Sefee, half(sfee), half(sfee), !odata.High)
	if err != nil {
		return err
	}
	u.dlc.SetTxInsAndTxOut(txins, txout, odata.High)
	date, err := time.Parse(oracle.OracleTimeLayout, odata.Date)
	if err != nil {
		return err
	}
	u.dlc.SetGameConditions(date, odata.Length)
	u.dlc.SetPublicKey(pub, odata.High)
	u.status = StatusCanGetAccept
	return nil
}

// AcceptData is the accept dataset.
type AcceptData struct {
	Pubkey string   `json:"pubkey"` // public key
	Inputs []string `json:"inputs"` // inputs of fund transaction
	Output string   `json:"output"` // output of fund transaction
	Signs  []string `json:"signs"`  // signatures of the settlement transaction
	Rsign  string   `json:"rsign"`  // signature of the refund transaction
}

// GetAcceptData returns Serialized AcceptData.
func (u *User) GetAcceptData() ([]byte, error) {
	if u.status != StatusCanGetAccept {
		return nil, fmt.Errorf("illegal status : %d", u.status)
	}
	pub := u.wallet.GetPublicKey()
	u.dlc.SetPublicKey(pub, u.dlc.IsA())

	// find inputs(utxo) and output of fund transaction
	tx := wire.NewMsgTx(2)
	amt := u.dlc.FundAmount() + u.dlc.SettlementFee()
	fefee := u.dlc.FundEstimateFee()
	err := u.wallet.FundTx(tx, half(amt)+
		half(dlc.DlcFundTxBaseSize*u.dlc.FundEstimateFee()), fefee)
	if err != nil {
		return nil, err
	}
	inputs := []string{}
	txins := []*wire.TxIn{}
	for _, txin := range tx.TxIn {
		op := &txin.PreviousOutPoint
		inputs = append(inputs, hex.EncodeToString(OpToBs(op)))
		txins = append(txins, wire.NewTxIn(op, nil, nil))
	}
	var txout *wire.TxOut
	output := ""
	if len(tx.TxOut) > 0 {
		txout = tx.TxOut[0]
		output = hex.EncodeToString(TxOutToBs(txout))
	}
	u.dlc.SetTxInsAndTxOut(txins, txout, u.dlc.IsA())

	// create the signatures of the settlement transaction
	high := !u.dlc.IsA()
	rates := u.dlc.Rates()
	signs := []string{}
	script := u.dlc.FundScript()
	for _, rate := range rates {
		stx := u.dlc.SettlementTx(rate, high)
		if stx == nil {
			signs = append(signs, "")
			continue
		}
		sign, serr := u.wallet.GetWitnessSignature(stx, 0, amt, script, pub)
		if serr != nil {
			return nil, serr
		}
		signs = append(signs, hex.EncodeToString(sign))
	}

	// create the signature of the refund transaction
	rtx := u.dlc.RefundTx()
	if tx == nil {
		return nil, fmt.Errorf("RefundTx is nil")
	}
	rsign, err := u.wallet.GetWitnessSignature(rtx, 0, amt, script, pub)
	if err != nil {
		return nil, err
	}
	u.dlc.SetRefundSign(rsign, u.dlc.IsA())

	// serialize
	adata := &AcceptData{}
	adata.Pubkey = hex.EncodeToString(pub.SerializeCompressed())
	adata.Inputs = inputs
	adata.Output = output
	adata.Signs = signs
	adata.Rsign = hex.EncodeToString(rsign)
	bs, _ := json.Marshal(adata)
	u.status = StatusWaitForSign
	return bs, nil
}

// SetAcceptData sets Serialized AcceptData.
func (u *User) SetAcceptData(data []byte) error {
	if u.status != StatusWaitForAccept {
		return fmt.Errorf("illegal status : %d", u.status)
	}
	// deserialize
	var adata AcceptData
	err := json.Unmarshal(data, &adata)
	if err != nil {
		return err
	}
	pub, err := StrToPub(adata.Pubkey)
	if err != nil {
		return err
	}
	u.dlc.SetPublicKey(pub, !u.dlc.IsA())
	txins, txout, err := StrToInputsOutput(adata.Inputs, adata.Output)
	if err != nil {
		return err
	}
	u.dlc.SetTxInsAndTxOut(txins, txout, !u.dlc.IsA())

	// verify the signatures of the settlement transaction
	err = u.VerifySettlementTxSigns(adata.Signs)
	if err != nil {
		return err
	}

	rsign, err := hex.DecodeString(adata.Rsign)
	if err != nil {
		return err
	}
	// verify signature of the refund transaction
	err = u.dlc.VerifyRefundTx(rsign, pub)
	if err != nil {
		return err
	}
	u.dlc.SetRefundSign(rsign, !u.dlc.IsA())
	u.status = StatusCanGetSign
	return nil
}

// VerifySettlementTxSigns verifies the signatures of settlement transaction.
func (u *User) VerifySettlementTxSigns(signs []string) error {
	rates := u.dlc.Rates()
	if len(rates) != len(signs) {
		return fmt.Errorf("size Error : %d, %d", len(rates), len(signs))
	}
	high := u.dlc.IsA()
	pub := u.dlc.PublicKey(!high)
	for i, sign := range signs {
		rate := rates[i]
		if sign == "" {
			if rate.Amount(high) != 0 {
				return fmt.Errorf("not found sign. rate : %+v", rate)
			}
			continue
		}
		s, err := hex.DecodeString(sign)
		if err != nil {
			return err
		}
		err = u.dlc.Verify(rate, high, s, pub)
		if err != nil {
			return err
		}
	}
	return nil
}

// SignData is the sign dataset.
type SignData struct {
	Ftws  [][]string `json:"ftws"`  // witnesses of the fund transaction
	Signs []string   `json:"signs"` // signatures of the settlement transaction
	Rsign string     `json:"rsign"` // signature of the refund transaction
}

// GetSignData returns Serialized SignData.
func (u *User) GetSignData() ([]byte, error) {
	if u.status != StatusCanGetSign {
		return nil, fmt.Errorf("illegal status : %d", u.status)
	}
	// create the signatures of the settlement transaction
	pub := u.dlc.PublicKey(u.dlc.IsA())
	high := !u.dlc.IsA()
	rates := u.dlc.Rates()
	signs := []string{}
	amt := u.dlc.FundAmount() + u.dlc.SettlementFee()
	script := u.dlc.FundScript()
	for _, rate := range rates {
		tx := u.dlc.SettlementTx(rate, high)
		if tx == nil {
			signs = append(signs, "")
			continue
		}
		sign, err := u.wallet.GetWitnessSignature(tx, 0, amt, script, pub)
		if err != nil {
			return nil, err
		}
		signs = append(signs, hex.EncodeToString(sign))
	}

	// create the witnesses of the fund transaction
	tws := []wire.TxWitness{}
	tx := u.dlc.FundTx()
	err := u.wallet.SignTx(tx)
	if err != nil {
		return nil, err
	}
	for _, txin := range tx.TxIn {
		if txin.Witness != nil {
			tws = append(tws, txin.Witness)
		}
	}

	// create the signature of the refund transaction
	rtx := u.dlc.RefundTx()
	if tx == nil {
		return nil, fmt.Errorf("RefundTx is nil")
	}
	rsign, err := u.wallet.GetWitnessSignature(rtx, 0, amt, script, pub)
	if err != nil {
		return nil, err
	}
	u.dlc.SetRefundSign(rsign, u.dlc.IsA())

	// serialize
	sdata := &SignData{}
	sdata.Ftws = TwsToSss(tws)
	sdata.Signs = signs
	sdata.Rsign = hex.EncodeToString(rsign)
	bs, _ := json.Marshal(sdata)
	u.status = StatusWaitSendTx
	return bs, nil
}

// SetSignData sets Serialized SignData.
func (u *User) SetSignData(data []byte) error {
	if u.status != StatusWaitForSign {
		return fmt.Errorf("illegal status : %d", u.status)
	}
	// deserialize
	var sdata SignData
	err := json.Unmarshal(data, &sdata)
	if err != nil {
		return err
	}

	// witnesses of the fund transaction
	tws, err := SssToTws(sdata.Ftws)
	if err != nil {
		return err
	}
	txins := u.dlc.FundTxIns(!u.dlc.IsA())
	if len(tws) != len(txins) {
		return fmt.Errorf("illegal length %d, %d", len(tws), len(txins))
	}
	for i := range txins {
		txins[i].Witness = tws[i]
	}

	// verify the signatures of the settlement transaction
	err = u.VerifySettlementTxSigns(sdata.Signs)
	if err != nil {
		return err
	}

	rsign, err := hex.DecodeString(sdata.Rsign)
	if err != nil {
		return err
	}
	// verify signature of the refund transaction
	pub := u.dlc.PublicKey(!u.dlc.IsA())
	err = u.dlc.VerifyRefundTx(rsign, pub)
	if err != nil {
		return err
	}
	u.dlc.SetRefundSign(rsign, !u.dlc.IsA())

	u.status = StatusWaitSendTx
	return nil
}

// SendFundTx sends the fund transaction.
func (u *User) SendFundTx() error {
	if u.status != StatusWaitSendTx {
		return fmt.Errorf("illegal status : %d", u.status)
	}
	tx := u.dlc.FundTx()
	err := u.wallet.SignTx(tx)
	if err != nil {
		return err
	}
	txid, err := u.wallet.SendTx(tx)
	if err != nil {
		return err
	}
	fmt.Printf("%s sends the Fund Transaction :%v\n", u.name, txid)
	fmt.Printf("txout[%d]: %10d / %x\n", 0, tx.TxOut[0].Value, tx.TxOut[0].PkScript)
	return nil
}

// GameDate returns the date for game.
func (u *User) GameDate() time.Time {
	return u.dlc.GameDate()
}

// SetOracleKeys sets Serialized OracleKeys.
func (u *User) SetOracleKeys(data []byte) error {
	var okeys oracle.Keys
	err := json.Unmarshal(data, &okeys)
	if err != nil {
		return err
	}
	pub, err := StrToPub(okeys.Pubkey)
	if err != nil {
		return err
	}
	keys := []*btcec.PublicKey{}
	for _, key := range okeys.Keys {
		p, err := StrToPub(key)
		if err != nil {
			return err
		}
		keys = append(keys, p)
	}
	u.dlc.SetOracleKeys(pub, keys)
	return nil
}

// SetOracleSigns sets Serialized OracleSigns.
func (u *User) SetOracleSigns(data []byte) error {
	if u.status != StatusWaitSendTx {
		return fmt.Errorf("illegal status : %d", u.status)
	}
	var osigs oracle.Signs
	err := json.Unmarshal(data, &osigs)
	if err != nil {
		return err
	}
	// hash, err := chainhash.NewHashFromStr(osigs.Hash)
	// if err != nil {
	// 	return err
	// }
	signs := []*big.Int{}
	for _, sign := range osigs.Signs {
		bs, e := hex.DecodeString(sign)
		if e != nil {
			return e
		}
		signs = append(signs, new(big.Int).SetBytes(bs))
	}
	err = u.dlc.SetOracleSigns(osigs.Value, signs)
	if err != nil {
		return err
	}
	rate := u.dlc.FixedRate()
	if rate == nil {
		return nil
	}
	if rate.Amount(u.dlc.IsA()) > u.dlc.FundAmount()/2 {
		fmt.Printf("%-5s Win  %v\n", u.name, rate)
		return nil
	}
	fmt.Printf("%-5s Lose %v\n", u.name, rate)
	return nil
}

// SendSettlementTx sends the settlement transaction.
func (u *User) SendSettlementTx() error {
	rate := u.dlc.FixedRate()
	if rate == nil {
		return fmt.Errorf("rate no fix")
	}
	sign1 := rate.ReceivedSign()
	high := u.dlc.IsA()
	tx := u.dlc.SettlementTx(rate, high)
	if tx == nil {
		return fmt.Errorf("no transaction")
	}
	pub := u.dlc.PublicKey(high)
	amt := u.dlc.FundAmount() + u.dlc.SettlementFee()
	script := u.dlc.FundScript()
	sign2, err := u.wallet.GetWitnessSignature(tx, 0, amt, script, pub)
	if err != nil {
		return err
	}
	var witness [][]byte
	witness = append(witness, []byte{})
	if high {
		witness = append(witness, sign2)
		witness = append(witness, sign1)
	} else {
		witness = append(witness, sign1)
		witness = append(witness, sign2)
	}
	witness = append(witness, script)
	tx.TxIn[0].Witness = witness
	txid, err := u.wallet.SendTx(tx)
	if err != nil {
		return err
	}
	fmt.Printf("%s sends the Settlement Transaction : %v\n", u.name, txid)
	for idx, txin := range tx.TxIn {
		fmt.Printf("txin [%d]: %v\n", idx, txin.PreviousOutPoint)
	}
	for idx, txout := range tx.TxOut {
		fmt.Printf("txout[%d]: %10d / %x\n", idx, txout.Value, txout.PkScript)
	}
	return nil
}

// SendSettlementTxTo sends the settlement amount to wallet.
func (u *User) SendSettlementTxTo(efee int64) error {
	rate := u.dlc.FixedRate()
	high := u.dlc.IsA()
	pub := u.dlc.PublicKey(high)
	pkScript := u.wallet.P2WPKHpkScript(u.wallet.GetPublicKey())
	tx, amt, script, err := u.dlc.SettlementToTx(rate, high, pkScript, efee)
	if err != nil {
		return err
	}
	sign, err := u.wallet.GetWitnessSignaturePlus(
		tx, 0, amt, script, pub, rate.MessageSign())
	if err != nil {
		return err
	}
	var witness [][]byte
	witness = append(witness, sign)
	witness = append(witness, []byte{1})
	witness = append(witness, script)
	tx.TxIn[0].Witness = witness
	txid, err := u.wallet.SendTx(tx)
	if err != nil {
		return err
	}
	fmt.Printf("%s forwards the Settlement Transaction : %v\n", u.name, txid)
	for idx, txin := range tx.TxIn {
		fmt.Printf("txin [%d]: %v\n", idx, txin.PreviousOutPoint)
	}
	for idx, txout := range tx.TxOut {
		fmt.Printf("txout[%d]: %10d / %x\n", idx, txout.Value, txout.PkScript)
	}
	return nil
}

// SendRefundTx sends the refund transaction.
func (u *User) SendRefundTx() error {
	tx := u.dlc.RefundTx()
	u.rpc.View = true
	txid, err := u.wallet.SendTx(tx)
	u.rpc.View = false
	if err != nil {
		return err
	}
	fmt.Printf("%s sends the Refund Transaction : %v\n", u.name, txid)
	for idx, txin := range tx.TxIn {
		fmt.Printf("txin [%d]: %v\n", idx, txin.PreviousOutPoint)
	}
	for idx, txout := range tx.TxOut {
		fmt.Printf("txout[%d]: %10d / %x\n", idx, txout.Value, txout.PkScript)
	}
	return nil
}

// ClearDlc clear user dlc.
func (u *User) ClearDlc() {
	u.dlc = nil
	u.status = StatusNone
}

func half(value int64) int64 {
	return int64(math.Ceil(float64(value) / float64(2)))
}

// Serialize and Deserialize

// MsgTxToBs change transaction to bytes.
func MsgTxToBs(tx *wire.MsgTx) []byte {
	buf := &bytes.Buffer{}
	tx.Serialize(buf)
	return buf.Bytes()
}

// OpToBs changes OutPoint to byte array.
func OpToBs(op *wire.OutPoint) []byte {
	bs := []byte{}
	bs = append(bs, op.Hash[:]...)
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, op.Index)
	bs = append(bs, b...)
	return bs
}

// BsToOp changes byte array to OutPoint.
func BsToOp(bs []byte) (*wire.OutPoint, error) {
	if len(bs) != 36 {
		return nil, fmt.Errorf("illegal size : %d", len(bs))
	}
	hash, _ := chainhash.NewHash(bs[:32])
	idx := binary.LittleEndian.Uint32(bs[32:])
	return wire.NewOutPoint(hash, idx), nil
}

// TxOutToBs TxOut to changes byte array.
func TxOutToBs(txout *wire.TxOut) []byte {
	bs := []byte{}
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, uint64(txout.Value))
	bs = append(bs, b...)
	buf := new(bytes.Buffer)
	err := wire.WriteVarInt(buf, 0, uint64(len(txout.PkScript)))
	if err != nil {
		log.Printf("wire.WriteVarInt Error %+v", err)
		return bs
	}
	bs = append(bs, buf.Bytes()...)
	bs = append(bs, txout.PkScript...)
	return bs
}

// BsToTxOut changes byte array to TxOut.
func BsToTxOut(bs []byte) (*wire.TxOut, error) {
	if len(bs) <= 8 {
		return nil, fmt.Errorf("illegal size : %d", len(bs))
	}
	buf := bytes.NewBuffer(bs[8:])
	length, err := wire.ReadVarInt(buf, 0)
	if err != nil {
		return nil, err
	}
	size := wire.VarIntSerializeSize(length)
	if len(bs) != 8+size+int(length) {
		return nil, fmt.Errorf("illegal size : %d", len(bs))
	}
	value := binary.LittleEndian.Uint64(bs[:8])
	pkScript := bs[8+size:]
	txout := wire.NewTxOut(int64(value), pkScript)
	return txout, nil
}

// TwsToSss changes TxWitness array to string arrays.
func TwsToSss(tws []wire.TxWitness) [][]string {
	sss := [][]string{}
	for _, tw := range tws {
		ss := []string{}
		for _, bs := range tw {
			ss = append(ss, hex.EncodeToString(bs))
		}
		sss = append(sss, ss)
	}
	return sss
}

// SssToTws changes string arrays to TxWitness array.
func SssToTws(sss [][]string) ([]wire.TxWitness, error) {
	tws := []wire.TxWitness{}
	for _, ss := range sss {
		tw := wire.TxWitness{}
		for _, s := range ss {
			bs, err := hex.DecodeString(s)
			if err != nil {
				return nil, err
			}
			tw = append(tw, bs)
		}
		tws = append(tws, tw)
	}
	return tws, nil
}

// StrToPub changes string to publickey.
func StrToPub(str string) (*btcec.PublicKey, error) {
	bs, err := hex.DecodeString(str)
	if err != nil {
		return nil, err
	}
	pub, err := btcec.ParsePubKey(bs, btcec.S256())
	if err != nil {
		return nil, err
	}
	return pub, nil
}

// StrToInputsOutput changes string to inputs and output.
func StrToInputsOutput(inputs []string, output string) ([]*wire.TxIn, *wire.TxOut, error) {
	txins := []*wire.TxIn{}
	for _, input := range inputs {
		bs, err := hex.DecodeString(input)
		if err != nil {
			return nil, nil, err
		}
		op, err := BsToOp(bs)
		if err != nil {
			return nil, nil, err
		}
		txins = append(txins, wire.NewTxIn(op, nil, nil))
	}
	var txout *wire.TxOut
	if output != "" {
		bs, err := hex.DecodeString(output)
		if err != nil {
			return nil, nil, err
		}
		txout, err = BsToTxOut(bs)
		if err != nil {
			return nil, nil, err
		}
	}
	return txins, txout, nil
}
