package main

import (
	"archive/zip"
	"bufio"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

func (bshunter *BSHunter) TraceDebug() {
	blockNum := int32(717996)
	txids := make([]string, 1)
	txids[0] = "734fe8547e47923de80f9b38eec0e3896c03e19d25e21f7f4f5940bb8d30786a"
	bshunter.traceBlock(blockNum, txids)
}

func (bshunter *BSHunter) TraceTxsToZip(zipPath string) {
	fi, err := os.Open("/media/myname/bigdisk/bshunter-btc/UnsafeBTC.com/confirmed_block_txid_new_new_new.txt")
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		return
	}
	defer fi.Close()

	needs := make(map[int32]([]string), 0)
	br := bufio.NewReader(fi)
	for {
		a, _, c := br.ReadLine()
		if c == io.EOF {
			break
		}
		oneLine := string(a)
		if oneLine == "" {
			break
		}
		arr := strings.Split(oneLine, ",")
		blockStr := arr[0]
		txidStr := arr[1]
		block, err := strconv.Atoi(blockStr)
		block32 := int32(block)
		if err != nil {
			panic(err)
		}
		if needs[block32] == nil {
			needs[block32] = make([]string, 0)
		}
		needs[block32] = append(needs[block32], txidStr)
	}
	btcdLog.Info(len(needs))

	height := int32(748999)
	fw, err := os.Create(zipPath)
	if err != nil {
		panic(err)
	}
	zw := zip.NewWriter(fw)
	for blockNum := int32(0); blockNum <= height; blockNum++ {
		if needs[blockNum] == nil {
			continue
		}
		btcdLog.Info("need", blockNum)

		toWrite := bshunter.traceBlock(blockNum, needs[blockNum])
		f, err := zw.Create(strconv.Itoa(int(blockNum)) + ".json")
		if err != nil {
			panic(err)
		}
		_, err1 := f.Write(toWrite)
		if err1 != nil {
			panic(err)
		}
	}
	zw.Close()
	fw.Close()
}

func (bshunter *BSHunter) traceBlock(blockHeight int32, txids []string) []byte {
	bc := bshunter.Server.GetChain()

	block, err := bc.BlockByHeight(blockHeight)
	if err != nil {
		btcdLog.Info("IsErrNum", blockHeight)
		block = bshunter.RawBlockByNm[blockHeight]
	}

	blockHeader := block.MsgBlock().Header
	var scriptFlags txscript.ScriptFlags
	enforceBIP0016 := blockHeader.Timestamp.Unix() >= txscript.Bip16Activation.Unix()
	if enforceBIP0016 {
		scriptFlags |= txscript.ScriptBip16

		scriptFlags |= txscript.ScriptVerifyWitness
	}
	if blockHeader.Version >= 3 && blockHeight >= bshunter.Server.chainParams.BIP0066Height {
		scriptFlags |= txscript.ScriptVerifyDERSignatures
	}
	if blockHeader.Version >= 4 && blockHeight >= bshunter.Server.chainParams.BIP0065Height {
		scriptFlags |= txscript.ScriptVerifyCheckLockTimeVerify
	}

	txns := block.Transactions()
	txOuts := make(map[string]*wire.TxOut)
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

		mTx := tx.MsgTx()
		for _, txIn := range mTx.TxIn {
			outputpoint := txIn.PreviousOutPoint
			if outputpoint.Index == math.MaxUint32 {
				continue
			}
			txOut := getTxOutFromDb(outputpoint, bshunter.Server.GetTxIndex(), bshunter.Server.GetDb())
			txOuts[outputpoint.String()] = txOut
		}
	}
	return blockchain.CheckBlockScriptsBSHunter(block, txids, txOuts, scriptFlags, bshunter.Server.sigCache, bshunter.Server.hashCache)
}

func (bshunter *BSHunter) traceBlockOld(blockHeight int32, txids []string) {
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

		txInIndex := 0
		//txIn := tx.MsgTx().TxIn[txInIndex]
		//sigScript := txIn.SignatureScript
		//witness := txIn.Witness
		pkScript, _ := hex.DecodeString(rawTxn.Vin[txInIndex].VoutContent.ScriptPubKey.Hex)
		//inputAmount := int(100)

		var scriptFlags txscript.ScriptFlags
		enforceBIP0016 := blockHeader.Timestamp.Unix() >= txscript.Bip16Activation.Unix()
		if enforceBIP0016 {
			scriptFlags |= txscript.ScriptBip16
		}
		if blockHeader.Version >= 3 && blockHeight >= bshunter.Server.chainParams.BIP0066Height {
			scriptFlags |= txscript.ScriptVerifyDERSignatures
		}
		if blockHeader.Version >= 4 && blockHeight >= bshunter.Server.chainParams.BIP0065Height {
			scriptFlags |= txscript.ScriptVerifyCheckLockTimeVerify
		}
		scriptFlags |= txscript.ScriptVerifyWitness

		vm, err := txscript.NewEngine(pkScript, tx.MsgTx(),
			txInIndex, scriptFlags, nil, nil,
			0)
		if err != nil {
			btcdLog.Info("exec err1", err)
		}

		// Execute the script pair.
		if err := vm.Execute(); err != nil {
			btcdLog.Info("exec err2", err)
		}

		btcdLog.Info("finish")

		break
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

}
