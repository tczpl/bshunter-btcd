package main

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcutil"
)

type void struct{}

var voidM void

const CACHE_LIMIT = 10000000

//	5000000x2 8G
//
// 10000000x2 16G
// 20000000x2 32G
const ALICE_INT = 0
const BOB_INT = 1

type BSHunter struct {
	Server         *server
	CacheAlice     map[string]btcjson.BSHunterVout
	CacheBob       map[string]btcjson.BSHunterVout
	CacheFirst     int
	CacheAddCnt    int
	CacheDeleteCnt int
	CacheNoneedCnt int
	CacheMissed    int

	IsErrNum       map[int32]void
	RawBlockByHash map[string]*btcutil.Block
	RawBlockByNm   map[int32]*btcutil.Block
}

func newBSHunter(server *server) *BSHunter {
	bshunter := BSHunter{
		Server:         server,
		CacheBob:       make(map[string]btcjson.BSHunterVout),
		CacheAlice:     make(map[string]btcjson.BSHunterVout),
		CacheFirst:     ALICE_INT,
		IsErrNum:       make(map[int32]void),
		RawBlockByHash: make(map[string]*btcutil.Block),
		RawBlockByNm:   make(map[int32]*btcutil.Block),
	}
	return &bshunter
}

func (bshunter *BSHunter) CacheAddTxResult(tx *btcjson.BSHunterTxResult) {
	// only disk
	// return

	txid := tx.Txid
	for vout, oneOut := range tx.Vout {
		if oneOut.ScriptPubKey.Unspendable == false {
			// two tier
			// bshunter.CacheAddVoutTwoTier(txid, vout, oneOut)
			bshunter.CacheAddVout(txid, vout, oneOut)
		} else {
			bshunter.CacheNoneedCnt++
		}
	}
}

func (bshunter *BSHunter) CacheGetVout(txid string, vout int) (btcjson.BSHunterVout, bool) {
	theKey := txid + ":" + strconv.Itoa(vout)
	if bshunter.CacheFirst == ALICE_INT {
		ret, ok := bshunter.CacheAlice[theKey]
		if !ok {
			ret, ok = bshunter.CacheBob[theKey]
		}
		return ret, ok
	} else {
		ret, ok := bshunter.CacheBob[theKey]
		if !ok {
			ret, ok = bshunter.CacheAlice[theKey]
		}
		return ret, ok
	}
}

func (bshunter *BSHunter) CacheAddVout(txid string, vout int, oneOut btcjson.BSHunterVout) {
	theKey := txid + ":" + strconv.Itoa(vout)
	if bshunter.CacheFirst == ALICE_INT {
		bshunter.CacheAlice[theKey] = oneOut
	} else {
		bshunter.CacheBob[theKey] = oneOut
	}
	bshunter.CacheAddCnt++

	// Check, Remove and Switch
	if bshunter.CacheFirst == ALICE_INT {
		if len(bshunter.CacheAlice) > CACHE_LIMIT {
			for k := range bshunter.CacheBob {
				delete(bshunter.CacheBob, k)
			}
			bshunter.CacheFirst = BOB_INT
		}
	} else {
		if len(bshunter.CacheBob) > CACHE_LIMIT {
			for k := range bshunter.CacheAlice {
				delete(bshunter.CacheAlice, k)
			}
			bshunter.CacheFirst = ALICE_INT
		}
	}
}

func (bshunter *BSHunter) CacheAddVoutTwoTier(txid string, vout int, oneOut btcjson.BSHunterVout) {
	theKey := txid + ":" + strconv.Itoa(vout)
	bshunter.CacheAlice[theKey] = oneOut
	bshunter.CacheAddCnt++

	if len(bshunter.CacheAlice) > CACHE_LIMIT {
		for k := range bshunter.CacheAlice {
			delete(bshunter.CacheAlice, k)
		}
	}
}

func (bshunter *BSHunter) CachePopVoutTwoTier(txid string, vout int) (btcjson.BSHunterVout, bool) {
	theKey := txid + ":" + strconv.Itoa(vout)
	ret, ok := bshunter.CacheAlice[theKey]
	if ok {
		delete(bshunter.CacheAlice, theKey)
		bshunter.CacheDeleteCnt++
	} else {
		bshunter.CacheMissed++
	}
	return ret, ok
}

