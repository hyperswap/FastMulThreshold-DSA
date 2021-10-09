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
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"time"
	"encoding/json"
	"sync"

	"sort"
	"github.com/anyswap/Anyswap-MPCNode/smpc-lib/ecdsa/keygen"
	"github.com/anyswap/Anyswap-MPCNode/smpc-lib/ecdsa/reshare"
	smpclib "github.com/anyswap/Anyswap-MPCNode/smpc-lib/smpc"
	"github.com/anyswap/Anyswap-MPCNode/crypto/secp256k1"
	"github.com/anyswap/Anyswap-MPCNode/internal/common"
	"github.com/anyswap/Anyswap-MPCNode/internal/common/math/random"
	"github.com/fsn-dev/cryptoCoins/coins"
	"github.com/anyswap/Anyswap-MPCNode/smpc-lib/crypto/ec2"
)

//----------------------------------------------------------------------------------------

func GetReShareNonce(account string) (string, string, error) {
	key := Keccak256Hash([]byte(strings.ToLower(account + ":" + "RESHARE"))).Hex()
	exsit,da := GetPubKeyData([]byte(key))
	if !exsit {
		return "0", "", nil
	}

	nonce, _ := new(big.Int).SetString(string(da.([]byte)), 10)
	one, _ := new(big.Int).SetString("1", 10)
	nonce = new(big.Int).Add(nonce, one)
	return fmt.Sprintf("%v", nonce), "", nil
}

func SetReShareNonce(account string,nonce string) (string, error) {
	key2 := Keccak256Hash([]byte(strings.ToLower(account + ":" + "RESHARE"))).Hex()
	err := PutPubKeyData([]byte(key2),[]byte(nonce))
	if err != nil {
	    return err.Error(),err
	}

	return "", nil
}

//--------------------------------------------------------------------------------

func IsValidReShareAccept(from string,gid string) bool {
    if from == "" || gid == "" {
	return false
    }

    h := coins.NewCryptocoinHandler("FSN")
    if h == nil {
	return false
    }
    
    _, enodes := GetGroup(gid)
    nodes := strings.Split(enodes, common.Sep2)
    for _, node := range nodes {
	node2 := ParseNode(node)
	pk := "04" + node2 
	
	fr, err := h.PublicKeyToAddress(pk)
	if err != nil {
	    return false
	}

	if strings.EqualFold(from, fr) {
	    return true
	}
    }

    return false
}

//--------------------------------------------------------------------------------

type TxDataReShare struct {
    TxType string
    PubKey string
    GroupId string
    TSGroupId string
    ThresHold string
    Account string
    Mode string
    Sigs string
    TimeStamp string
}

func ReShare(raw string) (string, string, error) {
    key,_,_,txdata,err := CheckRaw(raw)
    if err != nil {
	common.Error("=====================ReShare,check raw data error ================","raw",raw,"err",err)
	return "",err.Error(),err
    }

    rh,ok := txdata.(*TxDataReShare)
    if !ok {
	return "","check raw fail,it is not *TxDataReShare",fmt.Errorf("check raw data fail")
    }

    common.Debug("=====================ReShare, SendMsgToSmpcGroup ================","raw",raw,"gid",rh.GroupId,"key",key)
    SendMsgToSmpcGroup(raw, rh.GroupId)
    SetUpMsgList(raw,cur_enode)
    return key, "", nil
}

//-----------------------------------------------------------------------------------

func RpcAcceptReShare(raw string) (string, string, error) {
    _,_,_,txdata,err := CheckRaw(raw)
    if err != nil {
	common.Error("=====================RpcAcceptReShare,check raw data error ================","raw",raw,"err",err)
	return "Failure",err.Error(),err
    }

    acceptrh,ok := txdata.(*TxDataAcceptReShare)
    if !ok {
	return "Failure","check raw fail,it is not *TxDataAcceptReShare",fmt.Errorf("check raw fail,it is not *TxDataAcceptReShare")
    }

    exsit,da := GetReShareInfoData([]byte(acceptrh.Key))
    if exsit {
	ac,ok := da.(*AcceptReShareData)
	if ok && ac != nil {
	    common.Debug("=====================RpcAcceptReShare, SendMsgToSmpcGroup ================","raw",raw,"gid",ac.GroupId,"key",acceptrh.Key)
	    SendMsgToSmpcGroup(raw, ac.GroupId)
	    SetUpMsgList(raw,cur_enode)
	    return "Success", "", nil
	}
    }

    return "Failure","accept fail",fmt.Errorf("accept fail")
}

