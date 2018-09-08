package main

import (
	"log"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/context"
	"googlemaps.github.io/maps"
)

func AccessGoogleGeocodingAPI(t GeoTweet, placename string, apiKey string, ch chan GeoTweet, wg *sync.WaitGroup) {
	/*
		地名とAPIKeyを受け取り、緯度／経度とerrorを返す
		地名をGoogle Maps Geocoding APIで緯度経度に変換する
	*/
	c, err := maps.NewClient(maps.WithAPIKey(apiKey))
	if err != nil {
		log.Println("API key が間違っている可能性があります")
	}
	r := &maps.GeocodingRequest{
		Address:    placename,
		Region:     "jp",
		Language:   "ja",
		Components: map[maps.Component]string{"country": "Japan"},
	}
	result, err := c.Geocode(context.Background(), r)
	switch {
	case err != nil:
		// API制限に引っかかったなどのエラー処理
		err_message := err.Error()
		switch {
		case strings.Contains(err_message, "OVER_QUERY_LIMIT"):
			// OVER QUERY LIMIT
			log.Println(err_message)
			if freezeAPI != true {
				freezeAPI = true
			}
			overLimit.Lock()
			time.Sleep(300 * time.Second)
			AccessGoogleGeocodingAPI(t, placename, apiKey, ch, wg)
			freezeAPI = false
			overLimit.Unlock()
			log.Println("Restart to request Google API")
		case strings.Contains(err_message, "REQUEST_DENIED"):
			log.Println(err_message)
			time.Sleep(60 * time.Second)
			AccessGoogleGeocodingAPI(t, placename, apiKey, ch, wg)
		case strings.Contains(err_message, "UNKNOWN_ERROR"):
			log.Println(err_message)
			time.Sleep(10 * time.Second)
			AccessGoogleGeocodingAPI(t, placename, apiKey, ch, wg)
		default:
			log.Println(err_message)
			time.Sleep(10 * time.Second)
			AccessGoogleGeocodingAPI(t, placename, apiKey, ch, wg)
		}
	case len(result) > 0:
		freezeAPI = false
		lat := result[0].Geometry.Location.Lat
		lng := result[0].Geometry.Location.Lng
		coordinate := [2]float64{lat, lng}
		// 辞書を更新
		log.Println("辞書の更新: 地名 " + placename)
		geoDictSync.Store(placename, coordinate)
		t.Coordinate = coordinate
		t.PlaceName = placename
		ch <- t
		wg.Done()
	case len(result) == 0:
		freezeAPI = false
		// 何もかえってこなかった status ZERO_RESULTS
		coordinate := [2]float64{0, 0}
		// 辞書を更新
		log.Println("辞書の更新, ZERO_RESULTS 地名: " + placename)
		geoDictSync.Store(placename, coordinate)
		wg.Done()
	}
	return
}

func HandleAPI(ch chan GeoTweet, apikey string, parseDone chan string, apiDone chan string, apiGT chan GT) {
	finish := false
	var wg sync.WaitGroup
FILEPARSE_FOR:
	for {
		select {
		case geotweet := <-apiGT:
			wg.Add(1)
			// API制限にすでに達している場合
			if freezeAPI == true {
				for {
					log.Println("WARN: API制限にすでに達しています。120秒後に再確認します。  placename: " + geotweet.Placename)
					time.Sleep(120 * time.Second)
					if freezeAPI == false {
						break
					}
				}
			}
			// 地名をGoogle Maps APIで変換する
			log.Println("Google APIを使用します")
			go AccessGoogleGeocodingAPI(geotweet.GT, geotweet.Placename, apikey, ch, &wg)
			time.Sleep(200 * time.Millisecond)
		case <-parseDone:
			// ファイルのパースが終了
			finish = true
		default:
			if (finish == true) && (len(apiGT) == 0) {
				log.Println("Info: Google APIへの通信終了を待っています")
				break FILEPARSE_FOR
			}
		}
	}
	wg.Wait()
	log.Println("finish accessing google api")
	// API終了通知
	apiDone <- "apiFinished"
	return
}
