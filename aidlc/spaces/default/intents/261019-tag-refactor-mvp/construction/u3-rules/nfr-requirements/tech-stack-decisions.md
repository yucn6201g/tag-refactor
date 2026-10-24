# 使う技術: U3 ルール

U1 と U2 の技術の選択(Go、AWS の Go 用 SDK v2、cobra、goccy/go-yaml、`golang.org/x/sync`、`golang.org/x/term`、`CGO_ENABLED=0`、GitHub Actions)は、そのまま使います。U3 で新しい依存は足しません。

出典: 機能設計 `construction/u3-rules/functional-design/`、`security-requirements.md`、`performance-requirements.md`、`nfr-requirements-questions.md` Q2・Q3。

| 分野 | 選んだもの | 理由 | 採らなかった案 |
|---|---|---|---|
| ルールファイルの読み込み | U1 と同じ goccy/go-yaml の構文木を、ツール自身が調べて写す | 行番号を誤りに付けられる。アンカーや統合キーなどをツールが拒める(U1 のまま) | 構造体への自動の写し(行番号と知らないキーの扱いが弱い) |
| 下書きの書き出し | YAML のライブラリを使わず、ツールが決まった形の文字列を組み立てる。キーと値は Go の標準の文字列の処理で二重引用符と `\uXXXX` に変える | 並び、コメント、引用の形を固定でき、OS によらず同じバイト列になる(NFR9.4)。読み戻しはテストで確かめる(NFR10.4) | goccy/go-yaml の書き出し(コメントと引用の形を細かく決めにくい) |
| 壊れにくさの確かめ | Go の標準のファジング(`testing.F`)。種の入力はテストのコードと `testdata/fuzz/` に置く | 追加の道具が要らない。CI では種の入力だけが普通のテストとして動く(NFR8.13) | 外部のファジングの道具(依存が増える) |
| 改行 | 下書きは LF(Go の標準)。読み込みは U1 のまま LF と CRLF を同じに読む | Q2 | Windows で CRLF に変える |
| 種類の対応表 | 機能設計 BR3.4 の案内には、U2 の `internal/inventory` の種類の対応表を使う。`rules` から `inventory` の表を直接使わず、表を照会する小さな関数を渡す | `rules` が `inventory` に依存しない今の向き(U1 の部品の依存の向き)を保つ | `rules` から `inventory` を import する(依存の向きが変わる) |

## 依存の決まり

- 新しい依存は足さない。`go.mod` の変更はない。
- `rules suggest` の収集は、U2 の `inventory` と `awsaccess` の読み取りの口をそのまま使う。
