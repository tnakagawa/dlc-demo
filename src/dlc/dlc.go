// Package dlc project dlc.go
package dlc

import (
	"fmt"
	"math/big"

	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
)

// DlcSettlementTxSize is the size(byte) for settlement transaction.
const DlcSettlementTxSize = int64(345)

// DlcFundTxBaseSize is the size(byte) for fund transaction including 1 output.
const DlcFundTxBaseSize = int64(55)

// DlcTxInSize is the size(byte) per txin.
const DlcTxInSize = int64(149)

// DlcTxOutSize is the size(byte) per txout.
const DlcTxOutSize = int64(31)

// Dlc is the dlc dataset.
type Dlc struct {
	famta    int64            // Fund amount a (satoshi)
	famtb    int64            // Fund amount b (satoshi)
	fefee    int64            // Fund estimate fee (satotshi/byte)
	sefee    int64            // Settlement estimate fee (satotshi/byte)
	sfeea    int64            // Settlement fee a (satoshi)
	sfeeb    int64            // Settlement fee b (satoshi)
	isA      bool             // Is this contract a's?
	locktime uint32           // Refund transaction locktime
	puba     *btcec.PublicKey // Public key a
	pubb     *btcec.PublicKey // Public key b
	atxins   []*wire.TxIn     // Fund outpoints a
	btxins   []*wire.TxIn     // Fund outpoints b
	txouta   *wire.TxOut      // Fund txout a
	txoutb   *wire.TxOut      // Fund txout b
	rsigna   []byte           // Refund signature a
	rsignb   []byte           // Refund signature b
	game     *Game            // Game
}

// Rate is the rate dataset.
type Rate struct {
	msgs  [][]byte         // Settlement messages
	amta  int64            // Settlement amount a
	amtb  int64            // Settlement amount b
	key   *btcec.PublicKey // Settlement messages public key
	rsign []byte           // Signature of settlement transaction received
	msign *big.Int         // Fixed messages sign
	txid  *chainhash.Hash  // Settlement txid signed by itself
}

// NewRate returns a new Rate.
func NewRate(msgs [][]byte, amta, amtb int64) *Rate {
	rate := &Rate{}
	rate.msgs = msgs // message (byte array)
	rate.amta = amta // amount a (satoshi)
	rate.amtb = amtb // amount b (satotshi)
	return rate
}

// String returns information for rate.
func (r *Rate) String() string {
	str := fmt.Sprintf("msgs:%x", r.msgs)
	str += fmt.Sprintf("/amount_A,B:%d,%d", r.amta, r.amtb)
	if r.key != nil {
		str += fmt.Sprintf("/key:%x", r.key.SerializeCompressed())
	} else {
		str += fmt.Sprintf("/key:<nil>")
	}
	str += fmt.Sprintf("/sign:%x", r.rsign)
	str += fmt.Sprintf("/msgs_sign:%v", r.msign)
	str += fmt.Sprintf("/txid:%v", r.txid)
	return str
}

// Amount returns the amount of A or B.
func (r *Rate) Amount(isA bool) int64 {
	if isA {
		return r.amta
	}
	return r.amtb
}

// ReceivedSign returns signature of settlement transaction received.
func (r *Rate) ReceivedSign() []byte {
	return r.rsign
}

// MessageSign returns sign of message.
func (r *Rate) MessageSign() *big.Int {
	return r.msign
}

// NewDlc returns a new Dlc.
func NewDlc(famta, famtb, fefee, sefee, sfeea, sfeeb int64, isA bool) (*Dlc, error) {
	d := &Dlc{}
	d.famta = famta // Fund amount a (satoshi)
	d.famtb = famtb // Fund amount b (satoshi)
	d.fefee = fefee // Fund estimate fee (satotshi/byte)
	d.sefee = sefee // Settlement estimate fee (satotshi/byte)
	d.sfeea = sfeea // Settlement fee a (satoshi)
	d.sfeeb = sfeeb // Settlement fee b (satoshi)
	d.isA = isA     // Is this contract a's?
	return d, nil
}

// SetPublicKey sets the public key of A or B.
func (d *Dlc) SetPublicKey(pub *btcec.PublicKey, isA bool) {
	if isA {
		d.puba = pub
	} else {
		d.pubb = pub
	}
}

