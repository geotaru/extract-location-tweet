#!/usr/bin/env python
import pickle as pkl
import json


def main():
    """
    pickleとして保存された緯度経度の辞書を人間が読みやすいJSONファイルへ変換する
    """
    with open("./geo_dict.pkl", "rb") as f:
        gdict = pkl.load(f)
    with open("./geo_dict.json", "w") as fj:
        json.dump(gdict, fj, ensure_ascii=False, indent=4, sort_keys=True, separators=(',', ': '))


if __name__ == "__main__":
    main()
