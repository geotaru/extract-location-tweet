# Tweet Extractor
Twitterアカウント400万件規模の物を処理したプログラム

## Requirement
- MeCab 0.996, mecab-config
- Google maps Geocoding APIのキーを./api/api-key.txtに書き込んでおく必要がある

## Usage

inフラグの後にjsonファイルが入っているzipがあるディレクトリを指定する  
outフラグの後に出力先となるディレクトリ名を指定する  

```
./extractor -in="path/to/input/" -out="path/to/output/"
```

## Input

入力ファイルはTwitterのREST APIで取得したJSONファイルをzipで圧縮したものです。
サンプルファイルを作成してみたので参照してください

```
$ unzip -p input/sample.zip
```
