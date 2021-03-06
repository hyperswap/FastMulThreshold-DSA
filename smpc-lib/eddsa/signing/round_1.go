/*
 *  Copyright (C) 2020-2021  AnySwap Ltd. All rights reserved.
 *  Copyright (C) 2020-2021  haijun.cai@anyswap.exchange
 *
 *  This library is free software; you can redistribute it and/or
 *  modify it under the Apache License, Version 2.0.
 *
 *  This library is distributed in the hope that it will be useful,
 *  but WITHOUT ANY WARRANTY; without even the implied warranty of
 *  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.
 *
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package signing

import (
	"errors"
	"fmt"
	"github.com/anyswap/FastMulThreshold-DSA/smpc-lib/eddsa/keygen"
	"github.com/anyswap/FastMulThreshold-DSA/smpc-lib/smpc"
	"math/big"
	//"encoding/hex"
	"github.com/anyswap/FastMulThreshold-DSA/smpc-lib/crypto/ed"
	cryptorand "crypto/rand"
	"io"
)

var (
	zero = big.NewInt(0)
)

func newRound1(temp *localTempData, save *keygen.LocalDNodeSaveData, idsign smpc.SortableIDSSlice, out chan<- smpc.Message, end chan<- EdSignData, kgid string, threshold int, paillierkeylength int, txhash *big.Int) smpc.Round {
	finalizeendCh := make(chan *big.Int, threshold)
	return &round1{
		&base{temp, save, idsign, out, end, make([]bool, threshold), false, 0, kgid, threshold, paillierkeylength, nil, txhash, finalizeendCh}}
}

// Start get sk pkfinal R
func (round *round1) Start() error {
	if round.started {
		fmt.Printf("============= ed sign,round1.start fail =======\n")
		return errors.New("ed sign,round1 already started")
	}
	round.number = 1
	round.started = true
	round.ResetOK()

	curIndex, err := round.GetDNodeIDIndex(round.kgid)
	if err != nil {
		return err
	}

	var sk [32]byte
	copy(sk[:], round.save.Sk[:32])
	var tsk [32]byte
	copy(tsk[:], round.save.TSk[:32])
	var pkfinal [32]byte
	copy(pkfinal[:], round.save.FinalPkBytes[:32])

	var uids [][32]byte
	for _, v := range round.save.IDs {
		var tem [32]byte
		tmp := v.Bytes()
		copy(tem[:], tmp[:])
		if len(v.Bytes()) < 32 {
			l := len(v.Bytes())
			for j := l; j < 32; j++ {
				tem[j] = byte(0x00)
			}
		}
		uids = append(uids, tem)
	}
	round.temp.uids = uids

	round.temp.sk = sk
	round.temp.tsk = tsk
	round.temp.pkfinal = pkfinal

	if round.txhash == nil {
		return errors.New("no unsign hash")
	}

	//tmpstr := hex.EncodeToString(round.txhash.Bytes())
	//round.temp.message, _ = hex.DecodeString(tmpstr)
	round.temp.message = round.txhash.Bytes() 

	// [Notes]
	// 1. calculate R
	var r [32]byte
	var rTem [64]byte
	var RBytes [32]byte

	rand := cryptorand.Reader
	if _, err := io.ReadFull(rand, r[:]); err != nil {
		fmt.Println("Error: io.ReadFull(rand, r)")
		return err 
	}
	copy(rTem[:], r[:])
	ed.ScReduce(&r, &rTem)

	var R ed.ExtendedGroupElement
	ed.GeScalarMultBase(&R, &r)

	// 2. commit(R)
	R.ToBytes(&RBytes)
	CR, DR,err := ed.Commit(RBytes)
	if err != nil {
	    return err
	}

	// 3. zkSchnorr(rU1)
	//zkR,err := ed.Prove(r)
	zkR,err := ed.Prove2(r,RBytes)
	if err != nil {
	    return err
	}

	round.temp.DR = DR
	round.temp.zkR = zkR
	round.temp.r = r

	srm := &SignRound1Message{
		SignRoundMessage: new(SignRoundMessage),
		CR:               CR,
	}
	srm.SetFromID(round.kgid)
	srm.SetFromIndex(curIndex)

	round.temp.signRound1Messages[curIndex] = srm
	round.out <- srm

	//fmt.Printf("============= ed sign,round1.start success, current node id = %v =======\n", round.kgid)
	return nil
}

// CanAccept is it legal to receive this message 
func (round *round1) CanAccept(msg smpc.Message) bool {
	if _, ok := msg.(*SignRound1Message); ok {
		return msg.IsBroadcast()
	}

	return false
}

// Update  is the message received and ready for the next round? 
func (round *round1) Update() (bool, error) {
	for j, msg := range round.temp.signRound1Messages {
		if round.ok[j] {
			continue
		}
		if msg == nil || !round.CanAccept(msg) {
			return false, nil
		}
		round.ok[j] = true
	}

	return true, nil
}

// NextRound enter next round
func (round *round1) NextRound() smpc.Round {
	round.started = false
	return &round2{round}
}
