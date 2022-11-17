package main

import (
	"archive/zip"
	"bufio"
	"fmt"
	"io"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/btcsuite/btcd/btcjson"
)

func (bshunter *BSHunter) CheckUTXOinAllBlock() {
	height := bshunter.Server.chain.BestSnapshot().Height
	btcdLog.Info("target height", height)
	allCnt := 0
	isCnt := 0
	for blockHeight := int32(0); blockHeight < height; blockHeight++ {
		bc := bshunter.Server.GetChain()
		block, err := bc.BlockByHeight(blockHeight)
		if err != nil {
			btcdLog.Info("IsErrNum", blockHeight)
			block = bshunter.RawBlockByNm[blockHeight]
		}
		txns := block.Transactions()
		isInThisBlock := make(map[string]void)
		for _, tx := range txns {
			isInThisBlock[tx.Hash().String()] = voidM
		}
		for _, tx := range txns {
			mtx := tx.MsgTx()
			for _, vin := range mtx.TxIn {
				previousTx := vin.PreviousOutPoint.Hash.String()
				_, ok := isInThisBlock[previousTx]
				if ok {
					btcdLog.Info("is!!!", blockHeight, previousTx)
					isCnt++
					panic("!")
				}
				allCnt++
			}
		}
		if blockHeight%1000 == 0 {
			btcdLog.Info(blockHeight, allCnt, isCnt)
		}
	}
}

func (bshunter *BSHunter) GetAllBlock() {
	height := bshunter.Server.chain.BestSnapshot().Height
	btcdLog.Info("target height", height)
	for i := int32(0); i < height; i += 1000 {
		toWrite := bshunter.getBlock(i)
		writeFile("./testdata/"+strconv.Itoa(int(i))+".json", toWrite)
	}
}

func (bshunter *BSHunter) GetAllBlockToZip(zipPath string) {
	height := bshunter.Server.chain.BestSnapshot().Height
	begin := int32(718000)
	height = 718999

	/*
		ZipWriteTime i=748000       ziptime=10600123897     writetime=1847460745070
		2022-08-13 05:58:02.035 [INF] BTCD: blockNum=748999     alice=6947058   bob=6381488     add=110535485   delete=97206939 noneed=609659    missed=10055301
		2022-08-13 05:58:02.035 [INF] BTCD: sysmem=9341158600   alloc=8263862576        gctime=2680212072
	*/
	btcdLog.Info("modified target height", height)
	var memstats runtime.MemStats
	ziptime := int64(0)
	writetime := int64(0)
	for i := begin; i < height; i += 1000 {
		runtime.ReadMemStats(&memstats)
		btcdLog.Infof("i=%d\talice=%d\tbob=%d\tadd=%d\tdelete=%d\tnoneed=%d\tmissed=%d", i, len(bshunter.CacheAlice), len(bshunter.CacheBob), bshunter.CacheAddCnt, bshunter.CacheDeleteCnt, bshunter.CacheNoneedCnt, bshunter.CacheMissed)
		btcdLog.Infof("sysmem=%d\talloc=%d\tgctime=%d", memstats.Sys, memstats.Alloc, memstats.PauseTotalNs)
		fw, err := os.Create(zipPath + strconv.Itoa(int(i)) + ".zip")
		if err != nil {
			panic(err)
		}
		zw := zip.NewWriter(fw)
		for blockNum := i; blockNum < i+1000; blockNum++ {
			btcdLog.Infof("blockNum=%d", blockNum)
			toWrite := bshunter.getBlockJsonByte(blockNum)
			time1 := time.Now().UnixNano()
			f, err := zw.Create(strconv.Itoa(int(blockNum)) + ".json")
			if err != nil {
				panic(err)
			}
			time2 := time.Now().UnixNano()
			_, err1 := f.Write(toWrite)
			if err1 != nil {
				panic(err)
			}
			time3 := time.Now().UnixNano()
			ziptime += time2 - time1
			writetime += time3 - time2
		}
		btcdLog.Infof("ZipWriteTime i=%d\tziptime=%d\twritetime=%d", i, ziptime, writetime)
		zw.Close()
		fw.Close()
	}
	runtime.ReadMemStats(&memstats)
	btcdLog.Infof("blockNum=%d\talice=%d\tbob=%d\tadd=%d\tdelete=%d\tnoneed=%d\tmissed=%d", height, len(bshunter.CacheAlice), len(bshunter.CacheBob), bshunter.CacheAddCnt, bshunter.CacheDeleteCnt, bshunter.CacheNoneedCnt, bshunter.CacheMissed)
	btcdLog.Infof("sysmem=%d\talloc=%d\tgctime=%d", memstats.Sys, memstats.Alloc, memstats.PauseTotalNs)
}

