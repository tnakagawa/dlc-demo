// Package dlc project game.go
package dlc

import (
	"fmt"
	"math/big"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/btcsuite/btcd/btcec"

	"oracle"
)

// SetGameConditions sets the contidions of game.
func (d *Dlc) SetGameConditions(date time.Time, length int) {
	d.date = date
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
	rates = append(rates, NewRate([][]byte{big.NewInt(10).Bytes()}, 0, amount))
	rates = append(rates, NewRate([][]byte{big.NewInt(20).Bytes()}, amount/4, (amount/4)*3))
	rates = append(rates, NewRate([][]byte{big.NewInt(30).Bytes()}, amount/2, amount/2))
	rates = append(rates, NewRate([][]byte{big.NewInt(40).Bytes()}, (amount/4)*3, amount/4))
	rates = append(rates, NewRate([][]byte{big.NewInt(50).Bytes()}, amount, 0))
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
func (d *Dlc) SetOracleSigns(value string, signs []*big.Int) error {
	msgs := [][]byte{}
	vals := strings.Split(value, ",")
	for _, val := range vals {
		i, err := strconv.Atoi(val)
		if err != nil {
			return err
		}
		msgs = append(msgs, big.NewInt(int64(i)).Bytes())
	}
	if len(msgs) != len(signs) {
		return fmt.Errorf("illegal parameters %v,%x", value, signs)
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
	d.value = value
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

// GameDate returns the date
func (d *Dlc) GameDate() time.Time {
	return d.date
}

// GameLength returns the length.
func (d *Dlc) GameLength() int {
	return d.length
}
