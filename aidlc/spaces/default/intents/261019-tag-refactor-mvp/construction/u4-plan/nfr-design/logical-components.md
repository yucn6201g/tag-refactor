# 論理的な部品: U4 変更案

U1〜U3 のパッケージの構成と依存の向きを保ち、U4 で足すものを示します。新しいパッケージは、ファイルの書き出しを `rules suggest` と `plan` で共有するための `internal/fileout` の1つだけです。

出典: `nfr-design-questions.md` Q1〜Q3、`security-design.md`、`nfr-requirements/`、機能設計 `functional-design/`、部品の設計 `inception/domain-design/components.md`(Planner は保存と読み込みを、Display は4つの形式と言語と色を受け持つ)、契約 C2・C3・C5・C9。

## パッケージごとの変更

| パッケージ | U4 で足すもの、変えるもの | 契約 |
|---|---|---|
| `internal/planner` | 変更の4つの種類(機能設計 BR1.1〜BR1.9)、衝突の比べ方を値の別名のあとにする、集計(`PlanSummary`、`ChangeLine`)、変更後のタグを求める関数(U6 も使う)、契約 C2 の JSON の型(U4 で足した項目を含む)、JSON の後処理(`SafeJSONWriter`。保存と標準出力の両方が使う)、保存(`WritePlan`。配列の項目を1つずつ書く)と読み込み(`LoadPlan`)、`FuzzLoadPlan` | C2、C9(変えない) |
| `internal/display` | 4つの形式(一覧、差分、JSON、Markdown)と scan の JSON・Markdown、文言の一覧(英語と日本語)、言語の決め方、色の判定と色の付け方、端末での幅、3桁区切り、Markdown のエスケープ(`mdCode`)。JSON は `planner.SafeJSONWriter` を通して書く。Windows の仮想端末の処理を標準出力にも広げる | C3、C11 |
| `internal/fileout` | 新しく作る。U3 の `cli` の `writeDraftFileWith` を移し、「書けるかの試し」「書く直前に一時ファイルを作る」「大きさを数えて書く」「ハードリンクで名前を付ける」「閉じてから消す後片付け」を1組の関数にする(`security-design.md` §3 の口) | — |
| `internal/cli` | `plan` の `--format`、`--out`、`scan` の `--format`、すべてのコマンドの `--lang` を使えるようにし、U1 の「この版では使えない」の扱いと、U3 の「values / required / remove are read but not applied」の1行を消す。使い方の確かめの順(`security-design.md` §1)。`context.Canceled` を「Interrupted」と終了コード1に変える。エラーの2行の案内を文言の一覧から選ぶ | C5 |
| `internal/rules` | 変えない。U3 の照会(`CanonicalKeyOf`、`CanonicalValue`、`IsRemoved`、`RequiredValue`)をそのまま使う | C8 |
| `internal/inventory` | 変えない。`aws:` の判定(`IsAWSReservedKey`)と、タグを変更できない種類の判定(`IsNotTaggable`)をそのまま使う | C7 |
| `cmd/tag-refactor` | `signal.NotifyContext` で Ctrl+C(Unix では SIGTERM も)を受け取り、`context` を取り消す | — |
| `README.md` | `plan` の形式と `--out`、`--lang`、色と `NO_COLOR`、PowerShell 5.1 のリダイレクトの注意 | — |

### 依存の向き(U4 で変わるところ)

| パッケージ | 使うパッケージ | 理由 |
|---|---|---|
| `planner` | `inventory`、`rules`、`errkind` | 保存は `io.Writer` に書くだけで、一時ファイルと名前の付け方は `cli` が `fileout` で行う。`display` にも `fileout` にも依存しない |
| `display` | `planner`、`inventory`、`awsaccess`、`errkind`(U1〜U3 のまま) | C3 の JSON は `planner` の型を使う |
| `fileout` | `errkind` だけ | 書き出しの誤りを種類つき(StorageFailed、InvalidInput)で返す |
| `cli` | U1〜U3 のものに加えて `fileout` | `plan --out` と `rules suggest --output` の一時ファイルと名前の付け方を `fileout` で行い、`planner.WritePlan` に書き込み口を渡す |

```go
// plan --out のつなぎ方(cli)
if err := fileout.Prepare(out, ".tag-refactor-plan-"); err != nil { return err } // AWS を呼ぶ前
// ... 収集、変更案の作成、影響分析 ...
p, err := fileout.Create(out, ".tag-refactor-plan-", planner.MaxPlanBytes)
defer p.Discard()
w := planner.NewSafeJSONWriter(p.Writer())
if err := planner.WritePlan(w, plan); err != nil { return err }
if err := w.Close(); err != nil { return err }
if err := p.Commit(); err != nil { return err }
```