func (bshunter *BSHunter) GetConfirmedTxToZip(zipPath string) {
	fi, err := os.Open("/home/zpl/bshunter-btc/UnsafeBTC.com/confirmed_block_txid.txt")
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

	height := int32(694999)
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

		toWrite := bshunter.getBlockJsonByteForTxs(blockNum, needs[blockNum])
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

func (bshunter *BSHunter) GetConfirmedBSHunterTxToZip(zipPath string) {
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

		toWrite := bshunter.getBSHunterJsonByteForTxs(blockNum, needs[blockNum])
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

func (bshunter *BSHunter) buok984() {
	bshunter.checkBlock(600984)
}

func (bshunter *BSHunter) ok984() {
	bshunter.checkBlock(600550)
	bshunter.checkBlock(600551)
	bshunter.checkBlock(600552)
	bshunter.checkBlock(600553)
	bshunter.checkBlock(600554)
	bshunter.checkBlock(600555)
	bshunter.checkBlock(600556)
	bshunter.checkBlock(600560)
	bshunter.checkBlock(600561)
	bshunter.checkBlock(600984)
}

func (bshunter *BSHunter) ScanWSHNoWitness() {
	for number := int32(600900); number < 610000; number++ {
		res := bshunter.findWSHNoWitnessWithCache(number)
		if res >= 0 {
			btcdLog.Info("Find!!!!!!!!", number)
			//break
		}
	}
}

func (bshunter *BSHunter) checkBlock(blockHeight int32) int {
	bc := bshunter.Server.GetChain()
	block, _ := bc.BlockByHeight(blockHeight)

	blockBytes, _ := block.Bytes()

	btcdLog.Info("checkBlock", "height", blockHeight, "len(bytes)", len(blockBytes))

	params := bshunter.Server.GetChainParams()
	blockHeader := block.MsgBlock().Header
	blockHash := block.Hash().String()

	txns := block.Transactions()

	okcnt := 0
	nocnt := 0

	hascnt := 0

	for _, tx := range txns {
		rawTxn, _ := createBSHunterTxResultWithCache(params, tx.MsgTx(),
			tx.Hash().String(), &blockHeader, blockHash,
			blockHeight, blockHeight, bshunter.Server.GetTxIndex(), bshunter.Server.GetDb(), bshunter)

		// Cache
		bshunter.CacheAddTxResult(rawTxn)

		if tx.MsgTx().HasWitness() {
			hascnt++
			btcdLog.Debug("tx.MsgTx().HasWitness()", rawTxn.Txid)
		}

		for vinIndex, oneIn := range rawTxn.Vin {
			if oneIn.VoutContent.ScriptPubKey.Type == "scripthash" {
				reply, err := decodeScript(oneIn.ScriptSig.Asm, params)
				//btcdLog.Info("decoded!!!", reply, err)
				if err == nil {
					if reply.Type == "witness_v0_scripthash" {
						if len(oneIn.Witness) == 0 {
							if blockHeight == 600984 {
								btcdLog.Info(blockHeight, "SH-WSH No Witness !!!", rawTxn.Txid, vinIndex, oneIn.Txid, oneIn.Vout)
							}
							nocnt++
						} else {
							if blockHeight == 600984 {
								btcdLog.Debug(blockHeight, "SH-WSH ok", rawTxn.Txid, vinIndex)
							}
							okcnt++
						}
					}
				}
			}
		}

		if rawTxn.Txid == "f3c51a8f10cd68652bf236a000f8dfc225c0b72adad406771303b4e8596f897f" {
			t := 0
			for _, oneIn := range rawTxn.Vin {
				if oneIn.HasWitness() {
					t++
				}
			}
			btcdLog.Info("Here!!!", rawTxn.Txid, t, "of", len(rawTxn.Vin))
			//for vinIndex, oneIn := range rawTxn.Vin {
			//	btcdLog.Info(vinIndex, oneIn.Txid, oneIn.Vout, oneIn.VoutContent.Value, oneIn.VoutContent.ScriptPubKey.Type)
			//}
		}

	}

	btcdLog.Info(blockHeight, len(txns), bshunter.CacheAddCnt, bshunter.CacheDeleteCnt)
	btcdLog.Info(blockHeight, hascnt, okcnt, nocnt)

	return -1
}

func (bshunter *BSHunter) findWSHNoWitnessWithCache(blockHeight int32) int {
	bc := bshunter.Server.GetChain()
	block, err := bc.BlockByHeight(blockHeight)
	btcdLog.Debug("test", "block", block, "err", err)

	params := bshunter.Server.GetChainParams()
	blockHeader := block.MsgBlock().Header
	blockHash := block.Hash().String()

	txns := block.Transactions()

	rawTxns := make([]btcjson.BSHunterTxResult, len(txns))

	okcnt := 0
	nocnt := 0
	for i, tx := range txns {
		rawTxn, _ := createBSHunterTxResultWithCache(params, tx.MsgTx(),
			tx.Hash().String(), &blockHeader, blockHash,
			blockHeight, blockHeight, bshunter.Server.GetTxIndex(), bshunter.Server.GetDb(), bshunter)
		rawTxns[i] = *rawTxn

		// Cache
		bshunter.CacheAddTxResult(rawTxn)

		// linshi
		for vinIndex, oneIn := range rawTxn.Vin {
			if oneIn.VoutContent.ScriptPubKey.Type == "scripthash" {
				reply, err := decodeScript(oneIn.ScriptSig.Asm, params)
				//btcdLog.Info("decoded!!!", reply, err)
				if err == nil {
					if reply.Type == "witness_v0_scripthash" {
						if len(oneIn.Witness) == 0 {
							btcdLog.Debug(blockHeight, "SH-WSH No Witness !!!", rawTxn.Txid, vinIndex)
							nocnt++
						} else {
							btcdLog.Debug(blockHeight, "SH-WSH ok", rawTxn.Txid, vinIndex)
							okcnt++
						}
					}
				} else {
					btcdLog.Debug("p2sh cannot decode!!!")
				}
			}
		}
	}

	btcdLog.Info(blockHeight, len(txns), bshunter.CacheAddCnt, bshunter.CacheDeleteCnt)
	btcdLog.Info(blockHeight, okcnt, nocnt)

	return -1
}

func (bshunter *BSHunter) TODO0() {
	var number int32
	number = 600993
	toWrite := bshunter.getBlock(number)
	writeFile("./"+strconv.Itoa(int(number))+".json", toWrite)
}

func (bshunter *BSHunter) TODO1() {
	var number int32
	number = 600001
	toWrite := bshunter.getBlock(number)
	writeFile("./"+strconv.Itoa(int(number))+".json", toWrite)
}

func (bshunter *BSHunter) TODO2() {
	bshunter.getBlock(223000)
}

func (bshunter *BSHunter) TODO3() {
	bshunter.getBlock(273568)
	bshunter.getBlock(274000)
}

func (bshunter *BSHunter) TODO4() {
	bshunter.getBlock(317001)
	bshunter.getBlock(317000)
}

func (bshunter *BSHunter) TODO5() {
	var number int32
	number = 545348
	toWrite := bshunter.getBlock(number)
	writeFile("./"+strconv.Itoa(int(number))+".json", toWrite)
}

func (bshunter *BSHunter) CheckFixBlock() {
	bc := bshunter.Server.GetChain()

	okBlockCnt := 0
	errBlockCnt := 0
	txCnt := 0

	errNums := make([]int32, 0)
	for number, _ := range bshunter.IsErrNum {
		block, err := bc.BlockByHeight(number)
		if err == nil {
			okBlockCnt++
			txCnt += len(block.Transactions())
		} else {
			errBlockCnt++
			errNums = append(errNums, number)
		}
		btcdLog.Info(number, "ok", okBlockCnt, "err", errBlockCnt, "txCnt", txCnt)
		btcdLog.Info("errNums", errNums)
	}
	btcdLog.Info("finish", "ok", okBlockCnt, "err", errBlockCnt, "txCnt", txCnt)
	btcdLog.Info("errNums", errNums)
	// 273566 273567 273568 273569 273570 273571 273572 273573 403767 403768 542425 542426
}
func (bshunter *BSHunter) CheckAllBlock() {
	height := bshunter.Server.chain.BestSnapshot().Height
	btcdLog.Info("target height", height)
	bc := bshunter.Server.GetChain()

	okBlockCnt := 0
	errBlockCnt := 0
	txCnt := 0

	errNums := make([]int32, 0)
	for number := height; number > int32(0); number-- {
		block, err := bc.BlockByHeight(number)
		if err == nil {
			okBlockCnt++
			txCnt += len(block.Transactions())
		} else {
			errBlockCnt++
			errNums = append(errNums, number)
		}
		if number%1000 == 0 || number > 600000 {
			btcdLog.Info(number, "ok", okBlockCnt, "err", errBlockCnt, "txCnt", txCnt)
			btcdLog.Info("errNums", errNums)
		}
	}
	btcdLog.Info("finish", "ok", okBlockCnt, "err", errBlockCnt, "txCnt", txCnt)
	btcdLog.Info("errNums", errNums)
	// 273566 273567 273568 273569 273570 273571 273572 273573 403767 403768 542425 542426
}

func (bshunter *BSHunter) CheckMarshal() {
	toWrite := bshunter.getBlock(574503)
	writeFile("./574503.json", toWrite)
}

func (bshunter *BSHunter) ScanTxType() {
	for number := int32(0); number < 610000; number++ {
		res := bshunter.findBlockTxInTypesWithCache(number, "witness_v0_scripthash")
		if res >= 0 {
			btcdLog.Info("Find!!!!!!!!", number, res)
			break
		}
	}

}
func (bshunter *BSHunter) WriteBlockResult() {
	// print for
	for number := int32(170060); number < 170061; number++ {
		toWrite := bshunter.getBlock(number)
		writeFile("./"+strconv.Itoa(int(number))+".json", toWrite)
	}

}

func (bshunter *BSHunter) findBlockTxInTypesWithCache(blockHeight int32, theType string) int {
	// multisig 	170748
	// scripthash 	170060
	// witness_v0_scripthash
	bc := bshunter.Server.GetChain()
	block, err := bc.BlockByHeight(blockHeight)
	btcdLog.Debug("test", "block", block, "err", err)

	params := bshunter.Server.GetChainParams()
	blockHeader := block.MsgBlock().Header
	blockHash := block.Hash().String()

	txns := block.Transactions()

	btcdLog.Info(blockHeight, len(txns), bshunter.CacheAddCnt, bshunter.CacheDeleteCnt)

	rawTxns := make([]btcjson.BSHunterTxResult, len(txns))
	for i, tx := range txns {
		rawTxn, _ := createBSHunterTxResultWithCache(params, tx.MsgTx(),
			tx.Hash().String(), &blockHeader, blockHash,
			blockHeight, blockHeight, bshunter.Server.GetTxIndex(), bshunter.Server.GetDb(), bshunter)
		rawTxns[i] = *rawTxn

		// Cache
		bshunter.CacheAddTxResult(rawTxn)

		// linshi
		for vinIndex, oneIn := range rawTxn.Vin {
			if oneIn.VoutContent.ScriptPubKey.Type == theType {
				btcdLog.Info("Find!!!", rawTxn)
				return vinIndex
			}
		}
	}
	return -1
}
func (bshunter *BSHunter) findBlockTxInTypes(blockHeight int32, theType string) int {
	bc := bshunter.Server.GetChain()
	block, err := bc.BlockByHeight(blockHeight)
	btcdLog.Debug("test", "block", block, "err", err)

	params := bshunter.Server.GetChainParams()
	blockHeader := block.MsgBlock().Header
	blockHash := block.Hash().String()

	txns := block.Transactions()

	btcdLog.Info(blockHeight, len(txns))

	rawTxns := make([]btcjson.BSHunterTxResult, len(txns))
	for i, tx := range txns {
		rawTxn, _ := createBSHunterTxResult(params, tx.MsgTx(),
			tx.Hash().String(), &blockHeader, blockHash,
			blockHeight, blockHeight, bshunter.Server.GetTxIndex(), bshunter.Server.GetDb(), bshunter)
		rawTxns[i] = *rawTxn
		// linshi
		for vinIndex, oneIn := range rawTxn.Vin {
			if oneIn.VoutContent.ScriptPubKey.Type == theType {
				return vinIndex
			}
		}
	}
	return -1
}