func (bshunter *BSHunter) CachePopVout(txid string, vout int) (btcjson.BSHunterVout, bool) {
	theKey := txid + ":" + strconv.Itoa(vout)
	if bshunter.CacheFirst == ALICE_INT {
		ret, ok := bshunter.CacheAlice[theKey]
		if ok {
			delete(bshunter.CacheAlice, theKey)
			bshunter.CacheDeleteCnt++
		} else {
			ret, ok = bshunter.CacheBob[theKey]
			if ok {
				delete(bshunter.CacheBob, theKey)
				bshunter.CacheDeleteCnt++
			} else {
				bshunter.CacheMissed++
			}
		}
		return ret, ok
	} else {
		ret, ok := bshunter.CacheBob[theKey]
		if ok {
			delete(bshunter.CacheBob, theKey)
			bshunter.CacheDeleteCnt++
		} else {
			ret, ok = bshunter.CacheAlice[theKey]
			if ok {
				delete(bshunter.CacheAlice, theKey)
				bshunter.CacheDeleteCnt++
			} else {
				bshunter.CacheMissed++
			}
		}
		return ret, ok
	}
}

func (bshunter *BSHunter) Run() {
	//bshunter.GetAllBlockToZip("./extracted_twotier/")
	//bshunter.GetConfirmedTxToZip("./BlockTxs.zip")
	bshunter.GetConfirmedBSHunterTxToZip("./BSHunterTxs2_new_new_new.zip")
	bshunter.TraceTxsToZip("./TraceTxs_new_new_new.zip")

	//bshunter.TraceDebug()

	//bshunter.GetAllBlockToZip("/media/myname/fastdisk/extracted_new_new/")

	//bshunter.ScanWSHNoWitness()
	//bshunter.TODO5()
	//bshunter.CheckAllBlock()
	//	bshunter.CheckUTXOinAllBlock()

	//bshunter.ScanNonstandard()
	//bshunter.ExtractAllBlock("./extract/")
	os.Exit(0)
}

func (bshunter *BSHunter) TODO6() {

}

func (bshunter *BSHunter) ScanNonstandard() {
	height := bshunter.Server.chain.BestSnapshot().Height
	btcdLog.Info("target height", height)
	var memstats runtime.MemStats

	txsCnt := 0
	nonstandardTxs := make(map[string]void)
	for i := int32(0); i < height; i++ {
		blockResult := bshunter.getBlockResult(i)
		txsCnt += len(blockResult.Transactions)
		for _, tx := range blockResult.Transactions {
			for _, vin := range tx.Vin {
				if vin.VoutContent.ScriptPubKey.Type == "nonstandard" {
					nonstandardTxs[tx.Txid] = voidM
				} else if vin.DecodedScript != nil && vin.DecodedScript.Type == "nonstandard" {
					nonstandardTxs[vin.Txid] = voidM
					nonstandardTxs[tx.Txid] = voidM
				} else if vin.DecodedWitnessScript != nil && vin.DecodedWitnessScript.Type == "nonstandard" {
					nonstandardTxs[vin.Txid] = voidM
					nonstandardTxs[tx.Txid] = voidM
				}
			}
			for _, vout := range tx.Vout {
				if vout.ScriptPubKey.Type == "nonstandard" {
					nonstandardTxs[tx.Txid] = voidM
				}
			}
		}
		if i%1000 == 0 {
			runtime.ReadMemStats(&memstats)
			btcdLog.Infof("i=%d\ttxsCnt=%d\tnonStdTxs=%d", i, txsCnt, len(nonstandardTxs))
			btcdLog.Infof("alice=%d\tbob=%d\tadd=%d\tdelete=%d\tnoneed=%d\tmissed=%d", len(bshunter.CacheAlice), len(bshunter.CacheBob), bshunter.CacheAddCnt, bshunter.CacheDeleteCnt, bshunter.CacheNoneedCnt, bshunter.CacheMissed)
			btcdLog.Infof("sysmem=%d\talloc=%d\tgctime=%d", memstats.Sys, memstats.Alloc, memstats.PauseTotalNs)
		}
	}
}