//-------------------------------------------------------------------------------------

type ReShareStatus struct {
	Status    string
	Pubkey string
	Tip       string
	Error     string
	AllReply  []NodeReply 
	TimeStamp string
}

func GetReShareStatus(key string) (string, string, error) {
	exsit,da := GetPubKeyData([]byte(key))
	if !exsit || da == nil  {
		return "", "smpc back-end internal error:get reshare accept data fail from db when GetReShareStatus", fmt.Errorf("get reshare accept data fail from db")
	}

	ac,ok := da.(*AcceptReShareData)
	if !ok {
		return "", "smpc back-end internal error:get reshare accept data error from db when GetReShareStatus", fmt.Errorf("get reshare accept data error from db")
	}

	los := &ReShareStatus{Status: ac.Status, Pubkey: ac.PubKey, Tip: ac.Tip, Error: ac.Error, AllReply: ac.AllReply, TimeStamp: ac.TimeStamp}
	ret,_ := json.Marshal(los)
	return string(ret), "",nil 
}

//-------------------------------------------------------------------------------------

type ReShareCurNodeInfo struct {
	Key       string
	PubKey   string
	GroupId   string
	TSGroupId   string
	ThresHold  string
	Account string
	Mode string
	TimeStamp string
}

type ReShareCurNodeInfoSort struct {
	Info []*ReShareCurNodeInfo
}

func (r *ReShareCurNodeInfoSort) Len() int {
	return len(r.Info)
}

func (r *ReShareCurNodeInfoSort) Less(i, j int) bool {
	itime,_ := new(big.Int).SetString(r.Info[i].TimeStamp,10)
	jtime,_ := new(big.Int).SetString(r.Info[j].TimeStamp,10)
	return itime.Cmp(jtime) >= 0
}

func (r *ReShareCurNodeInfoSort) Swap(i, j int) {
    r.Info[i],r.Info[j] = r.Info[j],r.Info[i]
}

func GetCurNodeReShareInfo() ([]*ReShareCurNodeInfo, string, error) {
    var ret []*ReShareCurNodeInfo
    data := make(chan *ReShareCurNodeInfo,1000)

    var wg sync.WaitGroup
    iter := reshareinfodb.NewIterator()
    for iter.Next() {
	key2 := []byte(string(iter.Key())) //must be deep copy, Otherwise, an error will be reported: "panic: JSON decoder out of sync - data changing underfoot?"
	exsit,da := GetReShareInfoData(key2) 
	if !exsit || da == nil {
	    continue
	}
	
	wg.Add(1)
	go func(key string,value interface{},ch chan *ReShareCurNodeInfo) {
	    defer wg.Done()

	    vv,ok := value.(*AcceptReShareData)
	    if vv == nil || !ok {
		return
	    }

	    common.Debug("================GetCurNodeReShareInfo====================","vv",vv,"vv.Deal",vv.Deal,"vv.Status",vv.Status,"key",key)
	    if vv.Deal == "true" || vv.Status == "Success" {
		return
	    }

	    if vv.Status != "Pending" {
		return
	    }

	    los := &ReShareCurNodeInfo{Key: key, PubKey:vv.PubKey, GroupId:vv.GroupId, TSGroupId:vv.TSGroupId,ThresHold: vv.LimitNum, Account:vv.Account, Mode:vv.Mode, TimeStamp: vv.TimeStamp}
	    ch <-los
	    common.Debug("================GetCurNodeReShareInfo success return============================","key",key)
	}(string(key2),da,data)
    }
    iter.Release()
    wg.Wait()

    l := len(data)
    for i:=0;i<l;i++ {
	info := <-data
	ret = append(ret,info)
    }

    reshareinfosort := ReShareCurNodeInfoSort{Info:ret}
    sort.Sort(&reshareinfosort)

    return reshareinfosort.Info, "", nil
}

