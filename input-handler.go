package main

import (
	"archive/zip"
	"encoding/json"
	"io"
	"log"
)

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