func (bshunter *BSHunter) getBlockResultForTxs(blockHeight int32, txids []string) *btcjson.GetBlockVerboseResult {

	bc := bshunter.Server.GetChain()

	block, err := bc.BlockByHeight(blockHeight)
	if err != nil {
		btcdLog.Info("IsErrNum", blockHeight)
		block = bshunter.RawBlockByNm[blockHeight]
	}
	blkBytes, _ := block.Bytes()

	params := bshunter.Server.GetChainParams()
	blockHeader := block.MsgBlock().Header
	blockHash := block.Hash().String()
	blockReply := btcjson.GetBlockVerboseResult{
		Hash:          blockHash,
		Version:       blockHeader.Version,
		VersionHex:    fmt.Sprintf("%08x", blockHeader.Version),
		MerkleRoot:    blockHeader.MerkleRoot.String(),
		PreviousHash:  blockHeader.PrevBlock.String(),
		Nonce:         blockHeader.Nonce,
		Time:          blockHeader.Timestamp.Unix(),
		Confirmations: int64(695000 - blockHeight),
		Height:        int64(blockHeight),
		Size:          int32(len(blkBytes)),
		StrippedSize:  int32(block.MsgBlock().SerializeSizeStripped()),
		Weight:        int32(blockchain.GetBlockWeight(block)),
		Bits:          strconv.FormatInt(int64(blockHeader.Bits), 16),
		Difficulty:    getDifficultyRatio(blockHeader.Bits, params),
	}

	txns := block.Transactions()
	rawTxns := make([]btcjson.TxRawResult, 0)
	for _, tx := range txns {
		txid := tx.Hash().String()
		found := false
		for _, targetTxid := range txids {
			if txid == targetTxid {
				found = true
				btcdLog.Info("found", blockHeight, txid)
				break
			}
		}
		if !found {
			continue
		}
		rawTxn, err := createTxRawResult(params, tx.MsgTx(),
			tx.Hash().String(), &blockHeader, blockHash,
			blockHeight, 695000)
		if err != nil {
			panic(err)
		}
		rawTxns = append(rawTxns, *rawTxn)
	}
	blockReply.RawTx = rawTxns

	return &blockReply
}
func (bshunter *BSHunter) getBlockResult(blockHeight int32) *btcjson.BSHunterGetBlockResult {
	bc := bshunter.Server.GetChain()

	block, err := bc.BlockByHeight(blockHeight)
	if err != nil {
		btcdLog.Info("IsErrNum", blockHeight)
		block = bshunter.RawBlockByNm[blockHeight]
	}
	blkBytes, _ := block.Bytes()

	params := bshunter.Server.GetChainParams()
	blockHeader := block.MsgBlock().Header
	blockHash := block.Hash().String()
	blockResult := btcjson.BSHunterGetBlockResult{
		Hash:         blockHash,
		Version:      blockHeader.Version,
		Nonce:        blockHeader.Nonce,
		Time:         blockHeader.Timestamp.Unix(),
		Height:       int64(blockHeight),
		Size:         int32(len(blkBytes)),
		StrippedSize: int32(block.MsgBlock().SerializeSizeStripped()),
		Weight:       int32(blockchain.GetBlockWeight(block)),
		Bits:         strconv.FormatInt(int64(blockHeader.Bits), 16),
		Difficulty:   getDifficultyRatio(blockHeader.Bits, params),
	}

	txns := block.Transactions()

	//btcdLog.Info(blockHeight, len(txns))

	rawTxns := make([]btcjson.BSHunterTxResult, len(txns))
	for i, tx := range txns {
		rawTxn, _ := createBSHunterTxResultWithCache(params, tx.MsgTx(),
			tx.Hash().String(), &blockHeader, blockHash,
			blockHeight, blockHeight, bshunter.Server.GetTxIndex(), bshunter.Server.GetDb(), bshunter)
		rawTxns[i] = *rawTxn
		// parse
		for vinIndex, oneIn := range rawTxn.Vin {
			isWSH := false
			if oneIn.VoutContent.ScriptPubKey.Type == "scripthash" {
				//btcdLog.Info("scripthash!!!")
				arr := strings.Split(oneIn.ScriptSig.Asm, " ")
				reply, err := decodeScript(arr[len(arr)-1], params)
				//btcdLog.Info("decoded!!!", "reply=", reply, "err=", err)
				if err == nil {
					rawTxn.Vin[vinIndex].DecodedScript = reply
					if reply.Type == "witness_v0_scripthash" {
						isWSH = true
					}
				} else {
					btcdLog.Info("?", rawTxn.Txid, vinIndex, oneIn.VoutContent.ScriptPubKey.Type, isWSH, oneIn)
					panic(err)
				}
				//encodedJson, _ := json.Marshal(rawTxn.Vin[vinIndex])
				//btcdLog.Info("!!!!!!!", string(encodedJson))
				//encodedJson, _ := rawTxn.Vin[vinIndex].MarshalJSON()
				//btcdLog.Info("MarshalJSON", string(encodedJson))
			}
			if oneIn.VoutContent.ScriptPubKey.Type == "witness_v0_scripthash" || isWSH {
				//btcdLog.Info("?", rawTxn.Txid, vinIndex, oneIn.VoutContent.ScriptPubKey.Type, isWSH, oneIn.Witness)
				reply, err := decodeScript(oneIn.Witness[len(oneIn.Witness)-1], params)
				if err == nil {
					rawTxn.Vin[vinIndex].DecodedWitnessScript = reply
				} else {
					btcdLog.Info("?", rawTxn.Txid, vinIndex, oneIn.VoutContent.ScriptPubKey.Type, isWSH, oneIn)
					panic(err)
				}
			}
		}
		bshunter.CacheAddTxResult(rawTxn)
	}
	blockResult.Transactions = rawTxns

	return &blockResult
}

