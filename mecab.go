package main

import (
	"log"
	"strings"

	"github.com/shogo82148/go-mecab"
)

func ExtractPlaceName(tagger mecab.MeCab, text string) []string {
	/*
		MeCabによる地名抽出(Neologd使用)
	*/
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
