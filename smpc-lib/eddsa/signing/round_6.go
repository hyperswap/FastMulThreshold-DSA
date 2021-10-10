package signing

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/anyswap/Anyswap-MPCNode/smpc-lib/smpc"
	//"github.com/anyswap/Anyswap-MPCNode/crypto/secp256k1"
	"crypto/sha512"
	"encoding/hex"
	"github.com/anyswap/Anyswap-MPCNode/smpc-lib/crypto/ed"
)

func (round *round6) Start() error {
	if round.started {
		fmt.Printf("============= ed sign,round6.start fail =======\n")
		return errors.New("ed sign,round already started")
	}
	round.number = 6
	round.started = true
	round.resetOK()

	cur_index, err := round.GetDNodeIDIndex(round.kgid)
	if err != nil {
		return err
	}

	var sB2, temSB ed.ExtendedGroupElement

	for k := range round.idsign {
		msg4, ok := round.temp.signRound4Messages[k].(*SignRound4Message)
		if !ok {
			return errors.New("get csb fail")
		}

		msg5, ok := round.temp.signRound5Messages[k].(*SignRound5Message)
		if !ok {
			return errors.New("get dsb fail")
		}

		CSBFlag := ed.Verify(msg4.CSB, msg5.DSB)
		if !CSBFlag {
			fmt.Printf("Error: Commitment(SB) Not Pass at User: %v", round.kgid)
			return errors.New("smpc back-end internal error:commitment(CSB) not pass")
		}

		var temSBBytes [32]byte
		copy(temSBBytes[:], msg5.DSB[32:])
		temSB.FromBytes(&temSBBytes)

		if k == 0 {
			sB2 = temSB
		} else {
			ed.GeAdd(&sB2, &sB2, &temSB)
		}
	}

	var k2 [32]byte
	var kDigest2 [64]byte

	h := sha512.New()
	_, err = h.Write(round.temp.FinalRBytes[:])
	if err != nil {
		return errors.New("smpc back-end internal error:write final r fail in caling k2")
	}

	_, err = h.Write(round.temp.pkfinal[:])
	if err != nil {
		return errors.New("smpc back-end internal error:write final pk fail in calling k2")
	}

	_, err = h.Write(([]byte(round.temp.message))[:])
	if err != nil {
		return errors.New("smpc back-end internal error:write message fail in caling k2")
	}

	h.Sum(kDigest2[:0])
	ed.ScReduce(&k2, &kDigest2)

	// 3.6 calculate sBCal
	var FinalR2, sBCal, FinalPkB ed.ExtendedGroupElement
	FinalR2.FromBytes(&round.temp.FinalRBytes)
	FinalPkB.FromBytes(&round.temp.pkfinal)
	ed.GeScalarMult(&sBCal, &k2, &FinalPkB)
	ed.GeAdd(&sBCal, &sBCal, &FinalR2)

	// 3.7 verify equation
	var sBBytes2, sBCalBytes [32]byte
	sB2.ToBytes(&sBBytes2)
	sBCal.ToBytes(&sBCalBytes)

	if !bytes.Equal(sBBytes2[:], sBCalBytes[:]) {
		fmt.Printf("Error: Not Pass Verification (SB = SBCal) at User: %v, message = %v,msg str = %v, pk = %v,RBytes = %v  \n", round.kgid, round.temp.message, hex.EncodeToString(round.temp.message[:]), round.temp.pkfinal[:], round.temp.FinalRBytes[:])
		return errors.New("Error: Not Pass Verification (SB = SBCal).")
	}

	srm := &SignRound6Message{
		SignRoundMessage: new(SignRoundMessage),
		S:                round.temp.s,
	}
	srm.SetFromID(round.kgid)
	srm.SetFromIndex(cur_index)

	round.temp.signRound6Messages[cur_index] = srm
	round.out <- srm

	fmt.Printf("============= ed sign,round6.start success, current node id = %v =============\n", round.kgid)

	return nil
}

func (round *round6) CanAccept(msg smpc.Message) bool {
	if _, ok := msg.(*SignRound6Message); ok {
		return msg.IsBroadcast()
	}
	return false
}

func (round *round6) Update() (bool, error) {
	for j, msg := range round.temp.signRound6Messages {
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

func (round *round6) NextRound() smpc.Round {
	//fmt.Printf("========= round.next round ========\n")
	round.started = false
	return &round7{round}
}
