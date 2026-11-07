# コードの生成の結果: U7 取り消し

計画(`code-generation-plan.md`)の Step 1〜14 をすべて終えました。ブランチは `bolt/u7-rollback`(U6 のプルリクエスト #13 が squash マージされた `main` の `16ec356` から)。実際の AWS は呼んでおらず、検証用アカウントのスタックも変えていません(U7 は新しいスタックを作りません)。`scripts/verify.sh` の U7 の確かめは、チェックポイントで人の許可を得てから動かします。push とプルリクエストはまだです。

## コミット

| コミット | 内容 | 対応 |
|---|---|---|
| `3d8f8e3` | fix: Dependabot のマージ(#11、#12)で壊れた `go.sum` の行を消し、`go mod tidy` で `go.mod` の間接依存3つ(eventstream、checksum、s3shared)の版を上げる | 前提の直し(下の「main の CI」) |
| `2e4f1f3` | test: 偽のタグのアカウントに、タグが0個のリソースを答えに出さない切り替え(`HideEmpty`)と読み戻しの遅れの切り替えを足す | Step 1 |
| `01be6af` | test: U7 の受け入れ条件のテスト8件(この時点ですべて失敗することを確かめた) | Step 2、US5.1、AC6.1.1、BR7.4 |
| `c11c541` | refactor: 実行の状態、まとまりの書き方、印を取る流れを、rollback と共有する部品(`execution.go`)に移す。振る舞いは変えない | Step 3 |
| `aff5dd8` | feat: rollback のための記録の読み込み(`OpenForRollback`)と `rollbacks` への書き込み(`RollbackRun`)、5秒ごとの保存の役の切り出し | Step 4 |
| `1b781bc` | feat: ARN を6つに分ける関数 | Step 5 |
| `f28932e` | feat: 記録から apply を戻す rollback の流れ(`Rollbacker`) | Step 6 |
| `ace9763` | feat: rollback の確認画面と結果の表示(英語と日本語) | Step 7 |
| `d5df26a` | feat: `rollback` コマンド(オプション、確認、ヘルプ) | Step 8(Step 9 は直しなしで通った) |
| `2fd6a20` | fix: 確認画面の件数を、表示する行に合わせる | Step 10 で見つけた不具合 |
| `a979c39` | test: rollback、記録、表示、コマンドの単体テスト | Step 10 |
| `411f937` | test: 1万リソースの rollback の呼び出し回数を NFR5.20 の上限と比べる | Step 11 |
| `780441b` | test: 検証のスクリプトに U7 の確かめを足す | Step 12 |
| `56c9a05` | style: staticcheck の指摘2件を直す | Step 13 |
| `6080a34` | docs: README に rollback の節、限界、権限 | Step 13 |
| `4af8662` | fix: レビューの指摘(ヘルプの文、rollback の権限のテスト、飛ばしの reason) | レビュー1回目の指摘 |

## 作ったファイルと変えたファイル(46ファイル、+6,546 / −434。`aidlc/` を除く)

| 区分 | ファイル |
|---|---|
| 新規(実装) | `internal/apply/`(`execution.go`、`rollback.go`、`rollback_target.go`、`rollback_check.go`、`rollback_worker.go`)、`internal/journal/`(`saver.go`、`rollback.go`)、`internal/awsaccess/arn.go`、`internal/cli/rollback.go`、`internal/display/rollback.go` |
| 変更(実装) | `internal/apply/`(`apply.go`、`worker.go`、`batch.go`、`target.go`、`stop.go`)、`internal/journal/`(`run.go`、`lock.go`、`journal.go`)、`internal/cli/cli.go`、`internal/display/`(`messages.go`、`display.go`)、`internal/testutil/tagaccount.go` |
| 新規(テスト) | `internal/apply/`(`rollback_test.go`、`rollback_scenario_test.go`、`large_rollback_test.go`)、`internal/journal/rollback_test.go`、`internal/awsaccess/arn_test.go`、`internal/cli/`(`acceptance_u7_test.go`、`rollback_cli_test.go`、`rollback_internal_test.go`)、`internal/display/rollback_test.go` |
| 変更(テスト) | 下の「期待を変えた既存のテスト」と、`internal/testutil/tagaccount_test.go`、`internal/rules/rules_test.go`(U7 のルールファイルのテストを足した) |
| 検証 | `test/sandbox/`(`u7-src-rules.yaml`、`u7-rename-rules.yaml`、`u7-only-remove-rules.yaml`、`u7-zero-add-rules.yaml`、`u7-cleanup-remove-rules.yaml`)、`scripts/verify.sh`、`scripts/u6check.go` |
| ドキュメント・依存 | `README.md`、`go.mod`、`go.sum` |

削除したファイルはありません。新しいパッケージは作っていません(計画どおり)。

## 主な実装の決定

- **共有の部品(`internal/apply/execution.go`):**
  - 移したもの: 実行の状態(止める役、書き込みを始めた ARN の印、放棄の印、進み具合)、`forEachRegion`(止めの誤りの操作名は引数)、`writeRegions`(2回目の Ctrl+C で記録を閉じ、`abandonWait` だけ待つ)、`settle`、`abandon`、1つのまとまりの書き方(ARN ごとの結果を返す)、`takeLock`(目的を引数に、`--force-unlock` つき)、`closingContext` と `readContext`。
  - 変えていないもの: `planUnits`、`decide`、`groupKey`、`stopper`、`writeOutcome`。U6 のテストが直接呼んでいるので、名前と引数も変えていない。`groupKey` の中身だけ `batchKey` に切り出して共有した。
- **記録(`internal/journal`):**
  - `LockHandle` に目的を足した。`OpenForRollback` は目的が `rollback` でなければ Internal を返す。
  - `Run` から5秒ごとの保存の役を `saver` に切り出し、apply と rollback が同じものを使う。
  - `RollbackRun` は1回取得したバイト列を型付きの `RunRecord` と生の JSON の両方に解析し、最上位と `rollbacks` を生の JSON のまま保って、自分の RollbackRun だけを置き換える(BR6.1)。閉じたあとの `RecordRollbackResults` は何もせず、誤りも返さない(security-design.md §4)。
- **rollback の流れ:** 1回目は判定だけで、途中で止めたら記録に触れずに印を消す。2回目は書き戻す項目を ARN の順に100個の単位に切り、単位ごとに読み直して判定し直し、単位の中で同じ形の20個のまとまりにして書く。書いたあとに読み戻し、`touchedKeys` が `beforeTags` と違えば1・2・4秒の間隔で読み直す(時計は差し替えられる。中断すれば resultUnknown)。
- **タグが0個のリソース:** 判定と読み戻しで同じ関数を使う(security-design.md §3)。タグの API が0個のタグのリソースを答えに出す場合と出さない場合の両方を、偽の AWS の切り替えで確かめた。

## テストの結果(手元、Windows)

| 確かめたこと | 結果 |
|---|---|
| `gofmt -l` | 出力なし |
| `go vet ./...`、`go vet -tags integration ./...` | 問題なし |
| `golangci-lint run`(`integration` のタグあり・なし) | 0 issues |
| `go test ./...`(Go 1.27.0) | 13パッケージすべて通る(オーケストレーターも別に実行して確かめた) |
| `go test ./...`(Go 1.26.0。CI の oldstable と同じ系列) | すべて通る |
| カバレッジ(`-coverpkg` で `cmd` と `internal` のすべて) | 96.3%(下限80%) |
| 新しいテスト | 最上位で77件(受け入れ条件8、apply 35、journal 7、awsaccess 5、display 7、cli 12、testutil 2、rules 1) |
| `govulncheck` | 外部のモジュールに該当なし。手元の Go 1.27.0 の標準ライブラリの既知の脆弱性9件だけ(1.27.2〜1.27.3 で修正) |
| `bash -n scripts/verify.sh`、`go vet scripts/u6check.go` | 通る |

大きな偽の AWS のテスト(`TestLargeRollbackU7`。1万件、17リージョン、偏ったリージョンに2,000件。NFR5.19、NFR5.20、NFR5.22):

| 場面 | GetResources(上限) | TagResources(上限) | UntagResources(上限) | rollback の時間 |
|---|---|---|---|---|
| よくある変更 | 300(300) | 600(600) | 600(600) | 0.21〜0.38 秒 |
| 値がばらばらなキー名の変更 | 300(300) | 10,000(10,000) | 10,000(10,000) | 0.48〜1.87 秒 |
| 読み戻しの遅れ(M = 21) | 321(363) | 600(600) | 600(600) | 0.23〜0.78 秒 |

- 上限は NFR5.20 の式で、置いた件数から計算した。偏ったリージョンの書き込みは付ける120回・消す120回で、設計の見積もり(W = 120)と一致した。U7 の大きなテスト3件の合計は約2〜5秒。
- 受け入れ条件のテスト8件は Step 2 で先に書いてコミットし、失敗することを確かめてから実装した(Testing Contract の custom の順)。
- `-race` は手元に cgo が無いため動かしていない。race 用の大きさ(10分の1)を、定数を一時的に変えて流し、上限以内・0.2秒以内だった(戻したあと差分なし)。CI の `race` ジョブで確かめる。
- `u6check.go` の `rollback`、`entries`、`same` は、試しの JSON で成功と失敗の両方を確かめた。

## 期待を変えた既存のテスト

- **`internal/cli/cli_test.go`(U1):** 「まだ使えないコマンド」のテストの対象を `iac` だけにした(計画どおり)。かわりに rollback の使い方の誤り3件(`--run` なし、`--run` の形、`--record-from` の形)を足した。
- **`internal/display/messages_test.go`:** 文言の一覧の確かめの対象に `rollback` の接頭辞を足した。
- **`internal/journal/journal_test.go`:** 補助関数 `idle` だけを直した(保存中のしるしが `Run` から `saver` に移ったため `r.saving` を `r.sv.saving` に)。U6 のテストの期待は1つも変えていない。
- 受け入れ条件のテスト(Step 2)の期待は変えていない。

## 計画と設計からのずれ

- **読み直しの単位:** 判定したすべての項目ではなく、書き戻す項目だけを ARN の順に100個ずつ切った。すでに決まった項目を読み直しても意味がないため。呼び出しの数は NFR5.20 の式の上限以下のまま。
- **`journal` の型名:** 契約 C10 の `RollbackRun` の名前を、書き込みを行う型に使った。もとの JSON の型 `journal.RollbackRun` は `RollbackRecord` に名前を変えた(どこからも使われていなかった)。
- **記録の `reason`:** U6 と同じく、失敗のときは `writeFailed:AccessDenied` のように誤りの種類を付ける。
- **`verify.sh`:**
  - 写しは管理用のプロファイルで、`ap-northeast-1` だけで ARN を指定して読む(U5・U6 の対象はすべてこのリージョン)。
  - 一時キューの後片付けは、`u7_queue_arn` ではなく `u7_queue_url` が空かどうかで判断する。作ったあと ARN の取得だけ失敗した場合にキューを消し忘れないため。
  - 残りの確かめ `u6_remaining` を、4つのキーを確かめる `keys_remaining` に広げた。U6 の手順 4 もこれを使う。
- **作業の進め方:** 開発の役が Step 6 のコミットで `git add internal/apply/` とフォルダーを指定した(名指しの指示に外れる)。中身は意図した apply のファイルだけだった。

## レビュー(1回目、READY)の指摘への対応(`4af8662`)

- **Minor: ルートの `--help` の古い文。** 「It only reads from AWS.」を「apply と rollback だけが AWS に書き込み、実行ごとに記録を残す」という文に直した。まだ使えないコマンドの誤りで示す使えるコマンドの一覧(`err.supported`)にも apply と rollback を足した(英語と日本語)。
- **Minor: NFR3.13 のテストが操作名を手で並べるだけ。** BR7.4 の受け入れ条件のテスト(手元の記録と S3 の記録の両方)で、rollback が偽の AWS に実際に送ったすべての呼び出しが `docs/iam/apply-policy.json` の操作に含まれることを確かめるようにした(`sts:GetCallerIdentity` は権限が要らないので除く)。`traceability.json` の NFR3.13 の対象をこのテストに変えた。手で並べたテストは補助として残した。
- **Minor: reason の書き方が2つ。** 書き込みのときになくなったリソースの reason が `resourceNotFound:NotFound` になっていたのを、最初の読み取りでなくなったものと同じ `resourceNotFound` にした(誤りの種類を付けるのは failed だけ)。契約 C4 の注に、failed の reason に `:<誤りの種類>` が付くことと、skipped には付けないことを書いた。
- **Minor: U7 と関係のない依存の修復(`3d8f8e3`)。** 下の「main の CI」のとおり。コードは変えず、プルリクエストの説明に書く。`main` を先に直すかは人が決める。
- 直したあと、`gofmt`、`go vet`、golangci-lint(0 issues)、`internal/apply`・`internal/cli`・`internal/display` のテストが通ることを確かめた。

## main の CI

- `main` の CI は、Dependabot の #11(`16ec356` の2つ前)から失敗している。kms の更新のマージで `go.sum` に壊れた行が入り、`go.mod` に古い間接依存の版が残ったため、どの go のコマンドも動かない(modcheck、build、test、lint、coverage、vuln のすべてのジョブが失敗)。
- このブランチの最初のコミット `3d8f8e3` で直した。U7 のプルリクエストがマージされるまで `main` は赤のままになる。

## チェックポイントの検証(2026-10-31)

- 人がサインインし直したあと、検証コマンド(`scripts/verify.sh`)を1回動かし、`verify: OK`(終了コード0、02:16〜02:27 UTC、検証済みとして記録)。スタックは変えていない(U6 の確認用のスタックとプロファイル `tag-refactor-apply` をそのまま使った)。
- U7 の手順は、apply の記録を rollback で戻し、スナップショットと同じタグに戻ること(`aws:` のキーを除く)を確かめた。出力の最後の部分の例:
  - 1件の apply を戻す rollback(`20261031T022608Z-d28d`)が `completed`、restored 1 で、スナップショットと同じタグになった。
  - 一時キュー `tag-refactor-u7-zero-<乱数>` を作り、タグのないキューが plan に集まることを確かめたうえで、必須タグを足す apply(5件)を戻す rollback が `completed`、restored 5。もう一度の rollback は restored 0、already restored 5(差分0、NFR4.2・NFR4.3)。
  - 一時キューと U2 の確認用のキューは最後に消した。
- 測った値(NFR5.21。標本が少ないものは最大を L95 とした):
  - L95: `tag:GetResources` 64 ms(標本20)、`tag:UntagResources` 200 ms(標本5)、`s3:PutObject` 101 ms(標本3)、`s3:GetObject` 40 ms(標本2)。再試行は32回中0回。
  - 読み直しの単位は実測 1.7、R = 20 のとき M = 6.7。
  - 見積もり: よくある変更 3.2 分(上限30分)。値がばらばらな場合 10.7 分(記録だけで、判定しない)。
  - **見積もりの限界:** 最後の保存の時間は、11,169 バイトの1回の実測(101 ms)を 12 MB に大きさで引き延ばした 108.5 秒を使っており、見積もりの多くを占める。実際の S3 の保存は大きさに比例しないので、見積もりは大きめに出ている。標本も少ない。それでも、よくある変更の見積もりは上限の約11%。
- **人の確認:** 3回目の apply の S3 の記録 `20261031T022445Z-3713` を、PowerShell で `--yes` なし・`--lang ja` で rollback し、取り消し済みの記録の警告と日本語の結果を確かめる。人が確かめ、U7 のチェックポイントを承認した(2026-10-31)。
- 検証コマンドの出力は約17 KB で、記録に残るのは最後の部分だけ。

## まだ確かめていないこと

- **CI:** `-race`(とくに `TestResultsAndPeriodicSavesAtOnce`)、3つの OS、`cfn-lint` は、プルリクエストの CI で確かめる。