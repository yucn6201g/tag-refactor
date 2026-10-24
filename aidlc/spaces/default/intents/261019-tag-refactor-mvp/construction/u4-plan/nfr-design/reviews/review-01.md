## Review

**Verdict:** READY
**Reviewer:** aidlc-architecture-reviewer-agent
**Date:** 2026-10-24T06:37:29Z
**Iteration:** 1

### Prior findings

なし

### New findings

| Severity | Location | Finding | Required action |
|---|---|---|---|
| medium | security-design.md §3「書き方」2、nfr-design-questions.md Q2 A、logical-components.md 性能の設計「保存」 | 「`Encoder` で一時ファイルに少しずつ書く、メモリは小さいまま」は Go の実際の動きと違う。`json.Encoder.Encode` は値全体を内部の `encodeState` に組み立ててから `w.Write` を1回だけ呼ぶ(Go のソース `encoding/json/stream.go` で確認。`SetIndent` を使うと整形用のバッファがもう1つ要る)。そのため、書き込み口が上限超えを知るのは JSON 全体がメモリに載ったあとで、しかも1回の `Write` で来る。Q2 の A を選んだ理由(メモリが小さい)が成り立たない。上限を超える最悪の場合(NFR の根拠表で 500 MiB 以上)に、一番多くメモリを使う | 次のどちらかに直す。(a) 本当に流す書き方にする(トップレベルの `changes` などの配列を1要素ずつ `Encode` して `[`、`,`、`]` を自分で書く。書き込み口は要素ごとに上限を数えられる)。(b) 流さないと認めて、Q2 の理由と「メモリは小さい」の記述を直し、最悪でメモリが JSON の2倍ほど要ることを記録する。書き込み口が1回の `Write` で上限超えを判定して何も書かずに誤りを返すことも、設計に書く |
| medium | security-design.md §3「Ctrl+C(Q1)」最後の2項目、「書き方」1、§1 | NFR8.17 は「割り込みでも一時ファイルを残さない」だが、設計は一時ファイルを使い方の確かめで作って収集が終わるまで(数分)開いたまま持つ。2回目の Ctrl+C は Go の既定の動きですぐに終わるため `defer` が走らず、`.tag-refactor-plan-*.tmp` が残る(強制終了や電源断も同じ)。テストは1回目のキャンセルしか確かめず、この抜けを見つけられない。`rules suggest --output` は今のコードでは書く直前に一時ファイルを作るので、この問題は plan の新しい作りで増える | 一時ファイルを作る時点を書く直前にする。使い方の確かめは「作ってすぐ消す」試しの作成で行う(NFR3.8 の「AWS を呼ぶ前に知らせる」は保てる)。それができないなら、2回目のシグナルでも一時ファイルを消してから終わる処理(シグナルを数えて自分で終わる)にするか、残りうることを NFR8.17 の限界として明記する |
| medium | security-design.md §3「Ctrl+C(Q1)」2項目め | 「各コマンドは `context.Canceled` を返す」は今のコードと合わない。取り消された呼び出しは `awsaccess.classify` が `errkind.Error`(Kind=Internal、Detail「the call was cancelled」、Cause に元の誤り)にし、`inventory` と `discover` は `fatal.get()` を `ctx.Err()` より先に返す。さらに `runScan` は内側の `WithCancel` を持ち、他のリージョンの認証情報切れでも内側だけが取り消される。「`errors.Is(err, context.Canceled)` で Interrupted にする」と実装すると、認証情報の誤りを「Interrupted」と取り違えたり、逆に Internal の誤り(「unexpected internal error」系の文言)のまま出たりする | 判定を「入口の外側の `ctx.Err() != nil`(Run が持つ元の `context`)かつ誤りがある」に決めて設計に書く。Interrupted は内側の取り消しや認証情報の誤りより優先するか後にするかも決める。plan と rules suggest の両方で、リージョンの読み取り中、影響分析中、保存中に取り消した場合のテストを足す |
| medium | security-design.md §2 JSON の行、§3「書き方」2、logical-components.md 依存の向き | JSON の後処理(`display.writeJSON`)の置き場所と範囲が決まっていない。`--out` の保存は `planner` が `Encoder` で書くが、`planner` は `display` を使えない(`display` が `planner` を使う向きなので、逆は循環する)。そのため保存したファイルには後処理が掛からず、標準出力の JSON(C3、同じ形)とバイトが変わる(DEL、C1、U+202E が生のまま残る)。NFR8.15 は「出力」だけを対象にするが、保存した変更案を `cat` や編集ソフトで開くと同じ危険がある。また、`Encoder` に流す場合、後処理は書き込みの切れ目で UTF-8 の文字が分かれても正しく動く必要があり、BMP 外の書式の文字(U+E0001、U+E0020〜U+E007F、U+1D173 など)は `\uXXXX` 1つでは書けない(サロゲートペアが要る) | 後処理を `planner` 側(または `planner` と `display` の両方が使える場所)に置き、保存と標準出力の両方が同じ関数を通ると決める。文字の切れ目を持ち越せる `io.Writer` にするか、1回の `Write` で全体を処理すると決める。BMP 外は `\ud8xx\udcxx` の小文字のペアにすると書き、そのテストを足す |
| low | security-design.md §5 言語、traceability.json NFR9.7 | NFR9.7 の後半「Windows の端末で日本語がコードページによらず正しく出る」の設計がない。トレーサビリティは §5(言語の決め方だけ)を指している。実際には、Go の `os.File` は Windows のコンソールに `WriteConsoleW` で書くので、コードページの変更は要らない(リダイレクト時は UTF-8 のまま)。ただしそれは設計に書かれていない。`GetUserPreferredUILanguages` は `golang.org/x/sys/windows` にあり(`go doc` で確認)、依存は増えないが、フラグ(`MUI_LANGUAGE_NAME`)の指定が書かれていない | 「コンソールには Go が UTF-16 で書くのでコードページを変えない、リダイレクト先には UTF-8 のまま」と §5 に書く。`GetUserPreferredUILanguages` のフラグを書き、チェックポイントの目視確認(NFR9.7)を残す |
| low | logical-components.md 性能の設計「出力」、Q3 | 桁の揃えの設計に、(1) 色の制御の文字は幅に数えず、色を付ける前の文字列から余白を求めること、(2) 不正な UTF-8 を置き換えた U+FFFD(曖昧な幅は1)と、Sanitize が作る見える形のエスケープの文字列は、置き換えのあとの文字列で幅を測ること、(3) 幅の表の元にする Unicode の版を決める(東アジアの幅の W、F の範囲をツールの中に持つので、版と更新の手順を記録しないと古くなる)、が書かれていない | 幅の関数の入力を「置き換え済み、色なし」の文字列と決める。表の元の版を記録し、半角カナ(H=1)、全角記号(F=2)、結合文字(0)、U+FFFD のテストを足す |
| low | security-design.md §4 ファジング | `FuzzLoadPlan` の「1回が1秒を超えたら `t.Fatalf`」は、`-race` や遅い CI(Windows)で、正しい実装でも不安定に落ちうる。種の入力だけの CI 実行でも、深い入れ子の種は時間が揺れる | 上限に余裕を持たせ(例: 通常 1 秒、`race` のビルドタグでは 10 倍。U2・U3 の `race_test.go` と同じ流儀)、手元の5分ファジングだけを厳しい基準にする |
| low | logical-components.md `fileout` の API | `Prepare`/`Writer`/`Commit`/`Discard` だけでは、実装に要る点が決まっていない。(1) 上限超えの誤りの種類と文言(InvalidInput と `--region` の案内)を、書き込み口がどう返すか。(2) 既存の `writeDraftFileWith` のテストが差し替えている `link`/`rename` を、どう差し替えるか。(3) 一時ファイルの接頭辞(`.tag-refactor-draft-` と `.tag-refactor-plan-`)と、`planner` の保存関数と `Pending` のつなぎ(`planner.Save(w io.Writer, …)` か)。(4) Windows で一時ファイルを閉じてから消す順 | `Prepare` に接頭辞と上限を渡し、上限超えは `errkind` の種類つきの値を返すと決める。テスト用の差し替えの口(U3 の2引数と同じ)を残すと書く。`planner` の保存関数の形を決める |

