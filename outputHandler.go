package main

import (
	"compress/gzip"
	"log"
	"os"
	"strconv"
	"sync"
	"time"
)

func HandleOutput(rec chan GeoTweet, outputDir string, convertDict string, waitWrite *sync.WaitGroup) {
	ticker := time.NewTicker(1800 * time.Second) // 30分間隔のTicker
	defer ticker.Stop()
	for {
		select {
		case gt, ok := <-rec:
			if !ok {
				waitWrite.Done()
				return
			}
			waitWrite.Add(1)
			// Gzipファイルに追記
			go writeGzFile(gt, outputDir)
		case <-ticker.C:
			// 辞書の定期保存(一定時間ごとに辞書を保存する)
			DumpDict(convertDict)
		}
	}
}

func writeGzFile(gt GeoTweet, outputDir string) {
	// 書き込む内容を調整する
	// NowFlagをstringに
	nf := convertBooltoBinary(gt.NowFlag)
	ir := convertBooltoBinary(gt.IsReal)
	lat := strconv.FormatFloat(gt.Coordinate[0], 'f', -1, 64)
	lng := strconv.FormatFloat(gt.Coordinate[1], 'f', -1, 64)
	ca := strconv.FormatInt(gt.Created_at, 10)
	row := lat + "\t" + lng + "\t" + ca + "\t" + nf + "\t" + gt.ScreenName + "\t" + gt.UTCTime + "\t" + ir + "\t" + gt.PlaceName + "\t" + gt.Text + "\n"
	// write gzip file
	outputFileName := outputDir + gt.ScreenName + ".tsv.gz"
	// ファイル書き込み
	gzfile, err := os.OpenFile(outputFileName, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		//エラー処理
		log.Println("Error: ファイルに書き込めませんでした filePath: " + outputFileName)
	}
	defer gzfile.Close()
	// gzファイルに書き込み
	zw := gzip.NewWriter(gzfile)
	zw.Write([]byte(row))
	zw.Close()
	return
}

func convertBooltoBinary(b bool) string {
	// 入力のbがfalseなら0
	s := "0"
	if b == true {
		// 入力のbがtrueなら1
		s = "1"
	}
	return s
}