// SetTxInsAndTxOut sets txins and txout of A or B.
func (d *Dlc) SetTxInsAndTxOut(txins []*wire.TxIn, txout *wire.TxOut, isA bool) {
	if isA {
		d.atxins = txins
		d.txouta = txout
	} else {
		d.btxins = txins
		d.txoutb = txout
	}
}

// FundTxIns returns the txins of A or B for fund transaction.
func (d *Dlc) FundTxIns(isA bool) []*wire.TxIn {
	if isA {
		return d.atxins
	}
	return d.btxins
}

// SetRefundSign sets signs of A or B for refund transaction.
func (d *Dlc) SetRefundSign(sign []byte, isA bool) {
	if isA {
		d.rsigna = sign
	} else {
		d.rsignb = sign
	}
}

// SetGame sets the Game.
func (d *Dlc) SetGame(game *Game) {
	d.game = game
	d.locktime = uint32(game.GameHeight() + 144)
}

// SetOracleKeys sets the public key of oracle and the public keys of message.
func (d *Dlc) SetOracleKeys(pub *btcec.PublicKey, keys []*btcec.PublicKey) {
	d.game.SetOracleKeys(pub, keys)
}

// SetOracleSigs sets the block hash and message signatures.
func (d *Dlc) SetOracleSigs(hash *chainhash.Hash, signs []*big.Int) error {
	msgs := [][]byte{}
	for i := 0; i < chainhash.HashSize; i++ {
		msgs = append(msgs, []byte{hash[i]})
	}
	if len(msgs) != len(signs) {
		return fmt.Errorf("illegal parameters %v,%x", hash, signs)
	}
	err := d.game.SetOracleSigns(msgs, signs)
	if err != nil {
		return err
	}
	d.game.SetHash(hash)
	return nil
}

// IsA returns true if the Dlc is A otherwise it returns false.
func (d *Dlc) IsA() bool {
	return d.isA
}

// FundAmount returns the total amount for fund transaction.
func (d *Dlc) FundAmount() int64 {
	return d.famta + d.famtb
}

// SettlementFee returns the total fee for settlement transaction.
func (d *Dlc) SettlementFee() int64 {
	return d.sfeea + d.sfeeb
}

// FundEstimateFee returns the estimated fee(satoshi per byte) for fund transaction.
func (d *Dlc) FundEstimateFee() int64 {
	return d.fefee
}

// SettlementEstimateFee returns the estimated fee(satoshi per byte) for settlement transaction.
func (d *Dlc) SettlementEstimateFee() int64 {
	return d.sefee
}

// GameHeight returns the height of the block.
func (d *Dlc) GameHeight() int {
	return d.game.GameHeight()
}

// GameLen returns the game length.
func (d *Dlc) GameLen() int {
	return d.game.GameLength()
}

// Rates returns rate array.
func (d *Dlc) Rates() []*Rate {
	return d.game.Rates()
}

// PublicKey returns the public key of A or B.
func (d *Dlc) PublicKey(isA bool) *btcec.PublicKey {
	if isA {
		return d.puba
	}
	return d.pubb
}

// FundScript returns a funds script.
func (d *Dlc) FundScript() []byte {
	if d.puba == nil || d.pubb == nil {
		return nil
	}
	// fund script:
	// OP_2
	//   <public key a>
	//   <public key b>
	// OP_2
	// OP_CHECKMULTISIG
	builder := txscript.NewScriptBuilder()
	builder.AddOp(txscript.OP_2)
	builder.AddData(d.puba.SerializeCompressed())
	builder.AddData(d.pubb.SerializeCompressed())
	builder.AddOp(txscript.OP_2)
	builder.AddOp(txscript.OP_CHECKMULTISIG)
	script, _ := builder.Script()
	return script
}

// SettlementScript returns settlement script.
func SettlementScript(pub1, pub2 *btcec.PublicKey) []byte {
	// settlement script:
	// OP_IF
	//   <public key a/b add message keys>
	// OP_ELSE
	//   delay(fix 144?)
	//   OP_CHECKSEQUENCEVERIFY
	//   OP_DROP
	//   <public key b/a>
	// OP_ENDIF
	// OP_CHECKSIG
	delay := uint16(144)
	csvflg := uint32(0x00000000)
	builder := txscript.NewScriptBuilder()
	builder.AddOp(txscript.OP_IF)
	builder.AddData(pub1.SerializeCompressed())
	builder.AddOp(txscript.OP_ELSE)
	builder.AddInt64(int64(delay) + int64(csvflg))
	builder.AddOp(txscript.OP_CHECKSEQUENCEVERIFY)
	builder.AddOp(txscript.OP_DROP)
	builder.AddData(pub2.SerializeCompressed())
	builder.AddOp(txscript.OP_ENDIF)
	builder.AddOp(txscript.OP_CHECKSIG)
	script, _ := builder.Script()
	return script
}