//-----------------------------------------------------------------------------------------

//param groupid is not subgroupid
//w.groupid is subgroupid
func _reshare(wsid string, initator string, groupid string,pubkey string,account string,mode string,sigs string,ch chan interface{}) {

	rch := make(chan interface{}, 1)
	smpc_reshare(wsid,initator,groupid,pubkey,account,mode,sigs,rch)
	ret, _, cherr := GetChannelValue(ch_t, rch)
	if ret != "" {
		w, err := FindWorker(wsid)
		if w == nil || err != nil {
			res := RpcSmpcRes{Ret: "", Tip: "smpc back-end internal error:no find worker", Err: fmt.Errorf("get worker error.")}
			ch <- res
			return
		}

		//sid-enode:SendReShareRes:Success:ret
		//sid-enode:SendReShareRes:Fail:err
		mp := []string{w.sid, cur_enode}
		enode := strings.Join(mp, "-")
		s0 := "SendReShareRes"
		s1 := "Success"
		s2 := ret
		ss := enode + common.Sep + s0 + common.Sep + s1 + common.Sep + s2
		SendMsgToSmpcGroup(ss, groupid)

		tip, reply := AcceptReShare("",initator, groupid,w.groupid,pubkey, w.limitnum, mode,"true", "true", "Success", ret, "", "", nil, w.id)
		if reply != nil {
			res := RpcSmpcRes{Ret: "", Tip: tip, Err: fmt.Errorf("update reshare status error.")}
			ch <- res
			return
		}

		common.Info("================reshare,the terminal res is success=================","key",wsid)
		res := RpcSmpcRes{Ret: ret, Tip: tip, Err: err}
		ch <- res
		return
	}

	if cherr != nil {
		res := RpcSmpcRes{Ret: "", Tip: "smpc back-end internal error:reshare fail", Err: cherr}
		ch <- res
		return
	}
}

//---------------------------------------------------------------------------------------

//ec2
//msgprex = hash
//return value is the backup for smpc sig.
func smpc_reshare(msgprex string, initator string, groupid string,pubkey string,account string,mode string,sigs string,ch chan interface{}) {

	w, err := FindWorker(msgprex)
	if w == nil || err != nil {
		res := RpcSmpcRes{Ret: "", Tip: "smpc back-end internal error:no find worker", Err: fmt.Errorf("no find worker.")}
		ch <- res
		return
	}
	id := w.id

    var ch1 = make(chan interface{}, 1)
    for i:=0;i < recalc_times;i++ {
	if len(ch1) != 0 {
	    <-ch1
	}

	ReShare_ec2(msgprex, initator, groupid,pubkey, account,mode,sigs, ch1, id)
	ret, _, cherr := GetChannelValue(ch_t, ch1)
	if ret != "" && cherr == nil {
		res := RpcSmpcRes{Ret: ret, Tip: "", Err: cherr}
		ch <- res
		break
	}
	
	w.Clear2()
	time.Sleep(time.Duration(1) * time.Second) //1000 == 1s
    }
}

//-------------------------------------------------------------------------------------------------------

