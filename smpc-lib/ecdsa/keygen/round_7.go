package keygen

import (
	"errors"
	"fmt"
	"github.com/anyswap/Anyswap-MPCNode/smpc-lib/smpc"
)

func (round *round7) Start() error {
	if round.started {
		return errors.New("round already started")
	}
	round.number = 7
	round.started = true
	round.resetOK()

	round.end <- *round.Save

	fmt.Printf("========= round7 start success ==========\n")
	return nil
}

func (round *round7) CanAccept(msg smpc.Message) bool {
	return false
}

func (round *round7) Update() (bool, error) {
	return false, nil
}

func (round *round7) NextRound() smpc.Round {
	return nil
}