## 性能の設計(NFR1.13〜NFR1.15)

- 変更案の作成: リソースを ARN の順に1回だけ走査し、リソースごとに U3 の照会(対応表の引き当て)を呼ぶ。集計は、変更を作るときに種類ごとの対応表(`ChangeLine` の鍵 → 件数)に足していき、最後に1回だけ並べ替える。消す候補もキーの対応表に数える。
- 出力: どの形式も、`bufio.Writer` に書いてから最後にまとめて出す。差分と Markdown の全件の表は、リソースごとに文字列を作り直さず、1つの書き込み口に順に書く。JSON の出力も `WritePlan` と同じく項目を1つずつ書く。
- 端末での幅(Q3): 幅の関数の入力は、表示の前の置き換えのあとで、色を付ける前の文字列とする(色の制御の文字は数えない。U+FFFD や置き換えで作った見える形の文字列も、置き換えのあとの形で数える)。余白は色を付ける前に求め、色は余白の外側に付ける。ASCII だけの文字列なら文字の数をそのまま使い、表を引くのは ASCII でない文字だけにする。幅は、結合文字(Unicode の Mn と Me。Go の `unicode` で判定)とゼロ幅の文字は0、東アジアの幅が W と F の文字は2、それ以外(半角カナの H、曖昧な幅の A、U+FFFD を含む)は1とする。W と F の範囲の表は、Go の `unicode` パッケージと同じ Unicode の版(このツールを作る Go 1.27 では 17.0.0)の `EastAsianWidth.txt` から作り、版を Go の定数として表に持つ。Go を上げて Unicode の版が新しくなったら、表を作り直す(表を作るスクリプトと版の比べ方は、インフラの設計の `cicd-pipeline.md` のとおり)。
- 保存: `WritePlan` が配列の項目を1つずつ書き、一時ファイルに直接流す(メモリに載るのは1項目だけ。Q2)。
- 読み込み: ファイルの大きさの入れ物を先に用意して1回で読み、`Decoder` で型に写す。
- テスト: NFR1.13〜NFR1.15 を、`planner` と `display` の単体テストで、性能の条件の入力から測る。`race` のビルドタグで件数を10分の1にし、上限を10倍にする(U2・U3 と同じ `race_test.go` と `norace_test.go`)。

## 移植性とテストの設計(NFR9.6、NFR9.7、NFR10.6〜NFR10.8)

- 期待のファイル: `internal/display/testdata/plan/` に、決まった入力から作った4つの形式 × 2つの言語と、保存した変更案(`planId` と `createdAt` は決まった値)を置く。`.gitattributes` で LF に固定する。期待のファイルは、テストの `-update` の旗で作り直せるようにする(作り直したら差分を人が見る)。
- 4つの形式の件数の一致(NFR10.6): 乱数の種を固定して200通りの変更案を作り、4つの形式からそれぞれの件数を読み取って比べる。一覧と差分と Markdown からは、決まった行の形から件数を読み取る小さな読み取りの関数をテストの中に持つ。
- 文言の一覧(NFR10.7): 文言の識別子ごとに英語と日本語を持つ表にし、テストで、すべての項目に両方の言語があること、`fmt` の差し込みの印(`%s`、`%d` など)の数と順が同じことを確かめる。
- 言語の決め方(NFR9.7): 環境変数を読む関数と、Windows の表示言語を読む関数を差し替えられる形にし、3つの OS の CI で同じテストを動かす。Windows では、実際の関数を呼ぶテストも動かす。
- 端末での幅: 半角カナ(1)、全角の記号(2)、漢字とかな(2)、結合文字(0)、U+FFFD(1)、曖昧な幅の文字(1)、ASCII(1)のテストを置く。

## 信頼性の設計

- `--out` の保存と Ctrl+C の後片付けは `security-design.md` §3 のとおり。保存に失敗したら、標準出力には何も出さずに終了コード1にする(機能設計 BR4.1)。
- 標準出力への書き込みが失敗したとき(パイプの先が閉じたなど)は、U1 の決まり(書き込みの誤りを終了コード1に変える)のまま。

## 障害の範囲と隔て方(U1〜U3 の表に足す)

| 部品 | 失敗したとき | 広がり |
|---|---|---|
| 保存(`fileout`) | `--out` だけが失敗し、標準出力には結果を出さない。保存先と一時ファイルは残らない | そのコマンドだけ。AWS には何も書かない |
| 読み込み(`LoadPlan`) | 誤りを種類つきで返す。U6 は書き換えを始めない | U6 の apply だけ(U4 では使う場所がない) |
| 色と言語の判定 | 判定できなければ、色なしと英語にする | 表示だけ。結果の中身は変わらない |
| Ctrl+C | 呼び出しを取り消し、後片付けをして終わる | そのコマンドだけ |