### Validation Tool Results

| Tool | Result | Interpretation |
|---|---|---|
| `aidlc engine sensor-traceability` | pass: true(gaps、orphans、invalid なし) | 15個の NFR は traceability.json に漏れなく載っている。ただし NFR9.7 の行は、日本語の出力の半分を指していない(low の指摘) |
| `go doc golang.org/x/sys/windows.GetUserPreferredUILanguages` | 存在する | Windows の表示言語を読む関数は、新しい依存なしで使える |
| Go のソース `encoding/json/stream.go` の `Encoder.Encode` | 値全体を組み立ててから1回だけ `Write` する | 「少しずつ書く」の前提が成り立たない(medium の1つめ) |
| 既存コード(`awsaccess/invoke.go`、`inventory/*.go`、`display/terminal*.go`、`cli/suggest.go`) | 取り消しは `errkind.Error`(Internal)になる。`enableVT(w io.Writer)` は既にストリームを受け取り、元に戻す関数を返す。`planner` は `display` を使わず、`display` は `planner` を使う | Windows の仮想端末の拡張と、`planner → fileout → errkind` の向き(循環なし)は実現できる。Ctrl+C の判定は設計の記述と合わない |

### Summary

依存の向き(`planner → fileout → errkind`、`display → planner`)、`signal.NotifyContext` の二度目のシグナルの動き、`os.Link` と `Rename` による名前の付け方、JSON の構文がすべて ASCII であることを使った後処理の安全の説明、Markdown のコードスパンの区切りと型による書き間違いの防止は、実装できる形で、標準ライブラリの動きとも合っている。一方、Q2 の「少しずつ書く」は `json.Encoder` の実際の動きと違い、Ctrl+C の判定は今のエラーの流れと合わず、2回目のシグナルや強制終了では一時ファイルが残り、JSON の後処理は保存側に届かない。いずれもコード生成を止めるほどではないため、high はなく READY とするが、コード生成の前に直すことを勧める。

### Findings

| ID | Severity | Location | Finding | Required action | Status |
|---|---|---|---|---|---|
| - | - | - | No findings | No action required | Resolved |
