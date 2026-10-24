# コードの要約: U4 変更案

計画(`code-generation-plan.md`)の Step 1〜15 をすべて終え、コードのレビューの指摘も直しました。ブランチは `bolt/u4-plan`(`main` の `3aa4d95` の後の10件のコミット)。まだ push していません。

## コミット

| コミット | 内容 | 計画の手順 |
|---|---|---|
| `71e3d4e` | test: U4 の受け入れ条件のテスト15件(実装より先に単独でコミットし、すべて失敗することを確かめた) | Step 1、Step 2 |
| `1ac3999` | feat: 4つの形式、保存した変更案、日本語を含む plan の完成と、U1〜U3 のテストの期待の直し | Step 3〜Step 10 |
| `387906c` | test: U4 の単体テストと期待のファイル | Step 11 |
| `53da198` | test: 4つの形式の件数の一致、変更案の読み込みのファジング、性能 | Step 12 |
| `7488c4b` | test: 検証用アカウントでの plan の形式と保存した変更案の結合テストと `verify.sh` | Step 13 |
| `2c7599d` | docs: README(U3 の `[` と `{` のエスケープの記述の直しを含む) | Step 15 |
| `d85f87c` | refactor: 使っていなかった誤りの理由を消す | Step 14 の補い |
| `9e37819` | fix: 終了コードの規則を `Plan.ExitCode` の1か所に、読み込んだ変更案の空の演算子を出さない、英語の要約の単数形 | レビューの指摘 |
| `4db3997` | feat: 保存した変更案の読み込みの誤りを日本語でも出す(誤りに文言の識別子を持たせる) | レビューの指摘 |
| `385d8e9` | feat: `rules suggest` の下書きのコメントを `--lang ja` で日本語にする(人の判断 A) | レビューの指摘、BR3.9 |
| `f27c459` | fix: `verify.sh` の U3 の確認を `--lang en` で動かす。日本語の Windows では下書きのコメントが日本語になり、英語の見出しの確認が失敗したため | チェックポイントの検証、BR3.8 |

## 作ったファイルと変えたファイル

- **追加:**
  - `cmd/tag-refactor/` の `signals_unix.go`、`signals_windows.go`
  - `internal/fileout/fileout.go`
  - `internal/planner/` の `summary.go`、`saved.go`、`safejson.go`、`jsonwrite.go`
  - `internal/display/` の `lang.go`、`lang_windows.go`、`lang_other.go`、`color.go`、`width.go`、`width_table.go`、`gen_width.go`(`//go:build ignore`)、`messages.go`、`markdown.go`、`planout.go`、`scanout.go`
  - テストと期待のファイル:
    - `internal/cli/` の `acceptance_u4_test.go`、`options_u4_test.go`、`interrupt_test.go`、`main_test.go`、`integration_u4_test.go`
    - `internal/planner/` の `saved_test.go`、`safejson_test.go`、`fuzz_test.go`、`performance_test.go`、`race_test.go`、`norace_test.go` と、`testdata/fuzz/FuzzLoadPlan/` の種9件
    - `internal/display/` の `golden_test.go`、`markdown_test.go`、`color_test.go`、`lang_test.go`、`lang_windows_test.go`、`width_test.go`、`messages_test.go`、`counts_test.go`、`race_test.go`、`norace_test.go` と、`testdata/plan/`、`testdata/scan/` の期待のファイル
    - `internal/fileout/fileout_test.go`、`cmd/tag-refactor/signals_unix_test.go`
- **変更:**
  - `README.md`、`scripts/verify.sh`
  - `cmd/tag-refactor/main.go`
  - `internal/errkind/errkind.go`
  - `internal/planner/planner.go`
  - `internal/display/` の `display.go`、`results.go`、`progress.go`、`draft.go`
  - `internal/cli/` の `cli.go`、`commands.go`、`messages.go`、`suggest.go`
  - それぞれのテストと、U1〜U3 の受け入れ条件のテスト
- **削除:** なし。U3 の `writeDraftFileWith` は `internal/fileout` に移した。

## 主な実装の決定

