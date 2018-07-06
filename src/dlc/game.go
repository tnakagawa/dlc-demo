// Package dlc project game.go
package dlc

import (
	"fmt"
	"math"
	"math/big"

	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/chaincfg/chainhash"

	"oracle"
)

// Game is the game dataset.
type Game struct {
	// Common parameters
	pubo   *btcec.PublicKey   // Oracle public key
	okeys  []*btcec.PublicKey // Oracle contract keys
	omsgs  [][]byte           // Oracle contract Fixed messages
	osigns []*big.Int         // Oracle contract Fixed signs
	rates  []*Rate            // Rate list
	frate  *Rate              // Fixed rate
	dlc    *Dlc               // Dlc
	// Original parameters
	height int             // Block height
	length int             // Target length
	hash   *chainhash.Hash // Block hash
}

// NewGame returns a new Game.
func NewGame(dlc *Dlc, height, length int) *Game {
	game := &Game{}
	game.dlc = dlc
	game.height = height
	game.length = length
	return game
}

// Rates returns rate array.
func (g *Game) Rates() []*Rate {
	// cache check
	if g.rates != nil {
		return g.rates
	}
	// original calc
	rates := []*Rate{}
	amount := g.dlc.FundAmount()
	// quarter
	q := int64(math.Pow(float64(0x100), float64(g.length))) / 4
	// The first quarter is won low and all will be paid low.
	for x := int64(0); x < q; x++ {
		msgs := make([][]byte, g.length)
		for i := range msgs {
			if i == g.length-1 {
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
		msgs := make([][]byte, g.length)
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
		msgs := make([][]byte, g.length)
		for i := range msgs {
			if i == g.length-1 {
				msgs[i] = []byte{byte(x)}
				continue
			}
			msgs[i] = nil
		}
		rate := NewRate(msgs, amount, 0)
		rates = append(rates, rate)
	}
	// set cache
	g.rates = rates
	return g.rates
}

// SetOracleKeys sets the public key of oracle and the public keys of the message to the rate.
func (g *Game) SetOracleKeys(pub *btcec.PublicKey, keys []*btcec.PublicKey) {
	rates := g.Rates()
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
	g.pubo = pub
	g.okeys = keys
}

// SetOracleSigns sets oracle's signatures to rate and sets a fixed rate.
func (g *Game) SetOracleSigns(msgs [][]byte, signs []*big.Int) error {
	// search fixed rate
	rate := g.searchRate(msgs)
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
	g.frate = rate
	g.omsgs = msgs
	g.osigns = signs
	return nil
}

// GetFixedRate returns a fixed rate.
func (g *Game) GetFixedRate() *Rate {
	return g.frate
}

func (g *Game) searchRate(msgs [][]byte) *Rate {
	var rate *Rate
	rates := g.Rates()
	for _, r := range rates {
		for i := g.length - 1; i >= 0; i-- {
			if r.msgs[i] == nil || g.bseq(r.msgs[i], msgs[i]) {
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

func (g *Game) bseq(bs1, bs2 []byte) bool {
	if len(bs1) != len(bs2) {
		return false
	}
	for i := range bs1 {
		if bs1[i] != bs2[i] {
			return false
		}
	}
	return true
}

// original function

// GameHeight returns the block height.
func (g *Game) GameHeight() int {
	return g.height
}

// GameLength returns the length.
func (g *Game) GameLength() int {
	return g.length
}

// SetHash sets a block hash.
func (g *Game) SetHash(hash *chainhash.Hash) {
	g.hash = hash
}
