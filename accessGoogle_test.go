package main

import (
	"bufio"
	"encoding/gob"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"
)

// var (
// 	freeze      sync.Mutex
// 	freezeAPI   bool
// 	dictPath    string
// 	geoDictSync sync.Map
// )

func TestAccessGoogleGeocodingAPI(t *testing.T) {
	/*
		シリアライズされた地名辞書をデコードする
	*/
	// ファイル読み込み
	dictPath = "./geo-dict.bin"
	file, err := os.Open(dictPath)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	// 地名と緯度経度の変換辞書gobをデコード
	dec := gob.NewDecoder(file)
	dec.Decode(&geoDict)
	for key, value := range geoDict {
		geoDictSync.Store(key, value)
	}
	defer DumpDict()

	var rec = make(chan GeoTweet) // 出力用のチャネル
	// var done = make(chan string)  // 終了通知用のチャネル

	// go func() {
	// 	isFinished := false
	// 	for {
	// 		select {
	// 		case gt := <-rec:
	// 			if (gt.Coordinate[0] == 35.6506145) && (gt.Coordinate[1] == 139.5406936) {
	// 				t.Log("PASS")
	// 			} else {
	// 				t.Fatal("invalid response")
	// 			}
	// 			done <- "Done"
	// 		case <-done:
	// 			isFinished = true
	// 		}
	// 		if isFinished == true {
	// 			fmt.Println("finish")
	// 			break
	// 		}
	// 	}
	// }()
	freezeAPI = false
	// APIKeyの読み込み
	fp, err := os.Open("./api/api-key.txt")
	if err != nil {
		panic(err)
	}
	defer fp.Close()
	// APIキーを読み込む
	keyScanner := bufio.NewScanner(fp)
	var apiKeys []string
	for keyScanner.Scan() {
		apiKeys = append(apiKeys, keyScanner.Text())
	}
	key := apiKeys[1]
	t.Log("APIKey: %s\n", key)
	var gt GeoTweet
	gt.ScreenName = "Asamin"
	gt.Coordinate = [2]float64{0, 0}
	gt.Created_at = 111111111
	gt.NowFlag = true
	wg := sync.WaitGroup{}
	wg.Add(1)
	t.Log("Accessing API")
	fmt.Println("調布test")
	go AccessGoogleGeocodingAPI(gt, "調布", key, rec, &wg)
	resp := <-rec
	if (resp.Coordinate[0] == 35.6506145) && (resp.Coordinate[1] == 139.5406936) {
		t.Log("Chofu PASS")
	} else {
		t.Errorf("調布 is Not OK. invalid response lat: %f\tlng:%f\n", resp.Coordinate[0], resp.Coordinate[1])
	}
	fmt.Println("調布test done")
	wg.Add(1)
	fmt.Println("グアンタナモ")
	go AccessGoogleGeocodingAPI(gt, "グアンタナモ", key, rec, &wg)
	// r := <-rec
	fmt.Println("waiting 5 sec")
	time.Sleep(5 * time.Second)
	fmt.Println("restart")

	latlng, _ := geoDictSync.Load("グアンタナモ")
	c := latlng.([2]float64)
	if (c[0] == 0) && (c[1] == 0) {
		t.Log("日本の外はOK")
		fmt.Println("OK")
	} else {
		t.Errorf("グアンタナモ is Not OK. invalid response lat: %f\tlng:%f\n", c[0], c[1])
		fmt.Println("Error")
	}
}