- **変更の作り方(`planner.go`):** 値の別名を当ててから比べるキー名の変更と衝突、`mergesIntoExisting` が true なら書かずに消すだけ、値の変更は名前の変更のあと、必須タグは付いていないときだけ正しい値で付ける、消すキー、`aws:` の除外(大文字・小文字を区別しない)、タグを変更できない種類を飛ばす、決まった順と `changeId`、変更後のタグを求める公開の関数(U6 も使う)。影響分析は変更が1つ以上のときだけ呼ぶ。
- **集計(`summary.go`):** 種類ごとのまとめる単位で一覧の行を作り、4つの形式が同じ集計を使う。終了コードは、不完全なら1、変更・衝突・値が未指定・影響のどれかがあれば2、消す候補だけなら0。
- **保存と読み込み(`saved.go`、`jsonwrite.go`、`safejson.go`):** 契約 C2 の型(U4 で足した項目を含む)。`WritePlan` は配列の項目を1つずつ書く。`SafeJSONWriter` は DEL、C1 の制御文字、書式の文字をエスケープし、`Write` の切れ目をまたぐ文字も正しく扱う。保存と標準出力の JSON の両方がこれを通る。`LoadPlan` は 256 MiB、UTF-8 の BOM、UTF-16 の BOM の案内、機能設計 BR4.4 の確かめ、印の求め直し。
- **書き出し(`fileout`):** 試しの作成、書く直前の一時ファイル(Unix は 0600)、大きさの上限、ハードリンクで名前を付ける、閉じてから消す後片付け。`plan --out` と `rules suggest --output` が使う。
- **表示(`display`):**
  - 文言の一覧(英語と日本語)。
  - 言語の決め方(`--lang`、環境変数、Windows の表示言語)。
  - 色の判定(4つの条件をストリームごとに確かめる。Windows の仮想端末の処理は終了時に戻す)。
  - 端末での幅(Unicode 17.0.0 の東アジアの文字幅の表)と、3桁区切り(`FormatCount`)。
  - Markdown のエスケープ(データはコードスパンにだけ入れる)。
- **Ctrl+C(`cmd/tag-refactor`、`cli`):** `signal.NotifyContext` で受け取り、外側の `context` で「Interrupted. Nothing was saved.」と終了コード1にする。

## テストの結果(手元、Windows)

| 確認 | 結果 |
|---|---|
| `go test ./...` | 11パッケージすべて成功。U4 の受け入れ条件15件と、U1〜U3 の受け入れ条件も成功 |
| カバレッジ(文単位、`cmd` と `internal`) | 97.1%(レビューの指摘を直したあと。下限80%)。直す前は 97.2%(planner 94.5%、display 94.0%、fileout 86.1%) |
| `gofmt -l`、`go vet`(通常、`-tags integration`、`-tags race`)、`golangci-lint run`(Windows と `GOOS=linux`) | 差分なし、指摘なし |
| NFR1.13 変更案の作成(1万リソース) | 36 ミリ秒(上限2秒) |
| NFR1.14 各形式の出力 / 保存 | 最大34 ミリ秒 / 81 ミリ秒(上限3秒) |
| NFR1.15 読み込み | 78 ミリ秒、メモリの増えはファイルの6.8倍(上限3秒、10倍)。保存したファイルは 5.1 MiB(変更のあるリソースだけを保存するため、見積もりの約20 MiB より小さい) |
| NFR10.6 件数の一致 | 乱数の種を固定した200通りで、4つの形式の件数が一致 |
| ファジング `FuzzLoadPlan`(手元、5分) | 2026-10-24T11:24:20Z〜11:29:38Z、13,164回、新しい入力の候補94件、失敗する入力は0件。毎回ファイルを書いて読むため、1秒あたりの回数は少ない |
| `scripts/verify.sh` | `bash -n` の構文の確認だけ。検証用アカウントでの実行はチェックポイントで行う |
| `-race` | 手元に C のコンパイラがなく未実行。CI の `race` ジョブで確かめる |

## 作りながら確かめたこと

- `json.Marshal` は、DEL、C1 の制御文字、書式の文字(U+00AD、U+200B、U+202E、U+FEFF、U+E0001)をそのまま出す。U+001B、U+2028、`<`、`&` はエスケープする。後処理が要ることを確かめた。
- `GetUserPreferredUILanguages` は、手元では `ja-JP` を返した。CI の Windows のランナーの値は、テストのログで確かめる。
- 幅の表の URL(Unicode 17.0.0 の `EastAsianWidth.txt`)は 17.0.0 のファイルを返した。SHA-256 を表のファイルの先頭に書いた。

## 計画と設計からのずれ

