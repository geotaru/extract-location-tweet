package main

import (
	"archive/zip"
	"encoding/json"
	"flag"
	"io"
	"log"
	"os"
	"sync"
)

func flagParser() (string, string, string, string) {
	/*
		Flagを読み込む
	*/
	var inputDir = flag.String("in", "./input/", "input directory")
	var outputDir = flag.String("out", "./output/", "output directory")
	var convertDict = flag.String("c", "./geo_dict.json", "dictionary path for converting place name into latitude and longitude")
	var mecabDict = flag.String("m", "", "dictionary path for MeCab")
	flag.Parse()
	return *inputDir, *outputDir, *convertDict, *mecabDict
}

func loadLocationDict(dictPath string) sync.Map {
	/*
		地名をkey，[緯度, 経度]をvalueとする辞書を読み込む
	*/
	// 辞書を読み込む
	file, err := os.Open(dictPath)
	if err != nil {
		log.Fatal("Failed to open file: " + dictPath)
	}
	defer file.Close()

	// 地名と緯度経度の変換辞書jsonをデコード
	dec := json.NewDecoder(file)
	decerr := dec.Decode(&geoDict)
	if decerr != nil {
		log.Fatal("Failed to decode json: path" + dictPath)
	}
	var gd sync.Map
	for key, value := range geoDict {
		gd.Store(key, value)
	}
	return gd
}

func unzip(filename string) ([]Tweet, error) {
	/*
		zipファイルの中身のjsonファイルから必要な情報を抜き出す
		Input: p zipファイルのパス
		Output: 構造体Tweetの配列
	*/
	// zipファイル解凍
	r, err := zip.OpenReader(filename)
	if err != nil {
		log.Println("Error: Failed to open file: " + filename)
		return nil, err
	}
	defer r.Close()
	// 最初のファイルのみ読み込む
	file := r.File[0]
	rc, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	tweets, err := Extract(rc)
	// not passed
	if err != nil {
		log.Println(err)
		return nil, err
	}
	return tweets, nil
}

func Extract(jsonfile io.Reader) ([]Tweet, error) {
	// zipファイルの中身のJSONファイルをデコード
	dec := json.NewDecoder(jsonfile)
	// 最初の'['を読み取る
	_, err := dec.Token()
	if err != nil {
		return nil, err
	}
	// 解析結果を入れる配列
	var tweets []Tweet
	// 配列の中身をDecode
	for dec.More() {
		var tw Tweet
		err := dec.Decode(&tw)
		if err != nil {
			// GC が機能してない?
			var tws []Tweet
			tweets = tws
			return nil, err
		}
		tweets = append(tweets, tw)
	}
	return tweets, nil
}