// FundTx returns fund transaction.
func (d *Dlc) FundTx() *wire.MsgTx {
	// fund transaction
	// input:
	//   [*]:inputs of a
	//   [*]:inputs of b
	// output:
	//   [0] fund script (2-of-2 multisig)
	//   [*] output of a (option)
	//   [*] output of b (option)
	tx := wire.NewMsgTx(2)
	for _, txin := range d.atxins {
		tx.AddTxIn(txin)
	}
	for _, txin := range d.btxins {
		tx.AddTxIn(txin)
	}
	script := d.FundScript()
	if script != nil {
		txout := wire.NewTxOut(d.famta+d.famtb+d.sfeea+d.sfeeb, P2WSHpkScript(script))
		tx.AddTxOut(txout)
		if d.txouta != nil {
			tx.AddTxOut(d.txouta)
		}
		if d.txoutb != nil {
			tx.AddTxOut(d.txoutb)
		}
	}
	return tx
}

// SettlementTx returns a settlement transaction by rate and A or B.
func (d *Dlc) SettlementTx(rate *Rate, isA bool) *wire.MsgTx {
	// settlement transaction
	// input:
	//   [0]:fund transaction output[0]
	// output:
	//   [0]:settlement script
	//   [1]:p2wpkh (option)
	var val1 int64
	var val2 int64
	var pub1 *btcec.PublicKey
	var pub2 *btcec.PublicKey
	if isA {
		val1 = rate.amta
		val2 = rate.amtb
		pub1 = d.puba
		pub2 = d.pubb
	} else {
		val1 = rate.amtb
		val2 = rate.amta
		pub1 = d.pubb
		pub2 = d.puba
	}
	if val1 <= 0 {
		return nil
	}
	tx := wire.NewMsgTx(2)
	txid := d.FundTx().TxHash()
	tx.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&txid, 0), nil, nil))
	pub := &btcec.PublicKey{}
	pub.X, pub.Y = btcec.S256().Add(rate.key.X, rate.key.Y, pub1.X, pub1.Y)
	pkScript := P2WSHpkScript(SettlementScript(pub, pub2))
	txout1 := wire.NewTxOut(val1, pkScript)
	tx.AddTxOut(txout1)
	if val2 > 0 {
		txout2 := wire.NewTxOut(val2, P2WPKHpkScript(pub2))
		tx.AddTxOut(txout2)
	}
	if d.isA != isA {
		rate.txid, _ = chainhash.NewHashFromStr(tx.TxHash().String())
	}
	return tx
}

// RefundTx returns a refund transaction.
func (d *Dlc) RefundTx() *wire.MsgTx {
	// refund transaction
	// input:
	//   [0]:fund transaction output[0]
	//       Sequence (0xfeffffff LE)
	// output:
	//   [0]:p2wpkh a
	//   [1]:p2wpkh b
	// locktime:
	//    Value decided by contract.
	tx := wire.NewMsgTx(2)
	txid := d.FundTx().TxHash()
	txin := wire.NewTxIn(wire.NewOutPoint(&txid, 0), nil, nil)
	txin.Sequence-- // max(0xffffffff-0x01)
	if d.rsigna != nil && d.rsignb != nil {
		tw := wire.TxWitness{}
		tw = append(tw, []byte{})
		tw = append(tw, d.rsigna)
		tw = append(tw, d.rsignb)
		tw = append(tw, d.FundScript())
		txin.Witness = tw
	}
	tx.AddTxIn(txin)
	tx.AddTxOut(wire.NewTxOut(d.famta, P2WPKHpkScript(d.puba)))
	tx.AddTxOut(wire.NewTxOut(d.famtb, P2WPKHpkScript(d.pubb)))
	tx.LockTime = d.locktime
	return tx
}