- **`rules suggest` の下書きのコメントと `internal/rules`:** 最初は英語のままにしたが、レビューで機能設計 BR3.9 と食い違うと指摘され、人の判断(A)で `--lang ja` のときは日本語にした。言語は `rules.SuggestInput.Comments`(`rules.DraftComments`。nil なら英語)で渡し、日本語の文言は `display` の一覧から `cli` が組み立てる。`rules` は `display` に依存しない。NFR の設計の「`internal/rules` は変えない」からは外れる。64 KiB の判定は、その言語のコメントを含めた大きさで行う。英語が既定のときの出力は変わらない。
- **英語のまま残る文言:** `display` と `cli` の文言は一覧に移し、エラーの2行の枠は日本語になる。保存した変更案の読み込みの誤りは、文言の識別子で返して日本語にする(`planner/fileerrors.go`)。次の詳細は、ほかのパッケージが作るため英語のまま(機能設計 BR3.9 に例外として記録)。
  - ルールファイルの誤りの詳細
  - AWS へのアクセスの詳細
  - 影響分析の読めなかった理由
  - 一度もタグが付いていないリソースの検出の理由

  `--debug` の行と JSON も英語。
- **保存と JSON に書かない項目:** `unreadPolicies[].policyRef` と `impactFindings[].conditionOperator` は契約 C2 にないため書かない(NFR8.19)。読み込んだ変更案には、この2つがない。
- **scan の JSON に足した項目:** `variantGroups[]` に `normalizedKey` と `abbreviationOnly` を足した(項目を足すだけ)。`untaggedDiscoveryReason` は一部だけ探せたときも出す。ルールファイルがないときは `missingRequired` を出さない。
- **要約の行:** `Summary: N to change · N conflicts · N affected · N undecided · N value not set · N remove candidates · N IaC-managed` の形に広げた(件数が1のときは `1 conflict` などの単数形)。変更が要らないときも、消す候補があれば件数を足す。
- **差分の最後の行:** 種類ごとの件数を読み取れるよう、`Changes: N resources (rename key … · remove …)` の1行を足した(NFR10.6 のため)。
- **Markdown の並び:** 要約で終えるため、リソースごとの `<details>` は変更の節のすぐあとに置いた。値が未指定のリソースも `<details>` に入れた。
- **標準エラーの行:** 一覧と差分では、U1 のアカウントの行に加えて、変更案の作成日時の行を出す。
- **`rules suggest --output` の上限:** 64 MiB にした(U3 は 64 KiB を超える下書きも出す仕様のため)。
- **作成日時:** `plan --out` の作成日時は秒で切り、RFC3339Nano の形で書く。
- **結合テスト:** `TAG_REFACTOR_SANDBOX_ACCOUNT_ID` も読み、保存した変更案のアカウントを確かめる。なければ飛ばし、`verify.sh` では失敗として扱う。
- **U1〜U3 のテストの期待の直し(確かめる振る舞いは変えていない):**
  - 3桁区切り: 性能のテスト(U1、U2)と、U2 の結合テストの件数の読み取り。
  - 要約の行: U1 の要約の照合。
  - 消す候補が出るようになった: U2 の AC1.1.5、キーの規則のない変更案のテスト。
  - plan が values、required、remove を当てるようになった:
    - U3 の AC2.1.2 は「5 to change」になった。
    - 終了コード2になったテストを直した。
    - U1 の「not applied」の警告の期待をやめた。
  - 「この版では使えない」のオプションをやめた: 値の誤りのテストに入れ替えた。
  - 衝突の比べ方と行のまとめ方: `(merge into existing)` の表示を消した。
  - 色: 進み具合のテストは色を外してから比べる。
  - 文言の一覧: 終了コードの意味を文言の識別子で比べる。
  - 言語を固定した: `internal/cli` に `TestMain`(`LANG=C`)を足した。
  - U3 の書き出しのテストは、`internal/fileout` に移した。
- **作業中のこと:** ファイルを書く道具が、逆斜線と u と16進の並びを実際の文字に変えることがあった。`safejson.go` のコメントで起きたものは `387906c` で直し、テストの期待値は文字の番号から組み立てる形にした。ほかのファイルに生の見えない文字がないことを確かめた。

## まだ確かめていないこと

- `scripts/verify.sh` の U4 の確認を、検証用アカウントで動かすこと(チェックポイントの検証コマンドで確かめる)。
- 手元の Windows の PowerShell で `--lang ja` の表示と列の揃え、色、`NO_COLOR`、Markdown の貼り付けを確かめること(チェックポイントで人が確かめる)。
- `-race` と、CI の Windows のランナーでの表示言語の値(CI で確かめる)。
