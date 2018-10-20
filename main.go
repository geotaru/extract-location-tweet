package main

import (
	"bufio"
	"encoding/json"
	"fmt" // for profile
	"log"
	"net/http"         // for profile
	_ "net/http/pprof" // for profile
	"os"
	"path/filepath"
	"sync"

	"github.com/shogo82148/go-mecab"
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
	geoDictSync sync.Map
	freezeAPI   = false
)

func main() {

	go func() {
		fmt.Println(http.ListenAndServe("localhost:6060", nil))
	}()
	// オプションをロード
	inputDir, outputDir, convertDict, mecabDict := FlagParser()
	// 辞書のロード
	geoDictSync = LoadLocationDict(convertDict)
	defer DumpDict(convertDict)
	// API Keyの読み込み
	fp, err := os.Open("./api/api-key.txt")
	if err != nil {
		log.Panic("ファイルを開けませんでした: api key path")
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

	// MeCabの準備
	tagger, err := model.NewMeCab()
	if err != nil {
		panic(err)
	}
	defer tagger.Destroy()

	var rec = make(chan GeoTweet, 10000) // 出力用のチャネル
	wg := &sync.WaitGroup{}              // ファイルを読み込み終わるの検知するためのWaitGroup
	// 指定したディレクトリの下のすべてのファイルに対してTweetのデータを抜き取る
	ferr := filepath.Walk(inputDir,
		func(path string, info os.FileInfo, err error) error {
			if info.IsDir() {
				return nil
			}
			wg.Add(1)
			go Extract(path, key, tagger, rec, wg)
			return nil
		})
	if ferr != nil {
		log.Println("filepath error")
	}

	wt := &sync.WaitGroup{}
	wt.Add(1)
	go HandleOutput(rec, outputDir, convertDict, wt) // 変換結果を出力する
	wg.Wait()                                        // ファイル読み込み終了を待つ
	log.Println("ファイルの探索が終了しました")
	close(rec)
	wt.Wait() // 出力終了を待つ
	log.Println("プログラム終了")
	return
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
