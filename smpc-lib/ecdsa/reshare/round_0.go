package reshare 

import (
	"errors"
	"fmt"
	"math/big"
	"github.com/anyswap/Anyswap-MPCNode/smpc-lib/smpc"
	"github.com/anyswap/Anyswap-MPCNode/smpc-lib/ecdsa/keygen"
)

var (
	zero = big.NewInt(0)
)

func newRound0(save *keygen.LocalDNodeSaveData, temp *localTempData,out chan<- smpc.Message, end chan<- keygen.LocalDNodeSaveData,dnodeid string,dnodecount int,threshold int,paillierkeylength int,oldnode bool) smpc.Round {
    return &round0{
		&base{save,temp,out,end,make([]bool,dnodecount),false,0,dnodeid,dnodecount,threshold,paillierkeylength,oldnode,nil}}
}

func (round *round0) Start() error {
	if round.started {
	    fmt.Printf("============= round0.start fail =======\n")
	    return errors.New("round already started")
	}
	round.number = 0
	round.started = true
	round.resetOK()

	re := &ReshareRound0Message{
	    ReshareRoundMessage: new(ReshareRoundMessage),
	}
	re.SetFromID(round.dnodeid)
	re.SetFromIndex(-1)

	round.temp.reshareRound0Messages = append(round.temp.reshareRound0Messages,re)
	round.out <- re
	fmt.Printf("============= round0.start success, current node id = %v =======\n",round.dnodeid)
	return nil
}

func (round *round0) CanAccept(msg smpc.Message) bool {
	if _, ok := msg.(*ReshareRound0Message); ok {
		return msg.IsBroadcast()
	}
	return false
}

func (round *round0) Update() (bool, error) {
	for j, msg := range round.temp.reshareRound0Messages {
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

func (round *round0) NextRound() smpc.Round {
    //fmt.Printf("========= round.next round ========\n")
    round.started = false
    return &round1{round}
}