//msgprex = hash
//return value is the backup for the smpc sig
func ReShare_ec2(msgprex string, initator string, groupid string,pubkey string, account string,mode string,sigs string,ch chan interface{}, id int) {
	if id < 0 || id >= len(workers) {
		res := RpcSmpcRes{Ret: "", Err: fmt.Errorf("no find worker.")}
		ch <- res
		return
	}

	w := workers[id]
	if w.groupid == "" {
		res := RpcSmpcRes{Ret: "", Err: fmt.Errorf("get group id fail.")}
		ch <- res
		return
	}

	ns, _ := GetGroup(groupid)
	if ns != w.NodeCnt {
		res := RpcSmpcRes{Ret: "", Err: GetRetErr(ErrGroupNotReady)}
		ch <- res
		return
	}

	smpcpks, _ := hex.DecodeString(pubkey)
	exsit,da := GetPubKeyData(smpcpks[:])
	oldnode := true
	if !exsit {
	    oldnode = false
	}

	if oldnode {
	    _,ok := da.(*PubKeyData)
	    if !ok || (da.(*PubKeyData)).GroupId == "" {
		res := RpcSmpcRes{Ret: "", Tip: "smpc back-end internal error:get sign data from db fail", Err: fmt.Errorf("get sign data from db fail")}
		ch <- res
		return
	    }

	    save := (da.(*PubKeyData)).Save
	    mm := strings.Split(save, common.SepSave)
	    if len(mm) == 0 {
		    res := RpcSmpcRes{Ret: "", Err: fmt.Errorf("reshare get save data fail")}
		    ch <- res
		    return 
	    }
	    
	    sd := &keygen.LocalDNodeSaveData{}
	    ///sku1
	    da2 := getSkU1FromLocalDb(smpcpks[:])
	    if da2 == nil {
		    res := RpcSmpcRes{Ret: "", Tip: "reshare get sku1 fail", Err: fmt.Errorf("reshare get sku1 fail")}
		    ch <- res
		    return 
	    }
	    sku1 := new(big.Int).SetBytes(da2)
	    if sku1 == nil {
		    res := RpcSmpcRes{Ret: "", Tip: "reshare get sku1 fail", Err: fmt.Errorf("reshare get sku1 fail")}
		    ch <- res
		    return 
	    }
	    //
	    sd.SkU1 = sku1
	    pkx, pky := secp256k1.S256().Unmarshal(smpcpks[:])
	    sd.Pkx = pkx
	    sd.Pky = pky

	    sd.U1PaillierSk = GetCurNodePaillierSkFromSaveData(save,(da.(*PubKeyData)).GroupId,"EC256K1")

	    U1PaillierPk := make([]*ec2.PublicKey,w.NodeCnt)
	    U1NtildeH1H2 := make([]*ec2.NtildeH1H2,w.NodeCnt)
	    for i:=0;i<w.NodeCnt;i++ {
		U1PaillierPk[i] = GetPaillierPkByIndexFromSaveData(save,i)
		U1NtildeH1H2[i] = GetNtildeByIndexFromSaveData(save,i,w.NodeCnt)
	    }
	    sd.U1PaillierPk = U1PaillierPk
	    sd.U1NtildeH1H2 = U1NtildeH1H2

	    sd.Ids = GetIds("EC256K1",(da.(*PubKeyData)).GroupId)
	    sd.CurDNodeID = DoubleHash(cur_enode,"EC256K1")
	
	    msgtoenode := GetMsgToEnode("EC256K1",(da.(*PubKeyData)).GroupId)
	    kgsave := &KGLocalDBSaveData{Save:sd,MsgToEnode:msgtoenode}
	    
	    found := false
	    idreshare := GetIdReshareByGroupId(kgsave.MsgToEnode,w.groupid)
	    for _,v := range idreshare {
		if v.Cmp(sd.CurDNodeID) == 0 {
		    found = true
		    break
		}
	    }

	    if !found {
		oldnode = false
	    }

	    if oldnode {
		fmt.Printf("================= ReShare_ec2,oldnode is true, groupid = %v, w.groupid = %v =======================\n",groupid,w.groupid)
		commStopChan := make(chan struct{})
		outCh := make(chan smpclib.Message, ns)
		endCh := make(chan keygen.LocalDNodeSaveData, ns)
		errChan := make(chan struct{})
		reshareDNode := reshare.NewLocalDNode(outCh,endCh,ns,w.ThresHold,2048,sd,true)
		w.DNode = reshareDNode
		reshareDNode.SetDNodeID(fmt.Sprintf("%v",DoubleHash(cur_enode,"EC256K1")))
		
		uid,_ := new(big.Int).SetString(w.DNode.DNodeID(),10)
		w.MsgToEnode[fmt.Sprintf("%v",uid)] = cur_enode

		var reshareWg sync.WaitGroup
		reshareWg.Add(2)
		go func() {
			defer reshareWg.Done()
			if err := reshareDNode.Start(); nil != err {
			    fmt.Printf("==========reshare node start err = %v ==========\n",err)
				close(errChan)
			}
			
			HandleC1Data(nil,w.sid)
		}()
		go ReshareProcessInboundMessages(msgprex,commStopChan,&reshareWg,ch)
		newsku1,err := processReshare(msgprex,groupid,pubkey,account,mode,sigs,errChan, outCh, endCh)
		if err != nil {
		    fmt.Printf("==========process reshare err = %v ==========\n",err)
		    close(commStopChan)
		    res := RpcSmpcRes{Ret: "", Err: err}
		    ch <- res
		    return
		}

		res := RpcSmpcRes{Ret: fmt.Sprintf("%v",newsku1), Err: nil}
		ch <- res
		close(commStopChan)
		reshareWg.Wait()
		return
	    }
	}

	fmt.Printf("================= ReShare_ec2,oldnode is false, groupid = %v, w.groupid = %v,w.ThresHold = %v,w.sid = %v, msgprex = %v =======================\n",groupid,w.groupid,w.ThresHold,w.sid,msgprex)
	commStopChan := make(chan struct{})
	outCh := make(chan smpclib.Message, ns)
	endCh := make(chan keygen.LocalDNodeSaveData, ns)
	errChan := make(chan struct{})
	reshareDNode := reshare.NewLocalDNode(outCh,endCh,ns,w.ThresHold,2048,nil,false)
	w.DNode = reshareDNode
	reshareDNode.SetDNodeID(fmt.Sprintf("%v",DoubleHash(cur_enode,"EC256K1")))
	
	uid,_ := new(big.Int).SetString(w.DNode.DNodeID(),10)
	w.MsgToEnode[fmt.Sprintf("%v",uid)] = cur_enode

	var reshareWg sync.WaitGroup
	reshareWg.Add(2)
	go func() {
		defer reshareWg.Done()
		if err := reshareDNode.Start(); nil != err {
		    fmt.Printf("==========reshare node start err = %v ==========\n",err)
			close(errChan)
		}
		
		HandleC1Data(nil,w.sid)
	}()
	go ReshareProcessInboundMessages(msgprex,commStopChan,&reshareWg,ch)
	newsku1,err := processReshare(msgprex,groupid,pubkey,account,mode,sigs,errChan,outCh,endCh)
	if err != nil {
	    fmt.Printf("==========process reshare err = %v ==========\n",err)
	    close(commStopChan)
	    res := RpcSmpcRes{Ret: "", Err: err}
	    ch <- res
	    return
	}

	res := RpcSmpcRes{Ret: fmt.Sprintf("%v",newsku1), Err: nil}
	ch <- res
	close(commStopChan)
	reshareWg.Wait()
}

