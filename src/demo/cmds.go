// cmds.go
package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"

	"usr"
)

type cmd struct {
	n []string
	f func([]string, *Demo) error
}

func listCmds() []*cmd {
	list := []*cmd{}
	list = append(list, &cmd{[]string{"step", "s"}, step})
	list = append(list, &cmd{[]string{"set"}, set})
	list = append(list, &cmd{[]string{"generate", "g"}, generate})
	list = append(list, &cmd{[]string{"getrawtransaction", "grt"}, getrawtransaction})
	list = append(list, &cmd{[]string{"decodescript", "ds"}, decodescript})
	list = append(list, &cmd{[]string{"balance", "b"}, balance})
	list = append(list, &cmd{[]string{"fee"}, txfee})
	list = append(list, &cmd{[]string{"fauset"}, fauset})
	return list
}

func generate(args []string, d *Demo) error {
	var err error
	nblocks := 1
	if len(args) > 1 {
		nblocks, err = strconv.Atoi(args[1])
		if err != nil {
			return err
		}
	}
	if nblocks < 1 {
		return fmt.Errorf("nblocks is less than or equal to zero. %d", nblocks)
	}
	res, err := d.rpc.Request("generate", nblocks)
	if err != nil {
		return err
	}
	fmt.Printf("generate %d\n", nblocks)
	bs, err := json.Marshal(res.Result)
	if err != nil {
		return err
	}
	dump(bs)
	return nil
}

func getrawtransaction(args []string, d *Demo) error {
	if len(args) < 2 {
		return fmt.Errorf("illegal parameter")
	}
	txid := args[1]
	res, err := d.rpc.Request("getrawtransaction", txid, 1)
	if err != nil {
		return err
	}
	fmt.Printf("getrawtransaction %s 1\n", txid)
	bs, err := json.Marshal(res.Result)
	if err != nil {
		return err
	}
	dump(bs)
	return nil
}

func decodescript(args []string, d *Demo) error {
	if len(args) < 2 {
		return fmt.Errorf("illegal parameter")
	}
	hexstring := args[1]
	res, err := d.rpc.Request("decodescript", hexstring)
	if err != nil {
		return err
	}
	fmt.Printf("decodescript %s\n", hexstring)
	bs, err := json.Marshal(res.Result)
	if err != nil {
		return err
	}
	dump(bs)
	return nil
}

func balance(args []string, d *Demo) error {
	amta := d.alice.GetBalance()
	amtb := d.bob.GetBalance()
	fmt.Printf("alice amount : %.8f BTC\n", float64(amta)/btcutil.SatoshiPerBitcoin)
	fmt.Printf("bob   amount : %.8f BTC\n", float64(amtb)/btcutil.SatoshiPerBitcoin)
	return nil
}

func fauset(args []string, d *Demo) error {
	var err error
	satoshi := int(1 * btcutil.SatoshiPerBitcoin)
	if len(args) > 1 {
		satoshi, err = strconv.Atoi(args[1])
		if err != nil {
			return err
		}
	}
	if satoshi < 1 {
		return fmt.Errorf("satoshi is less than or equal to zero. %d", satoshi)
	}
	s := time.Now()
	fmt.Printf("begin fauset\n")
	_, err = d.rpc.Request("generate", 1)
	if err != nil {
		return err
	}
	lowest := int64(satoshi)
	users := []*usr.User{d.alice, d.bob}
	for _, user := range users {
		amt := user.GetBalance()
		if amt < lowest {
			_, err = d.rpc.Request("sendtoaddress", user.GetAddress(), float64(lowest-amt)/btcutil.SatoshiPerBitcoin)
			if err != nil {
				return err
			}
			_, err = d.rpc.Request("generate", 1)
			if err != nil {
				return err
			}
		}
	}
	balance(nil, d)
	fmt.Printf("end   fauset %f sec\n", (time.Now()).Sub(s).Seconds())
	return nil
}

func txfee(args []string, d *Demo) error {
	if len(args) < 2 {
		return fmt.Errorf("illegal parameter")
	}
	txid := args[1]
	res, err := d.rpc.Request("getrawtransaction", txid)
	if err != nil {
		return err
	}
	str, _ := res.Result.(string)
	bs, err := hex.DecodeString(str)
	if err != nil {
		return err
	}
	tx, err := bsToMsgTx(bs)
	if err != nil {
		return err
	}
	iamt := int64(0)
	for _, txin := range tx.TxIn {
		amt, err := getAmount(d, txin)
		if err != nil {
			return err
		}
		iamt += amt
	}
	oamt := int64(0)
	for _, txout := range tx.TxOut {
		oamt += txout.Value
	}
	fmt.Printf("input:%d output:%d fee:%d size:%d efee:%f\n",
		iamt, oamt, iamt-oamt, len(bs), float64(iamt-oamt)/float64(len(bs)))
	return nil
}

func getAmount(d *Demo, txin *wire.TxIn) (int64, error) {
	op := txin.PreviousOutPoint
	res, err := d.rpc.Request("getrawtransaction", op.Hash.String())
	if err != nil {
		return 0, err
	}
	str, _ := res.Result.(string)
	bs, err := hex.DecodeString(str)
	if err != nil {
		return 0, err
	}
	tx, err := bsToMsgTx(bs)
	if err != nil {
		return 0, err
	}
	if uint32(len(tx.TxOut)) <= op.Index {
		return 0, fmt.Errorf("out of range : %d,%d", len(tx.TxOut), op.Index)
	}
	txout := tx.TxOut[op.Index]
	return txout.Value, nil
}

func bsToMsgTx(bs []byte) (*wire.MsgTx, error) {
	var tx *wire.MsgTx
	tx = &wire.MsgTx{}
	buf := &bytes.Buffer{}
	_, err := buf.Write(bs)
	if err != nil {
		return nil, err
	}
	err = tx.Deserialize(buf)
	if err != nil {
		tx = &wire.MsgTx{}
		buf := &bytes.Buffer{}
		_, err := buf.Write(bs)
		if err != nil {
			return nil, err
		}
		err = tx.DeserializeNoWitness(buf)
		if err != nil {
			return nil, err
		}
	}
	return tx, nil
}

func dump(bs []byte) {
	var buf bytes.Buffer
	err := json.Indent(&buf, bs, "", "  ")
	if err != nil {
		fmt.Printf("dump Error : %+v\n", err)
		return
	}
	fmt.Printf("%s\n", buf.String())
}
