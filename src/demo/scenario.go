// scenario.go
package main

import (
	"fmt"
	"math"
	"strconv"

	"github.com/btcsuite/btcutil"

	"dlc"
)

type scenario struct {
	memo   string
	dlc    *dlc.Dlc
	steps  []func(int, *Demo) error
	pos    int
	sendAB bool
}

func (s *scenario) step(d *Demo) error {
	if s.pos < 0 || len(s.steps) <= s.pos {
		fmt.Printf("This scenario is over.\n")
		return nil
	}
	if s.pos == 0 {
		fmt.Printf("This scenario start.\n")
	}
	err := s.steps[s.pos](s.pos+1, d)
	if err != nil {
		return err
	}
	s.pos++
	if len(s.steps) == s.pos {
		fmt.Printf("This scenario finish.\n")
	}
	return nil
}

func set(args []string, d *Demo) error {
	var err error
	idx := 0
	if len(args) > 1 {
		idx, err = strconv.Atoi(args[1])
		if err != nil {
			return err
		}
	}
	list := []func(*Demo) (*scenario, error){}
	list = append(list, scenario0)
	list = append(list, scenario1)
	list = append(list, scenario2)
	if idx < 0 || len(list) <= idx {
		return fmt.Errorf("out of range. %d,%d", idx, len(list))
	}
	err = faucet(nil, d)
	if err != nil {
		return err
	}
	d.sc, err = list[idx](d)
	if err != nil {
		return err
	}
	d.alice.ClearDlc()
	d.bob.ClearDlc()
	fmt.Printf("set the scenario.\n")
	fmt.Printf("%s\n", d.sc.memo)
	return nil
}

func step(args []string, d *Demo) error {
	if d.sc == nil {
		return fmt.Errorf("scenario is nil")
	}
	return d.sc.step(d)
}

//----------------------------------------------------------------

func scenario0(d *Demo) (*scenario, error) {
	sc := &scenario{}
	sc.memo = "Alice bet high and it ends normally."
	sc.sendAB = true
	res, err := d.rpc.Request("getblockcount")
	if err != nil {
		return nil, err
	}
	height, _ := res.Result.(float64)
	sc.dlc, err = makeDlc(true, int(height+10), 1)
	if err != nil {
		return nil, err
	}
	sc.steps = append(sc.steps, stepAliceSendOfferToBob)
	sc.steps = append(sc.steps, stepBobSendAcceptToAlice)
	sc.steps = append(sc.steps, stepAliceSendSignToBob)
	sc.steps = append(sc.steps, stepAliceAndBobSetOracleSign)
	sc.steps = append(sc.steps, stepAliceOrBobSendSettlementTx)
	return sc, nil
}

func scenario1(d *Demo) (*scenario, error) {
	sc := &scenario{}
	sc.memo = "Alice bet low and it ends normally."
	sc.sendAB = false
	res, err := d.rpc.Request("getblockcount")
	if err != nil {
		return nil, err
	}
	height, _ := res.Result.(float64)
	sc.dlc, err = makeDlc(false, int(height+10), 1)
	if err != nil {
		return nil, err
	}
	sc.steps = append(sc.steps, stepAliceSendOfferToBob)
	sc.steps = append(sc.steps, stepBobSendAcceptToAlice)
	sc.steps = append(sc.steps, stepAliceSendSignToBob)
	sc.steps = append(sc.steps, stepAliceAndBobSetOracleSign)
	sc.steps = append(sc.steps, stepAliceOrBobSendSettlementTx)
	return sc, nil
}

func scenario2(d *Demo) (*scenario, error) {
	sc := &scenario{}
	sc.memo = "Since there is no Oracle, send a refund transaction."
	sc.sendAB = true
	res, err := d.rpc.Request("getblockcount")
	if err != nil {
		return nil, err
	}
	height, _ := res.Result.(float64)
	sc.dlc, err = makeDlc(true, int(height+10), 1)
	if err != nil {
		return nil, err
	}
	sc.steps = append(sc.steps, stepAliceSendOfferToBob)
	sc.steps = append(sc.steps, stepBobSendAcceptToAlice)
	sc.steps = append(sc.steps, stepAliceSendSignToBob)
	sc.steps = append(sc.steps, stepAliceOrBobSendRefundTx)
	return sc, nil
}

//----------------------------------------------------------------

func makeDlc(high bool, count int, length int) (*dlc.Dlc, error) {
	amount := int64(1 * btcutil.SatoshiPerBitcoin)
	fefee := int64(10)                      // fund transaction estimate fee satoshi/byte
	sefee := int64(10)                      // settlement transaction estimate fee satoshi/byte
	sfee := dlc.DlcSettlementTxSize * sefee // settlement transaction size is 345 bytes
	d, err := dlc.NewDlc(half(amount), half(amount), fefee,
		sefee, half(sfee), half(sfee), high)
	if err != nil {
		return nil, err
	}
	d.SetGameConditions(count, length)
	return d, nil
}

func half(value int64) int64 {
	return int64(math.Ceil(float64(value) / float64(2)))
}
