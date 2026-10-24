# CI/CD の流れ: U4 変更案

U1〜U3 の CI/CD の流れ(`ci.yml` のジョブ、`ci-gate`、`race` ジョブ、関門、秘密の値、検証コマンド `scripts/verify.sh`)は、そのまま使います。U4 では AWS のリソースも IAM の権限も変えないため、確認用のスタックと例の IAM ポリシーの変更はありません。公開(GitHub Releases)も行いません。

出典: NFR の設計(`security-design.md`、`logical-components.md`)、NFR 要件(NFR1.13〜NFR1.15、NFR3.8、NFR8.15〜NFR8.19、NFR9.6、NFR9.7、NFR10.6〜NFR10.8、NFR11.7)、`infrastructure-design-questions.md` Q1、チームの進め方(§Way of Working、§Testing Posture)。

## `ci.yml`

変更はありません。U4 で足すテストは、今のジョブの中で動きます。

| 足すテスト | 動くジョブ | 補足 |
|---|---|---|
| 変更の4つの種類、衝突、値が未指定、消す候補、集計、変更後のタグ | `test`(3つの OS × 2つの版)、`race`、`coverage` | AWS を呼ばない |
| 4つの形式 × 2つの言語の期待のファイル、保存した変更案の期待のファイル(NFR9.6) | 同上 | `testdata` は `.gitattributes` の `* text=auto eol=lf` で LF に固定される。3つの OS で同じファイルと比べる |
| 4つの形式の件数の一致(乱数の種を固定、200通り。NFR10.6)、文言の一覧(NFR10.7) | 同上 | |
| 出力の守り(一覧・差分の置き換え、`SafeJSONWriter`、`mdCode`。NFR8.15、NFR8.16) | 同上 | |
| 保存(`fileout`、`WritePlan`、上限、後片付け、0600)と Ctrl+C(外側の `context` の取り消し。NFR8.17) | 同上 | 0600 の確かめは Linux と macOS だけ。Windows は既定の ACL に任せる |
| `LoadPlan` とファジングの種の入力(NFR8.18) | 同上 | `go test` は種の入力を普通のテストとして動かす。CI ではファジングそのもの(`-fuzz`)は動かさない。種は `internal/planner/testdata/fuzz/FuzzLoadPlan/` に置き、今の `**/testdata/fuzz/** -text` で守られる |
| 認証情報を書かない(NFR8.19)、AWS の操作の集まり(NFR3.8) | `test`、`race` | 偽の AWS(U2 のまま) |
| 性能(NFR1.13〜NFR1.15) | `test`、`race` | `race` のときは U2・U3 と同じ `race` のビルドタグで件数を10分の1にし、上限を10倍にする |
| 色の判定(NFR11.7)、言語の決め方(NFR9.7) | `test`(3つの OS) | 判定の関数を差し替える。Windows のランナーでは、実際の `GetUserPreferredUILanguages` を呼ぶテストも動く(ほかの OS ではビルドタグで外れる) |
| 端末での幅の表の版と `unicode.Version` の比べ | `test`(`stable` と `oldstable` の両方) | 下の「幅の表の版の比べ方」 |

- カバレッジの下限80%は変えない(NFR10.8)。

### 幅の表の版の比べ方

- 表には、元にした Unicode の版を Go の定数(`widthTableUnicodeVersion = "17.0.0"`)として持つ。テストはこの定数と `unicode.Version` を、版の番号として比べる。
- `unicode.Version` が表の版と同じか古いときは通す。CI の `oldstable` の脚(今は `go.mod` の `go 1.26.0` の系列)の Go が古い Unicode の版でも、W と F の範囲は版が上がると増えるだけなので、新しい表で困ることはない(その Go が知らない文字は、もともと未割り当ての文字として扱われる)。
- `unicode.Version` が表の版より新しいときだけ落とし、「表を作り直す」手順を誤りの文言で示す。これは `stable` の Go が新しい Unicode の版に上がったとき(多くは年に1回、Go の新しい版が出たとき)に起きる。そのときは、表を作り直す短いプルリクエストを先に出す。落とさないと、日本語や新しい絵文字の幅が黙って古いままになるため、この手間は受け入れる。

### 幅の表を作るスクリプト

- `internal/display/gen_width.go` の1つのファイルにし、先頭に `//go:build ignore` を付ける。`go generate`(`//go:generate go run gen_width.go`)で手元で動かす。ビルド、テスト、カバレッジ、lint、`go vet`、`go mod tidy` の対象にならないため、カバレッジの下限を押し下げず、依存も増えない。
- 版を書く場所は1か所にする。スクリプトは、動かした Go の `unicode.Version` から版を含む固定の URL(今は `https://www.unicode.org/Public/17.0.0/ucd/EastAsianWidth.txt`)を作り、取ってきたファイルを読んで、表と `widthTableUnicodeVersion` の定数を同じ Go のファイルに書き出す。版を手で書き換える場所はない。取ってきたファイルの SHA-256 と URL を、表のファイルの先頭のコメントに書く。同じ版で作り直せば同じ表になることを、この値で後から確かめられる。
- CI はネットワークを使わず、コミットされた表だけを使う。

## 検証のスクリプト(Q1)

`scripts/verify.sh` に、U4 の確認を足します。検証コマンド(`"C:\Program Files\Git\bin\bash.exe" -c "TAG_REFACTOR_SANDBOX_ACCOUNT_ID=663798383745 bash scripts/verify.sh"`)は変えません。

