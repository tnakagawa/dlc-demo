// Package dlc project game.go
package dlc

import (
	"fmt"
	"math"
	"math/big"
	"reflect"

	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/chaincfg/chainhash"

	"oracle"
)

// SetGameConditions sets the contidions of game.
func (d *Dlc) SetGameConditions(height, length int) {
	d.height = height
	d.length = length
	d.locktime = uint32(d.length + 144)
}

// Rates returns rate array.
func (d *Dlc) Rates() []*Rate {
	// cache check
	if d.rates != nil {
		return d.rates
	}
	// original calc
	rates := []*Rate{}
	amount := d.FundAmount()
	// quarter
	q := int64(math.Pow(float64(0x100), float64(d.length))) / 4
	// The first quarter is won low and all will be paid low.
	for x := int64(0); x < q; x++ {
		msgs := make([][]byte, d.length)
		for i := range msgs {
			if i == d.length-1 {
				msgs[i] = []byte{byte(x)}
				continue
			}
			msgs[i] = nil
		}
		rate := NewRate(msgs, 0, amount)
		rates = append(rates, rate)
	}
	// The second and third quarters are paid linearly.
	// high value is y, quarter is q, amount is A.
	// y = a*x - b
	// 0 = a*(q-1) - b
	// A = a*(3*q) - b
	// a = A/(2*q+1)
	// b = a*(q-1)
	a := float64(amount) / float64(2*q+1)
	b := float64(a * float64(q-1))
	for x := q; x < 3*q; x++ {
		msgs := make([][]byte, d.length)
		tmp := x
		for i := range msgs {
			msgs[i] = []byte{byte(tmp % 0x100)}
			tmp = (tmp - tmp%0x100) / 0x100
		}
		high := int64(math.Round(a*float64(x) - b))
		low := amount - high
		rate := NewRate(msgs, high, low)
		rates = append(rates, rate)
	}
	// The last quarter is won high and all will be paid high.
	for x := q * 3; x < q*4; x++ {
		msgs := make([][]byte, d.length)
		for i := range msgs {
			if i == d.length-1 {
				msgs[i] = []byte{byte(x)}
				continue
			}
			msgs[i] = nil
		}
		rate := NewRate(msgs, amount, 0)
		rates = append(rates, rate)
	}
	// set cache
	d.rates = rates
	return d.rates
}

// SetOracleKeys sets the public key of oracle and the public keys of the message to the rate.
func (d *Dlc) SetOracleKeys(pub *btcec.PublicKey, keys []*btcec.PublicKey) {
	rates := d.Rates()
	for _, r := range rates {
		key := new(btcec.PublicKey)
		for idx, m := range r.msgs {
			if len(m) == 0 {
				continue
			}
			// R is contract key,O is oracle public key.
			// R - H(R,m)O
			p := oracle.Commit(keys[idx], pub, m)
			// If there are multiple messages, concatenate public keys.
			if key.X == nil {
				key.X, key.Y = p.X, p.Y
			} else {
				key.X, key.Y = btcec.S256().Add(key.X, key.Y, p.X, p.Y)
			}
		}
		r.key = key
	}
	d.pubo = pub
	d.okeys = keys
}

// SetOracleSigns sets oracle's signatures to rate and sets a fixed rate.
func (d *Dlc) SetOracleSigns(hash *chainhash.Hash, signs []*big.Int) error {
	msgs := [][]byte{}
	for i := 0; i < chainhash.HashSize; i++ {
		msgs = append(msgs, []byte{hash[i]})
	}
	if len(msgs) != len(signs) {
		return fmt.Errorf("illegal parameters %v,%x", hash, signs)
	}
	// search fixed rate
	rate := d.searchRate(msgs)
	if rate == nil {
		return fmt.Errorf("rate not found")
	}
	// calc signature
	sign := big.NewInt(0)
	for i, m := range rate.msgs {
		if len(m) > 0 {
			sign = new(big.Int).Mod(new(big.Int).Add(sign, signs[i]), btcec.S256().N)
		}
	}
	// check signature
	sG := new(btcec.PublicKey)
	sG.X, sG.Y = btcec.S256().ScalarBaseMult(sign.Bytes())
	if !rate.key.IsEqual(sG) {
		return fmt.Errorf("illegal oracle sings")
	}
	rate.msign = sign
	d.frate = rate
	d.omsgs = msgs
	d.osigns = signs
	d.hash = hash
	return nil
}

// FixedRate returns a fixed rate.
func (d *Dlc) FixedRate() *Rate {
	return d.frate
}

func (d *Dlc) searchRate(msgs [][]byte) *Rate {
	var rate *Rate
	rates := d.Rates()
	for _, r := range rates {
		for i := d.length - 1; i >= 0; i-- {
			if r.msgs[i] == nil || reflect.DeepEqual(r.msgs[i], msgs[i]) {
				if i == 0 {
					rate = r
				}
				continue
			}
			break
		}
		if rate != nil {
			break
		}
	}
	return rate
}

// original function

// GameHeight returns the block height.
func (d *Dlc) GameHeight() int {
	return d.height
}

// GameLength returns the length.
func (d *Dlc) GameLength() int {
	return d.length
}

// SetHash sets a block hash.
func (d *Dlc) SetHash(hash *chainhash.Hash) {
	d.hash = hash
}
