package main

import (
	"log"
	"strings"
	"sync"
	"time"

	"github.com/shogo82148/go-mecab"
)

func Extract(p string, key string, tagger mecab.MeCab, rec chan GeoTweet, wg *sync.WaitGroup) {
	defer wg.Done()
	// load Data
	snSlice := strings.Split(p, "/")
	ScreenName := strings.Replace(snSlice[len(snSlice)-1], ".zip", "", 1)
	// zipファイルからtweetの情報を抽出
	tweets, err := unzip(p)
	if err != nil {
		// failed to unzip
		log.Println(err)
		return
	}
	parseTweet(ScreenName, tweets, key, tagger, rec)
	return
}

func parseTweet(ScreenName string, tweets []Tweet, key string, tagger mecab.MeCab, rec chan GeoTweet) {
	log.Println("抽出開始 ScreenName: " + ScreenName)
	for _, t := range tweets {
		// ツイート本文から改行をのぞいておく
		tweetText := strings.Replace(t.Text, "\n", "", -1)
		twText := strings.Replace(tweetText, "\r", "", -1)
		if (t.RT == true) || (strings.Contains(twText, "RT")) {
			// リツイートは無視
			continue
		}
		created_at := t.Created_at
		format := "Mon Jan 2 15:04:05 +0000 2006"
		utctime, err := time.Parse(format, created_at)
		if err != nil {
			log.Println("Error: failed to parse time")
			continue
		}
		unix_utctime := utctime.Unix()
		// Represents the geographic location of this Tweet as reported by the user or client application
		// Swarm等で取得された場所
		lnglat := t.Coordinates.Coordinates
		var coordinate = [2]float64{lnglat[1], lnglat[0]}
		// ユーザがじぶんでつけた位置情報
		coordinatesBox := t.Place.BoundingBox.Coordinates
		lat := 0.0
		lng := 0.0
		for _, cB := range coordinatesBox[0] {
			lat += cB[1]
			lng += cB[0]
		}
		// 四つの点の重心をとる
		if lat != 0 && lng != 0 {
			coordinate[0] = lat / 4
			coordinate[1] = lng / 4
		}
		// 出力用の変数gt
		var gt GeoTweet
		// gtにスクリーンネーム, 緯度経度, 時刻などを入力していく
		gt.ScreenName = ScreenName
		gt.Coordinate = coordinate
		gt.Created_at = unix_utctime
		gt.UTCTime = created_at
		gt.Text = twText
		gt.IsReal = false
		if coordinate[0] == 0 && coordinate[1] == 0 {
			// 位置情報なしのツイートの時は地名含まれる地名から居場所を推測
			eg := ExtractPlaceName(tagger, twText)
			gt.NowFlag = (strings.Contains(twText, "I'm at")) || (strings.Contains(tweetText, "なう"))
			// 抽出された地名をそれぞれ緯度経度に変換する
			for _, placename := range eg {
				// 辞書にあるか調べる
				result, ok := geoDictSync.Load(placename)
				nilarr := [2]float64{0, 0}
				switch {
				case ok != true:
					// APIを使用
					if freezeAPI == true {
						// API制限にすでに達している場合
						for {
							log.Println("WARN: API制限にすでに達しています。120秒後に再確認します。  placename: " + placename)
							time.Sleep(120 * time.Second)
							if freezeAPI == false {
								break
							}
						}
					}
					AccessGoogleGeocodingAPI(gt, placename, key, rec)
				case result.([2]float64) != nilarr:
					// 辞書の中に登録されていて、かつnil値でなけば、出力チャンネルに投げる
					if r, ok := result.([2]float64); ok {
						gt.Coordinate = r
						gt.PlaceName = placename
						rec <- gt
					}
				}
			}
		} else {
			// 位置情報つきツイート
			gt.Coordinate = coordinate
			gt.NowFlag = true
			gt.PlaceName = t.Place.FullName
			gt.IsReal = true // 位置情報つきツイートを示す
			rec <- gt
		}
	}
	log.Println("抽出完了 ScreenName: " + ScreenName)
	return
}