- **差し込む位置:** U3 の確認のすぐあと、U2 のタグのないキューを作る前に置く。U4 の確認は、U1 の確認用のスタックの `env` と `Env` のタグだけを使い、キューも Resource Explorer の索引も要らないため、U2 の最大30分の待ちに巻き込まれないようにする。U1 の確認用のスタックの状態は、U3 の確認が既に確かめている。
- **動かし方:** U2 と同じく `run_go_test` で `go test -mod=readonly -count=1 -tags integration -run '^TestIntegrationU4' -v ./internal/cli/...` を動かし、読み取り用のプロファイルを `TAG_REFACTOR_IT_PROFILE` で渡す。テストが飛ばされたら(`--- SKIP`)失敗にし、さらに `--- PASS: TestIntegrationU4PlanFormatsAndSavedPlan (` で始まる行(名前のあとに空白と括弧が続く行。サブテストの行は数えない)がちょうど1つあることを確かめる(U2 が `--- PASS` の数を確かめているのと同じ)。結合テストはサブテストを使わない。`-run` の名前が一致しない、ファイルの `integration` のビルドタグを付け忘れた、などで何も動かずに `verify: OK` になるのを防ぐ。
- **結合テスト `TestIntegrationU4PlanFormatsAndSavedPlan`(`internal/cli/integration_u4_test.go`):**
  1. 言語を固定する。`plan` の呼び出しには必ず `--lang en` を付け、念のため `t.Setenv` で `LC_ALL=C`、`LANG=C` にする(Windows で OS の表示言語が日本語でも、英語の行の形から件数を読めるようにする)。
  2. `test/sandbox/u2-rules.yaml` で `plan --region ap-northeast-1 --lang en --format json --out <一時フォルダのファイル>` を1回動かす(`cli.Run` をテストの中で呼ぶ)。標準出力の JSON と保存した変更案は同じ1回の収集から作られるので、件数は厳密に一致するはずとして比べる。
  3. 続けて `--format table`、`diff`、`markdown` を1回ずつ動かす。この3回は別の収集なので、件数が JSON と違ったときは、アカウントが途中で変わった可能性があるとして、1回だけやり直す。やり直しでは、`--out` の保存先を新しい名前にして JSON と `--out` を同じ1回の収集でもう一度動かし(既にあるファイルは上書きしないため)、続けて3つの形式を動かす。手順5・6では新しいほうの保存した変更案を使う。それでも違えば失敗にする。
  4. 終了コードが 2(変更あり)で、どの形式でも標準出力に案内の行(`Exit code`、次の手順、`Saved plan`)がないことを確かめる。
  5. 保存した変更案を `planner.LoadPlan` で読めること、`accountId` が検証用アカウントであることを確かめる。
  6. 4つの形式と保存した変更案から読み取った件数(総数、種類ごと、衝突、値が未指定、影響あり、判定できない)が一致することを確かめる。
  7. U1 の確認用のスタックの `env` と `Env` から、`Environment` への `renameKey` があること、`Owner` の `addRequired`(値 `platform`)があることを確かめる(U1 の確認用のスタックには `Owner` を付けていないリソースがあるため。U2 のチェックポイントの scan で `Owner` の欠落が数えられている)。
- **時間の見込み:** `plan` は1回あたり、1つのリージョンの収集と IAM の読み取りで10〜20秒ほど(U2 の測定の L95 から)。4回で1〜2分、やり直しても4分ほど。
- **一時ファイル:** 保存先は Go のテストの一時フォルダ(`t.TempDir()`)に置き、テストの終わりに Go が消す。`verify.sh` の `cleanup` の変更はない。
- 認証情報は表示しない(U2 と同じ)。R2 の今の権限で動く(新しい権限は要らない)。

## チェックポイントでの人の確認

- 手元の Windows の PowerShell で `plan --rules test/sandbox/u2-rules.yaml --region ap-northeast-1 --lang ja` を動かし、日本語が化けずに表示され、列が揃っていることを確かめる(NFR9.7、Q3)。`--format markdown` の出力をプルリクエストの下書きなどに貼り、表と `<details>` が崩れないことも確かめる。
- 同じ PowerShell で、色が付くこと(一覧の `+`、`~`、`-`、`!`、`?` の行と差分)、`$env:NO_COLOR=1` を付けた実行と、ファイルへのリダイレクト(`> out.txt` のあと、まず `Select-String -Pattern 'Changes:' out.txt` で一覧の見出しが見つかることを確かめてから、`Select-String -Pattern ([char]27) out.txt` で何も見つからない)では色の制御の文字が出ないこと、実行のあとで PowerShell の表示が崩れていないこと(端末の設定が元に戻っていること)を確かめる(NFR11.7)。
- 手元で `go test -run '^$' -fuzz FuzzLoadPlan -fuzztime 5m ./internal/planner/` を動かし、結果をチェックポイントの記録に残す(NFR8.18)。Windows の Application Control が新しく作られたテストの実行ファイルをふさいだときは、同じコマンドをもう一度動かす(U3 と同じ扱い)。失敗する入力が見つかったら、Go が `internal/planner/testdata/fuzz/FuzzLoadPlan/` に書いた入力を種としてコミットし、直してから同じ5分をもう一度動かす。

## 流れ(この Bolt)

1. `bolt/u4-plan`(`main` から作成済み)で、受け入れ条件のテスト、実装、細かい単体テストの順にコミットする。U3 の README の誤り(`[` と `{` のエスケープの書き方)の直しもこの Bolt に含める。
2. push して、`main` へのプルリクエストを作る(例 `feat: complete plan with four output formats and Japanese`)。
3. `ci-gate` が通り、U4 のチェックポイントで人が承認したら、人が squash マージする。

## 戻し方と秘密の値

- U1〜U3 と同じ(打ち消しのプルリクエスト。CI に AWS の認証情報を置かない)。U4 は確認用のスタックを変えないので、スタックの戻しはない。
