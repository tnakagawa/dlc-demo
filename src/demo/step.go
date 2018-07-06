// step.go
package main

import (
	"fmt"
	"time"

	"usr"
)

func stepAliceSendOfferToBob(num int, d *Demo) error {
	s := time.Now()
	fmt.Printf("begin step%d\n", num)
	fmt.Printf("step%d : Alice GetOfferData\n", num)
	odata, err := d.alice.GetOfferData(d.sc.dlc)
	if err != nil {
		return err
	}
	fmt.Printf("step%d : Alice SetOracleKeys\n", num)
	keys, err := d.olivia.Keys(d.alice.GameHeight())
	if err != nil {
		return err
	}
	err = d.alice.SetOracleKeys(keys)
	if err != nil {
		return err
	}
	fmt.Printf("step%d : Alice -> Bob\n", num)
	dump(odata)
	fmt.Printf("step%d : Bob SetOfferData\n", num)
	err = d.bob.SetOfferData(odata)
	if err != nil {
		return err
	}
	fmt.Printf("end   step%d %f sec\n", num, (time.Now()).Sub(s).Seconds())
	return nil
}

func stepBobSendAcceptToAlice(num int, d *Demo) error {
	s := time.Now()
	fmt.Printf("begin step%d\n", num)
	fmt.Printf("step%d : Bob SetOracleKeys\n", num)
	keys, err := d.olivia.Keys(d.bob.GameHeight())
	if err != nil {
		return err
	}
	err = d.bob.SetOracleKeys(keys)
	if err != nil {
		return err
	}
	fmt.Printf("step%d: Bob GetAcceptData\n", num)
	adata, err := d.bob.GetAcceptData()
	if err != nil {
		return err
	}
	fmt.Printf("step%d : Bob -> Alice\n", num)
	dump(adata)
	fmt.Printf("step%d : Alice SetAcceptData\n", num)
	err = d.alice.SetAcceptData(adata)
	if err != nil {
		return err
	}
	fmt.Printf("end   step%d %f sec\n", num, (time.Now()).Sub(s).Seconds())
	return nil
}

func stepAliceSendSignToBob(num int, d *Demo) error {
	s := time.Now()
	fmt.Printf("begin step%d\n", num)
	fmt.Printf("step%d : Alice GetSignData\n", num)
	sdata, err := d.alice.GetSignData()
	if err != nil {
		return err
	}
	fmt.Printf("step%d : Alice -> Bob\n", num)
	dump(sdata)
	fmt.Printf("step%d : Bob SetSignData\n", num)
	err = d.bob.SetSignData(sdata)
	if err != nil {
		return err
	}
	err = d.bob.SendFundTx()
	if err != nil {
		return err
	}
	fmt.Printf("end   step%d %f sec\n", num, (time.Now()).Sub(s).Seconds())
	return nil
}

func stepAliceAndBobSetOracleSign(num int, d *Demo) error {
	s := time.Now()
	fmt.Printf("begin step%d\n", num)
	height := d.alice.GameHeight()
	sigs, err := d.olivia.Signs(height)
	if err != nil {
		return err
	}
	fmt.Printf("step%d : Alice & Bob SetOracleSigns\n", num)
	err = d.alice.SetOracleSigns(sigs)
	if err != nil {
		return err
	}
	height = d.bob.GameHeight()
	sigs, err = d.olivia.Signs(height)
	if err != nil {
		return err
	}
	err = d.bob.SetOracleSigns(sigs)
	if err != nil {
		return err
	}
	fmt.Printf("end   step%d %f sec\n", num, (time.Now()).Sub(s).Seconds())
	return nil
}

func stepAliceOrBobSendSettlementTx(num int, demo *Demo) error {
	s := time.Now()
	fmt.Printf("begin step%d\n", num)
	users := []*usr.User{demo.alice, demo.bob}
	//	if !demo.sc.SendAB {
	//		users = []*usr.User{demo.bob, demo.alice}
	//	}
	for _, user := range users {
		err := user.SendSettlementTx()
		if err != nil {
			fmt.Printf("SendSettlementTx error : %+v\n", err)
			continue
		}
		err = user.SendSettlementTxTo(int64(10))
		if err != nil {
			return err
		}
		break
	}
	fmt.Printf("end   step%d %f sec\n", num, (time.Now()).Sub(s).Seconds())
	return nil
}

func stepAliceOrBobSendRefundTx(num int, demo *Demo) error {
	s := time.Now()
	fmt.Printf("begin step%d\n", num)
	err := demo.alice.SendRefundTx()
	if err != nil {
		return err
	}
	fmt.Printf("end   step%d %f sec\n", num, (time.Now()).Sub(s).Seconds())
	return nil
}
