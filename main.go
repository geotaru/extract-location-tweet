package main

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt" // for profile
	"log"
	"net/http"         // for profile
	_ "net/http/pprof" // for profile
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/shogo82148/go-mecab"
	"golang.org/x/net/context"
	"googlemaps.github.io/maps"
)

type Tweet struct {
	Created_at  string `json:"created_at"`
	RT          bool   `json:"retweeted"`
	Text        string `json:"text"`
	Coordinates struct {
		Coordinates [2]float64 `json:"coordinates"`
	} `json:"coordinates"`
	Place struct {
		BoundingBox struct {
			Coordinates [1][4][2]float64 `json:"coordinates"`
		} `json:"bounding_box"`
		FullName string `json:"full_name"`
	} `json:"place"`
}

type GeoTweet struct {
	ScreenName string
	Coordinate [2]float64
	Created_at int64  // 作成日時(UNIX時間)
	UTCTime    string // もともとの日時(UTC)
	NowFlag    bool   // 位置情報つきのツイートまたは"なう"、"I'm at"がテキストに含まれるならtrue
	IsReal     bool   // 位置情報つきツイートならtrue
	PlaceName  string // 抽出された地名または位置情報の地名
	Text       string // つぶやきのテキスト
}

type GT struct {
	GT        GeoTweet
	Placename string
}

var (
	geoDict     map[string][2]float64
	geoDictSync sync.Map
	freezeAPI   bool
	overLimit   sync.Mutex
	tagger      mecab.MeCab
	wgmain      sync.WaitGroup
)

func main() {

	go func() {
		fmt.Println(http.ListenAndServe("localhost:6060", nil))
	}()
	// オプションをロード
	inputDir, outputDir, convertDict, mecabDict := flagParser()
	// 辞書のロード
	geoDictSync = loadLocationDict(convertDict)
	defer DumpDict(convertDict)

	var apiDone = make(chan string) // Google APIへのアクセスの終了通知用のチャネル

	var rec = make(chan GeoTweet, 10000) // 出力用のチャネル
	// 出力終了用channel
	var outputDone = make(chan string)

	// データを受け取り表示する
	go func() {
		// 出力用のGoRoutine
		isFinished := false
		ticker := time.NewTicker(1800 * time.Second) // 30分間隔のTicker
		defer ticker.Stop()
	RECEIVE_FOR:
		for {
			select {
			case gt := <-rec:
				// Gzipファイルに追記
				writeGzFile(gt, outputDir)
			case <-ticker.C:
				// 辞書の定期保存(1時間ごとに辞書を保存する)
				DumpDict(convertDict)
			case <-apiDone:
				isFinished = true
			default:
				if isFinished == true {
					if len(rec) == 0 {
						break RECEIVE_FOR
					}
				}
			}
		}
		// 出力終了
		outputDone <- "output finish"
		return
	}()

	freezeAPI = false
	// API Keyの読み込み
	fp, err := os.Open("./api/api-key.txt")
	if err != nil {
		log.Panic("ファイルを開けませんでした: api key path ./api/api-key.txt")
	}
	defer fp.Close()
	// APIキーを読み込む
	keyScanner := bufio.NewScanner(fp)
	var apiKeys []string
	for keyScanner.Scan() {
		apiKeys = append(apiKeys, keyScanner.Text())
	}
	key := apiKeys[0]

	// MeCabの準備
	model, err := mecab.NewModel(map[string]string{"dicdir": mecabDict, "output-format-type": "chasen"})

	if err != nil {
		panic(err)
	}
	defer model.Destroy()

	tagger, err := model.NewMeCab()
	if err != nil {
		panic(err)
	}
	defer tagger.Destroy()

	var parseDone = make(chan string) // ファイルのパース終了通知用のチャネル
	var apiGT chan GT = make(chan GT, 5)
	// Google API にうまくアクセスする
	go HandleAPI(rec, key, parseDone, apiDone, apiGT)

	// ファイルパスを送受信するためのchannel
	var filep chan string = make(chan string, 6)

	go func() {
		// TweetデータをパースするためのGoroutine
	FOR:
		for {
			select {
			case path, ok := <-filep:
				if !ok {
					log.Println("Info: twitterデータのパースを終了します")
					break FOR
				}
				// load Data
				snSlice := strings.Split(path, "/")
				ScreenName := strings.Replace(snSlice[len(snSlice)-1], ".zip", "", 1)
				// zipファイルからtweetの情報を抽出
				tweets, err := unzip(path)
				if err != nil {
					// failed to unzip
					log.Println(err)
					continue
				}
				wgmain.Add(1)
				go parseTweet(ScreenName, tweets, key, tagger, rec, apiGT)
			}
		}
		wgmain.Wait()
		parseDone <- "Done"
	}()

	log.Println("ファイルのサーチを開始")
	// 指定したディレクトリの下のすべてのファイルに対してTweetのデータを抜き取る
	werr := filepath.Walk(inputDir, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		// pathをチャネルに投げる
		filep <- path
		return nil
	})
	if werr != nil {
		log.Println("filepath error")
	}
	log.Println("ファイルの探索が終了しました")
	close(filep)
	// 終了通知
	log.Println("Google Maps Geocoding APIへのアクセスが終了するのを待っています")

	// 出力が終了するのをまつ
	<-outputDone
	log.Println("プログラム終了")
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