func (bshunter *BSHunter) getBSHunterResultForTxs(blockHeight int32, txids []string) *btcjson.BSHunterGetBlockResult {
	bc := bshunter.Server.GetChain()

	block, err := bc.BlockByHeight(blockHeight)
	if err != nil {
		btcdLog.Info("IsErrNum", blockHeight)
		block = bshunter.RawBlockByNm[blockHeight]
	}
	blkBytes, _ := block.Bytes()

	params := bshunter.Server.GetChainParams()
	blockHeader := block.MsgBlock().Header
	blockHash := block.Hash().String()
	blockResult := btcjson.BSHunterGetBlockResult{
		Hash:         blockHash,
		Version:      blockHeader.Version,
		Nonce:        blockHeader.Nonce,
		Time:         blockHeader.Timestamp.Unix(),
		Height:       int64(blockHeight),
		Size:         int32(len(blkBytes)),
		StrippedSize: int32(block.MsgBlock().SerializeSizeStripped()),
		Weight:       int32(blockchain.GetBlockWeight(block)),
		Bits:         strconv.FormatInt(int64(blockHeader.Bits), 16),
		Difficulty:   getDifficultyRatio(blockHeader.Bits, params),
	}

	txns := block.Transactions()

	//btcdLog.Info(blockHeight, len(txns))

	rawTxns := make([]btcjson.BSHunterTxResult, 0)
	for _, tx := range txns {
		txid := tx.Hash().String()
		found := false
		for _, targetTxid := range txids {
			if txid == targetTxid {
				found = true
				btcdLog.Info("found", blockHeight, txid)
				break
			}
		}
		if !found {
			continue
		}

		rawTxn, _ := createBSHunterTxResultWithCache(params, tx.MsgTx(),
			tx.Hash().String(), &blockHeader, blockHash,
			blockHeight, blockHeight, bshunter.Server.GetTxIndex(), bshunter.Server.GetDb(), bshunter)
		rawTxns = append(rawTxns, *rawTxn)
		// parse
		for vinIndex, oneIn := range rawTxn.Vin {
			isWSH := false
			if oneIn.VoutContent.ScriptPubKey.Type == "scripthash" {
				//btcdLog.Info("scripthash!!!")
				arr := strings.Split(oneIn.ScriptSig.Asm, " ")
				reply, err := decodeScript(arr[len(arr)-1], params)
				//btcdLog.Info("decoded!!!", "reply=", reply, "err=", err)
				if err == nil {
					rawTxn.Vin[vinIndex].DecodedScript = reply
					if reply.Type == "witness_v0_scripthash" {
						isWSH = true
					}
				} else {
					btcdLog.Info("?", rawTxn.Txid, vinIndex, oneIn.VoutContent.ScriptPubKey.Type, isWSH, oneIn)
					panic(err)
				}
				//encodedJson, _ := json.Marshal(rawTxn.Vin[vinIndex])
				//btcdLog.Info("!!!!!!!", string(encodedJson))
				//encodedJson, _ := rawTxn.Vin[vinIndex].MarshalJSON()
				//btcdLog.Info("MarshalJSON", string(encodedJson))
			}
			if oneIn.VoutContent.ScriptPubKey.Type == "witness_v0_scripthash" || isWSH {
				//btcdLog.Info("?", rawTxn.Txid, vinIndex, oneIn.VoutContent.ScriptPubKey.Type, isWSH, oneIn.Witness)
				reply, err := decodeScript(oneIn.Witness[len(oneIn.Witness)-1], params)
				if err == nil {
					rawTxn.Vin[vinIndex].DecodedWitnessScript = reply
				} else {
					btcdLog.Info("?", rawTxn.Txid, vinIndex, oneIn.VoutContent.ScriptPubKey.Type, isWSH, oneIn)
					panic(err)
				}
			}
		}
		bshunter.CacheAddTxResult(rawTxn)
	}
	blockResult.Transactions = rawTxns

	return &blockResult
}
func (bshunter *BSHunter) getBlock(blockHeight int32) string {
	blockResult := bshunter.getBlockResult(blockHeight)
	marshalledResult, _ := json.Marshal(blockResult)
	jsonstr := bytes.NewBuffer(marshalledResult).String()

	//btcdLog.Info("JSONSTR", jsonstr)
	return jsonstr
}

