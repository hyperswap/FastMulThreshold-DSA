/*
 *  Copyright (C) 2018-2019  Fusion Foundation Ltd. All rights reserved.
 *  Copyright (C) 2018-2019  haijun.cai@anyswap.exchange
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

package smpc

import (
	"container/list"
	"encoding/hex"
	"fmt"
	"github.com/anyswap/Anyswap-MPCNode/internal/common"
	"strings"
)

//---------------------------------------------------------------

type NodeReply struct {
	Enode     string
	Status    string
	TimeStamp string
	Initiator string // "1"/"0"
}

type RpcType int32

const (
	Rpc_REQADDR RpcType = 0
	Rpc_SIGN    RpcType = 2
	Rpc_RESHARE RpcType = 3
)

func GetAllReplyFromGroup(wid int, gid string, rt RpcType, initiator string) []NodeReply {
	if gid == "" {
		return nil
	}

	_, enodes := GetGroup(gid)
	nodes := strings.Split(enodes, common.Sep2)

	if wid < 0 || wid >= len(workers) {
		var ars []NodeReply
		for _, node := range nodes {
			node2 := ParseNode(node)
			sta := "Pending"
			ts := ""
			in := "0"
			if strings.EqualFold(initiator, node2) {
				in = "1"
			}

			nr := NodeReply{Enode: node2, Status: sta, TimeStamp: ts, Initiator: in}
			ars = append(ars, nr)
		}

		return ars
	}

	w := workers[wid]
	if w == nil {
		return nil
	}

	var req SmpcReq
	switch rt {
	case Rpc_SIGN:
		req = &ReqSmpcSign{}
	case Rpc_RESHARE:
		req = &ReqSmpcReshare{}
	case Rpc_REQADDR:
		req = &ReqSmpcAddr{}
	default:
		fmt.Printf("Unsupported request type")
		return nil
	}

	return req.GetReplyFromGroup(wid, gid, initiator)
}

//---------------------------------------------------------------------------

func GetReqAddrKeyByOtherKey(key string, rt RpcType) string {
	if key == "" {
		return ""
	}

	var req SmpcReq
	if rt == Rpc_SIGN {
		req = &ReqSmpcSign{}
		return req.GetReqAddrKeyByKey(key)
	}

	return ""
}

//--------------------------------------------------------------------------------

func CheckAccept(pubkey string, mode string, account string) bool {
	if pubkey == "" || mode == "" || account == "" {
		return false
	}

	smpcpks, _ := hex.DecodeString(pubkey)
	exsit, da := GetPubKeyData(smpcpks[:])
	if exsit {
		pd, ok := da.(*PubKeyData)
		if ok {
			exsit, da2 := GetPubKeyData([]byte(pd.Key))
			if exsit {
				ac, ok := da2.(*AcceptReqAddrData)
				if ok {
					if ac != nil {
						if ac.Mode != mode {
							return false
						}
						if mode == "1" && strings.EqualFold(account, ac.Account) {
							return true
						}

						if mode == "0" && CheckAcc(cur_enode, account, ac.Sigs) {
							return true
						}
					}
				}
			}
		}
	}

	return false
}

//-----------------------------------------------------------------------------------

type AcceptReqAddrData struct {
	Initiator string //enode id
	Account   string
	Cointype  string
	GroupId   string
	Nonce     string
	LimitNum  string
	Mode      string
	TimeStamp string

	Deal   string
	Accept string

	Status string
	PubKey string
	Tip    string
	Error  string

	AllReply []NodeReply

	WorkId int

	Sigs string //5:enodeid1:account1:enodeid2:account2:enodeid3:account3:enodeid4:account4:enodeid5:account5
}

func SaveAcceptReqAddrData(ac *AcceptReqAddrData) error {
	if ac == nil {
		return fmt.Errorf("no accept data.")
	}

	key := Keccak256Hash([]byte(strings.ToLower(ac.Account + ":" + ac.Cointype + ":" + ac.GroupId + ":" + ac.Nonce + ":" + ac.LimitNum + ":" + ac.Mode))).Hex()

	alos, err := Encode2(ac)
	if err != nil {
		return err
	}

	ss, err := Compress([]byte(alos))
	if err != nil {
		return err
	}

	err = PutReqAddrInfoData([]byte(key), []byte(ss))
	if err != nil {
		common.Error("===============================SaveAcceptReqAddrData,put accept reqaddr data to db fail===========================", "key", key, "err", err)
		return err
	}

	return nil
}

//-------------------------------------------------------------------------------------

type TxDataAcceptReqAddr struct {
	TxType    string
	Key       string
	Accept    string
	TimeStamp string
}

func AcceptReqAddr(initiator string, account string, cointype string, groupid string, nonce string, threshold string, mode string, deal string, accept string, status string, pubkey string, tip string, errinfo string, allreply []NodeReply, workid int, sigs string) (string, error) {
	key := Keccak256Hash([]byte(strings.ToLower(account + ":" + cointype + ":" + groupid + ":" + nonce + ":" + threshold + ":" + mode))).Hex()
	exsit, da := GetPubKeyData([]byte(key))
	if exsit {
		ac, ok := da.(*AcceptReqAddrData)
		if ok {
			if ac.Status != "Pending" {
				common.Info("=====================AcceptReqAddr, the reqaddr has been processed =======================", "key", key)
				return "", nil
			}
		}
	}

	exsit, da = GetReqAddrInfoData([]byte(key))
	if !exsit {
		common.Info("=====================AcceptReqAddr, key does not exist =======================", "key", key)
		return "smpc back-end internal error:get accept data fail from db", fmt.Errorf("get reqaddr accept data fail from db")
	}

	ac, ok := da.(*AcceptReqAddrData)
	if !ok {
		return "smpc back-end internal error:get accept data fail from db", fmt.Errorf("get reqaddr accept data fail from db")
	}

	in := ac.Initiator
	if initiator != "" {
		in = initiator
	}

	de := ac.Deal
	if deal != "" {
		de = deal
	}

	acp := ac.Accept
	if accept != "" {
		acp = accept
	}

	pk := ac.PubKey
	if pubkey != "" {
		pk = pubkey
	}

	ttip := ac.Tip
	if tip != "" {
		ttip = tip
	}

	eif := ac.Error
	if errinfo != "" {
		eif = errinfo
	}

	sts := ac.Status
	if status != "" {
		sts = status
	}

	arl := ac.AllReply
	if allreply != nil {
		arl = allreply
	}

	wid := ac.WorkId
	if workid >= 0 {
		wid = workid
	}

	gs := ac.Sigs
	if sigs != "" {
		gs = sigs
	}

	ac2 := &AcceptReqAddrData{Initiator: in, Account: ac.Account, Cointype: ac.Cointype, GroupId: ac.GroupId, Nonce: ac.Nonce, LimitNum: ac.LimitNum, Mode: ac.Mode, TimeStamp: ac.TimeStamp, Deal: de, Accept: acp, Status: sts, PubKey: pk, Tip: ttip, Error: eif, AllReply: arl, WorkId: wid, Sigs: gs}

	e, err := Encode2(ac2)
	if err != nil {
		common.Debug("=====================AcceptReqAddr,encode fail=======================", "err", err, "key", key)
		return "smpc back-end internal error:encode reqaddr accept data fail", err
	}

	es, err := Compress([]byte(e))
	if err != nil {
		common.Debug("=====================AcceptReqAddr,compress fail=======================", "err", err, "key", key)
		return "smpc back-end internal error:compress reqaddr accept data fail", err
	}

	if ac2.Status != "Pending" {
		DeleteReqAddrInfoData([]byte(key))
		err = PutPubKeyData([]byte(key), []byte(es))
		if err != nil {
			common.Error("===================================AcceptReqAddr,put reqaddr accept data to pubkey data db fail===========================", "err", err, "key", key)
			return err.Error(), err
		}
	} else {
		err = PutReqAddrInfoData([]byte(key), []byte(es))
		if err != nil {
			common.Error("===================================AcceptReqAddr,put reqaddr accept data to pubkey data db fail===========================", "err", err, "key", key)
			return err.Error(), err
		}
	}

	return "", nil
}

//--------------------------------------------------------------------------------------

type AcceptSignData struct {
	Initiator  string //enode id
	Account    string
	GroupId    string
	Nonce      string
	PubKey     string
	MsgHash    []string
	MsgContext []string
	Keytype    string
	LimitNum   string
	Mode       string
	TimeStamp  string

	Deal   string
	Accept string

	Status string
	Rsv    string //rsv1:rsv2:....:rsvn:NULL
	Tip    string
	Error  string

	AllReply []NodeReply
	WorkId   int
}

func SaveAcceptSignData(ac *AcceptSignData) error {
	if ac == nil {
		return fmt.Errorf("no accept data.")
	}

	//key := hash(acc + nonce + pubkey + hash + keytype + groupid + threshold + mode)
	key := Keccak256Hash([]byte(strings.ToLower(ac.Account + ":" + ac.Nonce + ":" + ac.PubKey + ":" + get_sign_hash(ac.MsgHash, ac.Keytype) + ":" + ac.Keytype + ":" + ac.GroupId + ":" + ac.LimitNum + ":" + ac.Mode))).Hex()

	alos, err := Encode2(ac)
	if err != nil {
		common.Error("========================SaveAcceptSignData======================", "enode err", err, "key", key)
		return err
	}

	ss, err := Compress([]byte(alos))
	if err != nil {
		common.Error("========================SaveAcceptSignData======================", "compress err", err, "key", key)
		return err
	}

	err = PutSignInfoData([]byte(key), []byte(ss))
	if err != nil {
		common.Error("========================SaveAcceptSignData,put sign accept data to local db fail======================", "err", err, "key", key)
		return err
	}

	return nil
}

//-----------------------------------------------------------------------------------------------------

type TxDataAcceptSign struct {
	TxType     string
	Key        string
	MsgHash    []string
	MsgContext []string
	Accept     string
	TimeStamp  string
}

func AcceptSign(initiator string, account string, pubkey string, msghash []string, keytype string, groupid string, nonce string, threshold string, mode string, deal string, accept string, status string, rsv string, tip string, errinfo string, allreply []NodeReply, workid int) (string, error) {
	key := Keccak256Hash([]byte(strings.ToLower(account + ":" + nonce + ":" + pubkey + ":" + get_sign_hash(msghash, keytype) + ":" + keytype + ":" + groupid + ":" + threshold + ":" + mode))).Hex()

	exsit, da := GetPubKeyData([]byte(key))
	if exsit {
		ac, ok := da.(*AcceptSignData)
		if ok {
			if ac.Status != "Pending" || ac.Rsv != "" {
				common.Info("=====================AcceptSign,the sign has been processed=======================", "key", key)
				return "", nil
			}
		}
	}

	exsit, da = GetSignInfoData([]byte(key))
	if !exsit {
		common.Error("=====================AcceptSign, key does not exist=======================", "key", key)
		return "smpc back-end internal error:get accept data fail from db", fmt.Errorf("get sign accept data fail from db")
	}

	ac, ok := da.(*AcceptSignData)
	if !ok {
		return "smpc back-end internal error:get accept data fail from db", fmt.Errorf("get sign accept data fail from db")
	}

	in := ac.Initiator
	if initiator != "" {
		in = initiator
	}

	de := ac.Deal
	if deal != "" {
		de = deal
	}

	acp := ac.Accept
	if accept != "" {
		acp = accept
	}

	ah := ac.Rsv
	if rsv != "" {
		ah = rsv
	}

	ttip := ac.Tip
	if tip != "" {
		ttip = tip
	}

	eif := ac.Error
	if errinfo != "" {
		eif = errinfo
	}

	sts := ac.Status
	if status != "" {
		sts = status
	}

	arl := ac.AllReply
	if allreply != nil {
		arl = allreply
	}

	wid := ac.WorkId
	if workid >= 0 {
		wid = workid
	}

	ac2 := &AcceptSignData{Initiator: in, Account: ac.Account, GroupId: ac.GroupId, Nonce: ac.Nonce, PubKey: ac.PubKey, MsgHash: ac.MsgHash, MsgContext: ac.MsgContext, Keytype: ac.Keytype, LimitNum: ac.LimitNum, Mode: ac.Mode, TimeStamp: ac.TimeStamp, Deal: de, Accept: acp, Status: sts, Rsv: ah, Tip: ttip, Error: eif, AllReply: arl, WorkId: wid}

	e, err := Encode2(ac2)
	if err != nil {
		common.Error("=====================AcceptSign,encode fail=======================", "err", err, "key", key)
		return "smpc back-end internal error:encode accept data fail", err
	}

	es, err := Compress([]byte(e))
	if err != nil {
		common.Error("=====================AcceptSign,compress fail=======================", "err", err, "key", key)
		return "smpc back-end internal error:compress accept data fail", err
	}

	if ac2.Status != "Pending" {
		DeleteSignInfoData([]byte(key))
		err = PutPubKeyData([]byte(key), []byte(es))
		if err != nil {
			common.Error("========================AcceptSign,put sign accept data to pubkey data db fail.=======================", "key", key, "err", err)
			return err.Error(), err
		}
	} else {
		err = PutSignInfoData([]byte(key), []byte(es))
		if err != nil {
			common.Error("========================AcceptSign,put sign accept data to local db fail.=======================", "key", key, "err", err)
			return err.Error(), err
		}
	}

	return "", nil
}

//----------------------------------------------------------------------------------

type AcceptReShareData struct {
	Initiator  string //enode id
	Account    string
	GroupId    string
	TSGroupId  string
	PubKey     string
	LimitNum   string
	PubAccount string
	Mode       string
	Sigs       string
	TimeStamp  string

	Deal   string
	Accept string

	Status string
	NewSk  string
	Tip    string
	Error  string

	AllReply []NodeReply
	WorkId   int
}

func SaveAcceptReShareData(ac *AcceptReShareData) error {
	if ac == nil {
		return fmt.Errorf("Accept data was not found.")
	}

	key := Keccak256Hash([]byte(strings.ToLower(ac.Account + ":" + ac.GroupId + ":" + ac.TSGroupId + ":" + ac.PubKey + ":" + ac.LimitNum + ":" + ac.Mode))).Hex()

	alos, err := Encode2(ac)
	if err != nil {
		common.Error("========================SaveAcceptReShareData======================", "enode err", err, "key", key)
		return err
	}

	ss, err := Compress([]byte(alos))
	if err != nil {
		common.Error("========================SaveAcceptReShareData======================", "compress err", err, "key", key)
		return err
	}

	err = PutReShareInfoData([]byte(key), []byte(ss))
	if err != nil {
		common.Error("========================SaveAcceptReShareData======================", "put reshare accept data to local db err", err, "key", key)
		return err
	}

	return nil
}

//--------------------------------------------------------------------------------------

type TxDataAcceptReShare struct {
	TxType    string
	Key       string
	Accept    string
	TimeStamp string
}

func AcceptReShare(initiator string, account string, groupid string, tsgroupid string, pubkey string, threshold string, mode string, deal string, accept string, status string, newsk string, tip string, errinfo string, allreply []NodeReply, workid int) (string, error) {
	key := Keccak256Hash([]byte(strings.ToLower(account + ":" + groupid + ":" + tsgroupid + ":" + pubkey + ":" + threshold + ":" + mode))).Hex()
	exsit, da := GetPubKeyData([]byte(key))
	if exsit {
		ac, ok := da.(*AcceptReShareData)
		if ok {
			if ac.Status != "Pending" {
				common.Info("=====================AcceptReShare,the reshare has been processed=======================", "key", key)
				return "", nil
			}
		}
	}

	exsit, da = GetReShareInfoData([]byte(key))
	if !exsit {
		common.Error("=====================AcceptReShare, key does not exist======================", "key", key)
		return "smpc back-end internal error:get accept data fail from db", fmt.Errorf("get reshare accept data fail from db")
	}

	ac, ok := da.(*AcceptReShareData)
	if !ok {
		common.Error("=====================AcceptReShare, get reshare accept data fail from db======================", "key", key)
		return "smpc back-end internal error:get accept data fail from db", fmt.Errorf("get reshare accept data fail from db")
	}

	in := ac.Initiator
	if initiator != "" {
		in = initiator
	}

	de := ac.Deal
	if deal != "" {
		de = deal
	}

	acp := ac.Accept
	if accept != "" {
		acp = accept
	}

	ah := ac.NewSk
	if newsk != "" {
		ah = newsk
	}

	ttip := ac.Tip
	if tip != "" {
		ttip = tip
	}

	eif := ac.Error
	if errinfo != "" {
		eif = errinfo
	}

	sts := ac.Status
	if status != "" {
		sts = status
	}

	arl := ac.AllReply
	if allreply != nil {
		arl = allreply
	}

	wid := ac.WorkId
	if workid >= 0 {
		wid = workid
	}

	ac2 := &AcceptReShareData{Initiator: in, Account: ac.Account, GroupId: ac.GroupId, TSGroupId: ac.TSGroupId, PubKey: ac.PubKey, LimitNum: ac.LimitNum, PubAccount: ac.PubAccount, Mode: ac.Mode, Sigs: ac.Sigs, TimeStamp: ac.TimeStamp, Deal: de, Accept: acp, Status: sts, NewSk: ah, Tip: ttip, Error: eif, AllReply: arl, WorkId: wid}

	e, err := Encode2(ac2)
	if err != nil {
		common.Error("=====================AcceptReShare, encode fail======================", "err", err, "key", key)
		return "smpc back-end internal error:encode accept data fail", err
	}

	es, err := Compress([]byte(e))
	if err != nil {
		common.Error("=====================AcceptReShare, compress fail======================", "err", err, "key", key)
		return "smpc back-end internal error:compress accept data fail", err
	}

	if ac2.Status != "Pending" {
		DeleteReShareInfoData([]byte(key))
		err = PutPubKeyData([]byte(key), []byte(es))
		if err != nil {
			common.Error("=====================AcceptReShare, put reshare accept data to pubkey data db fail======================", "err", err, "key", key)
			return err.Error(), err
		}
	} else {
		err = PutReShareInfoData([]byte(key), []byte(es))
		if err != nil {
			common.Error("=====================AcceptReShare, put reshare accept data to local db fail======================", "err", err, "key", key)
			return err.Error(), err
		}
	}

	return "", nil
}

//---------------------------------------------------------------------

type RawReply struct {
	From      string
	Accept    string
	TimeStamp string
}

func GetRawReply(l *list.List) *common.SafeMap {
	ret := common.NewSafeMap(10)
	if l == nil {
		return ret
	}

	var next *list.Element
	for e := l.Front(); e != nil; e = next {
		next = e.Next()

		if e.Value == nil {
			continue
		}

		s := e.Value.(string)

		if s == "" {
			continue
		}

		raw := s
		keytmp, from, _, txdata, err := CheckRaw(raw)
		if err != nil {
			continue
		}

		var req2 SmpcReq
		req, ok := txdata.(*TxDataReqAddr)
		if ok {
			reply := &RawReply{From: from, Accept: "true", TimeStamp: req.TimeStamp}
			req2 = &ReqSmpcAddr{}
			req2.GetRawReply(ret, reply)

			continue
		}

		sig, ok := txdata.(*TxDataSign)
		if ok {
			common.Debug("=================GetRawReply,the list item is TxDataSign=================", "key", keytmp, "from", from, "sig", sig)
			reply := &RawReply{From: from, Accept: "true", TimeStamp: sig.TimeStamp}
			req2 = &ReqSmpcSign{}
			req2.GetRawReply(ret, reply)

			continue
		}

		rh, ok := txdata.(*TxDataReShare)
		if ok {
			reply := &RawReply{From: from, Accept: "true", TimeStamp: rh.TimeStamp}
			req2 = &ReqSmpcReshare{}
			req2.GetRawReply(ret, reply)

			continue
		}

		acceptreq, ok := txdata.(*TxDataAcceptReqAddr)
		if ok {
			accept := "false"
			if acceptreq.Accept == "AGREE" {
				accept = "true"
			}

			reply := &RawReply{From: from, Accept: accept, TimeStamp: acceptreq.TimeStamp}
			req2 = &ReqSmpcAddr{}
			req2.GetRawReply(ret, reply)
		}

		acceptsig, ok := txdata.(*TxDataAcceptSign)
		if ok {
			common.Debug("=================GetRawReply,the list item is TxDataAcceptSign================", "key", keytmp, "from", from, "accept", acceptsig.Accept, "raw", raw)
			accept := "false"
			if acceptsig.Accept == "AGREE" {
				accept = "true"
			}

			reply := &RawReply{From: from, Accept: accept, TimeStamp: acceptsig.TimeStamp}
			req2 = &ReqSmpcSign{}
			req2.GetRawReply(ret, reply)
		}

		acceptrh, ok := txdata.(*TxDataAcceptReShare)
		if ok {
			accept := "false"
			if acceptrh.Accept == "AGREE" {
				accept = "true"
			}

			reply := &RawReply{From: from, Accept: accept, TimeStamp: acceptrh.TimeStamp}
			req2 = &ReqSmpcReshare{}
			req2.GetRawReply(ret, reply)
		}
	}

	return ret
}

func CheckReply(l *list.List, rt RpcType, key string) bool {
	if l == nil || key == "" {
		return false
	}

	var req SmpcReq
	if rt == Rpc_RESHARE {
		req = &ReqSmpcReshare{}
		return req.CheckReply(nil, l, key)
	}

	k := ""
	if rt == Rpc_REQADDR {
		k = key
	} else {
		k = GetReqAddrKeyByOtherKey(key, rt)
	}

	if k == "" {
		return false
	}

	exsit, da := GetReqAddrInfoData([]byte(k))
	if !exsit || da == nil {
		exsit, da = GetPubKeyData([]byte(k))
	}

	if !exsit {
		return false
	}

	ac, ok := da.(*AcceptReqAddrData)
	if !ok {
		return false
	}

	if ac == nil {
		return false
	}

	switch rt {
	case Rpc_REQADDR:
		req = &ReqSmpcAddr{}
	case Rpc_SIGN:
		req = &ReqSmpcSign{}
	default:
		return false
	}

	return req.CheckReply(ac, l, key)
}