//-------------------------------------------------------------------------------------------------------------

func GetIdReshareByGroupId(msgtoenode map[string]string,groupid string) smpclib.SortableIDSSlice {
    var ids smpclib.SortableIDSSlice

    _, enodes := GetGroup(groupid)

    nodes := strings.Split(enodes, common.Sep2)
    for _, node := range nodes {
	node2 := ParseNode(node)
	for key,value := range msgtoenode {
	    if strings.EqualFold(value,node2) {
		uid,_ := new(big.Int).SetString(key,10)
		ids = append(ids,uid)
		break
	    }
	}
    }
    
    sort.Sort(ids)
    return ids
}

//----------------------------------------------------------------------------------------------------------------

func GetNewIdsByNewGroupId(OldMsgToEnode map[string]string,NewGroupId string) smpclib.SortableIDSSlice {
    var ids smpclib.SortableIDSSlice

    _, enodes := GetGroup(NewGroupId)
    nodes := strings.Split(enodes, common.Sep2)
    for _, node := range nodes {
	node2 := ParseNode(node)
	found := false
	for key,value := range OldMsgToEnode {
	    if strings.EqualFold(value,node2) {
		uid,_ := new(big.Int).SetString(key,10)
		ids = append(ids,uid)
		found = true
		break
	    }
	}

	if !found {
	    uid := random.GetRandomIntFromZn(secp256k1.S256().N)
	    ids = append(ids,uid)
	}
    }
    
    sort.Sort(ids)
    return ids
}