func (bshunter *BSHunter) getBlockJsonByte(blockHeight int32) []byte {
	blockResult := bshunter.getBlockResult(blockHeight)
	marshalledResult, _ := json.Marshal(blockResult)
	return marshalledResult
}

func (bshunter *BSHunter) getBlockJsonByteForTxs(blockHeight int32, txids []string) []byte {
	blockResult := bshunter.getBlockResultForTxs(blockHeight, txids)
	marshalledResult, _ := json.Marshal(blockResult)
	return marshalledResult
}

func (bshunter *BSHunter) getBSHunterJsonByteForTxs(blockHeight int32, txids []string) []byte {
	blockResult := bshunter.getBSHunterResultForTxs(blockHeight, txids)
	marshalledResult, _ := json.Marshal(blockResult)
	return marshalledResult
}

func decodeScript(hexStr string, params *chaincfg.Params) (*btcjson.BSHunterDecodeScriptResult, error) {
	script, err := hex.DecodeString(hexStr)
	if err != nil {
		reply := &btcjson.BSHunterDecodeScriptResult{
			Asm:  "decodingerr",
			Type: "nonstandard",
		}
		return reply, nil
		return nil, rpcDecodeHexError(hexStr)
	}

	// The disassembled string will contain [error] inline if the script
	// doesn't fully parse, so ignore the error here.
	disbuf, _ := txscript.DisasmString(script)

	// Get information about the script.
	// Ignore the error here since an error means the script couldn't parse
	// and there is no additinal information about it anyways.
	scriptClass, addrs, reqSigs, _ := txscript.ExtractPkScriptAddrs(script,
		params)
	addresses := make([]string, len(addrs))
	for i, addr := range addrs {
		addresses[i] = addr.EncodeAddress()
	}

	// Convert the script itself to a pay-to-script-hash address.
	p2sh, err := btcutil.NewAddressScriptHash(script, params)
	if err != nil {
		context := "Failed to convert script to pay-to-script-hash"
		return nil, internalRPCError(err.Error(), context)
	}

	// Generate and return the reply.
	reply := &btcjson.BSHunterDecodeScriptResult{
		Asm:       disbuf,
		ReqSigs:   int32(reqSigs),
		Type:      scriptClass.String(),
		Addresses: addresses,
	}
	if scriptClass != txscript.ScriptHashTy {
		reply.P2sh = p2sh.EncodeAddress()
	}
	return reply, nil
}

func writeFile(filePath string, fileContent string) {
	//aa
	file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		btcdLog.Info()
		fmt.Println("err", err)
	}
	defer file.Close()
	write := bufio.NewWriter(file)
	write.WriteString(fileContent)
	write.Flush()
}