// Verify verifies signature for rate.
func (d *Dlc) Verify(rate *Rate, isA bool, sign []byte, pub *btcec.PublicKey) error {
	// verify settlement transaction
	// parse signature
	s, err := btcec.ParseDERSignature(sign, btcec.S256())
	if err != nil {
		return err
	}
	// settlement transaction
	tx := d.SettlementTx(rate, isA)
	// verify
	sighashes := txscript.NewTxSigHashes(tx)
	script := d.FundScript()
	amt := d.FundAmount() + d.SettlementFee()
	hash, err := txscript.CalcWitnessSigHash(script, sighashes, txscript.SigHashAll,
		tx, 0, amt)
	if err != nil {
		return err
	}
	verify := s.Verify(hash, pub)
	if !verify {
		return fmt.Errorf("verify fail : %v", verify)
	}
	// set signature for rate
	rate.rsign = sign
	return nil
}

// VerifyRefundTx verifies the refund transaction.
func (d *Dlc) VerifyRefundTx(sign []byte, pub *btcec.PublicKey) error {
	// parse signature
	s, err := btcec.ParseDERSignature(sign, btcec.S256())
	if err != nil {
		return err
	}
	// verify
	script := d.FundScript()
	if script == nil {
		return fmt.Errorf("not found fund script")
	}
	tx := d.RefundTx()
	sighashes := txscript.NewTxSigHashes(tx)
	amt := d.FundAmount() + d.SettlementFee()
	hash, err := txscript.CalcWitnessSigHash(script, sighashes, txscript.SigHashAll,
		tx, 0, amt)
	if err != nil {
		return err
	}
	verify := s.Verify(hash, pub)
	if !verify {
		return fmt.Errorf("verify fail : %v", verify)
	}
	return nil
}

// FixedRate returns fixed rate.
func (d *Dlc) FixedRate() *Rate {
	return d.game.GetFixedRate()
}

// SettlementToTx returns the transaction to send to pkScript.
func (d *Dlc) SettlementToTx(rate *Rate, isA bool, pkScript []byte, efee int64) (
	*wire.MsgTx, int64, []byte, error) {
	// send settlement transaction to pkScript
	// input:
	//   [0]:settlement transaction[0]
	// output:
	//   [0]:pkScript
	var val1 int64
	var pub1 *btcec.PublicKey
	var pub2 *btcec.PublicKey
	if isA {
		val1 = rate.amta
		pub1 = d.puba
		pub2 = d.pubb
	} else {
		val1 = rate.amtb
		pub1 = d.pubb
		pub2 = d.puba
	}
	// txid
	stx := d.SettlementTx(rate, isA)
	if stx == nil {
		return nil, -1, nil, fmt.Errorf("settlement transaction is nil")
	}
	txid := stx.TxHash()
	// fee
	fee := int64(216+len(pkScript)) * efee // 216 bytes + pkScript
	// txout value
	val := val1 - fee
	if val < 0 {
		return nil, -1, nil, fmt.Errorf("val is minus. val:%d, fee:%d", val, fee)
	}
	// transaction
	tx := wire.NewMsgTx(2)
	tx.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&txid, 0), nil, nil))
	txout := wire.NewTxOut(val, pkScript)
	tx.AddTxOut(txout)
	// script
	pub := &btcec.PublicKey{}
	pub.X, pub.Y = btcec.S256().Add(rate.key.X, rate.key.Y, pub1.X, pub1.Y)
	script := SettlementScript(pub, pub2)
	return tx, val1, script, nil
}

// P2WPKHpkScript creates P2WPKH pkScript
func P2WPKHpkScript(pub *btcec.PublicKey) []byte {
	// P2WPKH is OP_0 + HASH160(<public key>)
	builder := txscript.NewScriptBuilder()
	builder.AddOp(txscript.OP_0)
	builder.AddData(btcutil.Hash160(pub.SerializeCompressed()))
	pkScript, _ := builder.Script()
	return pkScript
}

// P2WSHpkScript creates P2WSH pkScript
func P2WSHpkScript(script []byte) []byte {
	// P2WSH is OP_0 + SHA256(script)
	builder := txscript.NewScriptBuilder()
	builder.AddOp(txscript.OP_0)
	builder.AddData(chainhash.HashB(script))
	pkScript, _ := builder.Script()
	return pkScript
}