func parseTweet(ScreenName string, tweets []Tweet, key string, tagger mecab.MeCab, rec chan GeoTweet, apiGT chan GT) {
	log.Println("抽出開始 ScreenName: " + ScreenName)
	for _, t := range tweets {
		// ツイート本文から改行をのぞいておく
		tweetText := strings.Replace(t.Text, "\n", "", -1)
		if (t.RT == true) || (strings.Contains(tweetText, "RT")) {
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
		gt.Text = tweetText
		gt.IsReal = false
		if coordinate[0] == 0 && coordinate[1] == 0 {
			// 位置情報なしのツイートの時は地名含まれる地名から居場所を推測
			eg := ExtractPlaceName(tagger, tweetText)
			gt.NowFlag = (strings.Contains(tweetText, "I'm at")) || (strings.Contains(tweetText, "なう"))
			// 抽出された地名をそれぞれ緯度経度に変換する
			for _, placename := range eg {
				// 辞書にあるか調べる
				result, ok := geoDictSync.Load(placename)
				nilarr := [2]float64{0, 0}
				switch {
				case ok != true:
					// APIを使用
					var geotweet GT
					geotweet.GT = gt
					geotweet.Placename = placename
					// 地名をAPIになげる
					apiGT <- geotweet
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
			// log.Println("位置情報つきツイート検出 ScreenName: " + ScreenName)
			gt.Coordinate = coordinate
			gt.NowFlag = true
			gt.PlaceName = t.Place.FullName
			gt.IsReal = true // 位置情報つきツイートを示す
			rec <- gt
		}
	}
	log.Println("抽出完了 ScreenName: " + ScreenName)
	wgmain.Done()
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
		// バッドノウハウかもしれないが、エラーメッセージを解析する以外に方法がわからなかった
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

func ExtractPlaceName(tagger mecab.MeCab, text string) []string {
	// MeCabによる地名抽出(Neologd使用)

	var geos []string
	// 解析器の作成
	lattice, err := mecab.NewLattice()
	if err != nil {
		log.Println("Error: MeCab パースに失敗")
		log.Println(err)
		var nilarr = []string{}
		return nilarr
	}
	defer lattice.Destroy()

	lattice.SetSentence(text)
	// 解析
	terr := tagger.ParseLattice(lattice)
	if terr != nil {
		panic(err)
	}
	result := lattice.String()

	// 地名を抜き出す
	for _, row := range strings.Split(result, "\n") {
		r := strings.Split(row, ",")
		if len(r) != 1 {
			if (r[1] == "固有名詞") && (r[2] == "地域") && (r[3] == "一般") {
				rr := strings.Split(r[0], "\t")
				// 地名を配列に追加
				geos = append(geos, rr[0])
			}
		}
	}
	return geos
}

func DumpDict(convertDict string) {
	os.Rename(convertDict, convertDict+".backup")
	f, err := os.Create(convertDict)
	if err != nil {
		log.Println("Error: failed to open file  path = " + convertDict)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	var d = make(map[string][2]float64)
	geoDictSync.Range(func(key, value interface{}) bool {
		d[key.(string)] = value.([2]float64)
		return true
	})
	enc.Encode(d)
	log.Println("辞書を保存")
	return
}
