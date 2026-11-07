# コードの生成の結果: U8 IaC への反映

計画(`code-generation-plan.md`)の Step 1〜16 をすべて終えました。ブランチは `bolt/u8-iac`(U7 のプルリクエスト #14 が squash マージされた `main` の `82bb6f3` から)。実際の AWS は呼んでおらず、検証用アカウントのスタックも変えていません。`scripts/verify.sh` の U8 の確かめは、チェックポイントで人の許可を得てから動かします。push とプルリクエストはまだです。

## コミット

| コミット | 内容 | 対応 |
|---|---|---|
| `43b7c51` | build: hcl/v2 と yaml/v3、解析の計測、確認用の変更案 | Step 1 |
| `2296959` | test: `iac` の受け入れ条件のテスト(この時点で失敗することを確かめた) | Step 2 |
| `961c52e` | refactor: 名前の変更のやり直しを `fileout` に移し、利用者のファイル用の `Replace` を足す | Step 3 |
| `999c006` | feat: 反映するリソースの種類の表 | Step 4 |
| `ad8e6b7` | feat: 変更案と Terraform の state の読み込み、対象の決め方 | Step 5 |
| `134e2b1` | feat: 候補のファイルの集め方、上限、入れ子の走査 | Step 6 |
| `8696fff` | feat: テンプレートと `.tf` のタグをバイトの範囲つきで読む | Step 7 |
| `9ea78e2` | feat: 変更をメモリの上で当て、確かめた Edit にする | Step 8 |
| `8f3d1d9` | feat: 差分、git の確かめ、書き出し、流れ全体 | Step 9 |
| `c5c0f6c` | feat: `iac` コマンドと英語・日本語の表示 | Step 10、Step 11 |
| `55ef70f` | fix: タグの場所全体の差し替え、ほかの行を残す、すき間のない変更をまとめて示す | Step 12 で見つけた不具合 |
| `46e1270` | test: iac、restype、fileout、表示、コマンドの単体テスト | Step 12 |
| `a55f384` | test: 大きなテスト、一時ファイルを同じハンドルで読み直す | Step 13 |
| `50cb313` | ci: 書き換えたテンプレートの確かめ、cgo なしのテスト、検証への `iac` の追加 | Step 14 |
| `7258f2c` | build: `x/text` を GO-2026-5970 を直した版に上げる | Step 15 |
| `43b0119` | fix: 1つの大きなファイルのリソースを、探索でなくマップで引く | Step 15 |
| `06c7896` | docs: README に `iac` と hcl/v2 のライセンス | Step 15 |
| `2fcdf97` | docs: Terraform の例を何と照らしたか、何と照らしていないかを書く | Step 16 |
| `cfc09c2` | fix: 解析に時間がかかりすぎる `.tf` を外す、書く直前に通常のファイルであることを確かめる | レビュー1回目の指摘 |

## 作ったファイルと変えたファイル(63ファイル。`aidlc/` を除く)

| 区分 | ファイル |
|---|---|
| 新規(実装) | `internal/iac/`(`doc`、`iac`、`run`、`target`、`tfstate`、`walk`、`depth`、`source`、`read_yaml`、`read_json`、`read_hcl`、`changes`、`edit`、`quote`、`diff`、`git`)、`internal/restype/restype.go`、`internal/fileout/`(`replace.go`、`transient_windows.go`、`transient_other.go`)、`internal/cli/iac.go`、`internal/display/iac.go` |
| 変更(実装) | `internal/journal/store.go`、`internal/cli/`(`cli.go`、`messages.go`)、`internal/display/messages.go`、`internal/errkind/errkind.go` |
| 新規(テスト) | `internal/iac/*_test.go`、`internal/iac/testdata/`、`internal/restype/restype_test.go`、`internal/fileout/replace_test.go`、`internal/cli/`(`acceptance_u8_test.go`、`iac_internal_test.go`)、`internal/display/iac_test.go`、`internal/display/testdata/iac/`、`internal/testutil/iacfiles.go` |
| 変更(テスト) | `internal/cli/`(`cli_test.go`、`messages_test.go`)、`internal/display/messages_test.go`、`internal/rules/rules_test.go` |
| 検証と CI | `test/sandbox/u8-rules.yaml`、`scripts/verify.sh`、`scripts/u6check.go`、`.github/workflows/ci.yml` |
| ドキュメント・依存 | `README.md`、`go.mod`、`go.sum` |

削除したファイルはありません。

## 主な実装の決定

- **resourceType の書き方:** scan の書き方に合わせた。設計の `s3:bucket`、`sqs:queue`、`sns:topic` は仮の書き方で、実際は `s3`、`sqs`、`sns`。残りの15行は設計のとおり。
- **検証が読む英語の文言**(`verify.sh` と表示のテストで同じ文字列):
  - keyNotInSource の理由: `the key is not in the source; it may come from a stack tag or default_tags`
  - 反映済みの数の行: `already reflected: 4`
- **「まだ使えないコマンド」の扱い:** なくなったので、`unavailableCommand`、`ReasonOptionNotAvailable`、`err.supported`、`err.notAvailable` を消し、U1 のテストの該当の期待を直した。

## テストの結果(手元、Windows)

オーケストレーターも `gofmt`、`go vet`、golangci-lint、テストとカバレッジ、`bash -n`、`go vet scripts/u6check.go` を別に実行して、同じ結果を確かめた。

| 確かめたこと | 結果 |
|---|---|
| `gofmt -l` | 出力なし |
| `go vet ./...`、`go vet -tags integration ./...` | 問題なし |
| `golangci-lint run`(`integration` のタグあり・なし) | 0 issues |
| `go test ./...`(Go 1.27.0 と 1.26.0、`CGO_ENABLED=0`) | すべて通る |
| カバレッジ(`-coverpkg` で `cmd` と `internal` のすべて) | 96.1%(下限80%) |
| `internal/iac` 単体 / `internal/restype` 単体 | 95.1% / 100.0%(NFR10.12) |
| 新しいテスト | 最上位で190件(iac 141、fileout 16、受け入れ条件 10、cli 9、display 8、restype 5、rules 1) |
| `govulncheck` | 外部のモジュールに該当なし。手元の Go 1.27.0 の標準ライブラリの既知の脆弱性9件だけ(1.27.2〜1.27.3 で修正) |
| `bash -n scripts/verify.sh`、`go vet scripts/u6check.go` | 通る |
| CI の lint のジョブの手順を手元で試行 | 終了コード 2、`4 4 u6-fixtures.yaml`(cfn-lint は手元にないので除く) |

**解析の計測(Step 1。10 MB):**

| 解析器 | 時間 | 解析後に残るメモリ | 割り当ての合計 |
|---|---|---|---|
| yaml/v3 | 約 0.42 秒 | 151 MB(約15倍) | 246 MB |
| hclsyntax | 約 1.0 秒 | 183 MB(約18倍。30倍以下なので予算は変えない) | 1.7 GB |

NFR5.25 の見込みは、YAML 100 MB、`.tf` 80 MB、JSON 約 20 MB で、1つの流れで約14秒(JSON の分は計測せず約0.4秒と見積もった)。

**大きなテスト `TestLargeIacU8`(1万リソース、3万変更、候補3,000ファイル。NFR5.25、NFR5.26、NFR3.19):**

| 場面 | 時間(上限 60 秒) | ヒープの最大(上限 2 GB) | 開いた回数(上限 11,000) |
|---|---|---|---|
| git を差し替え | 13.7 秒 | 243 MB | 9,000 |
| 実際の git(`TAG_REFACTOR_LARGE_REAL_GIT=1`) | 15.0 秒 | 247 MB | 9,000 |

## 依存の記録(NFR9.12、NFR10.13)

| 項目 | 前 | 後 |
|---|---|---|
| `go` の行 | `1.26.0` | `1.26.0`(変わらない) |
| 直接の依存 | — | `github.com/hashicorp/hcl/v2 v2.25.0`、`go.yaml.in/yaml/v3 v3.0.6` を追加 |
| 間接の依存 | — | `agext/levenshtein v1.2.1`、`apparentlymart/go-textseg/v15`・`/v17`、`mitchellh/go-wordwrap v1.0.1`、`zclconf/go-cty v1.19.0`、`golang.org/x/text v0.39.0` を追加 |
| 実行ファイル(`-trimpath -s -w`) | 16,644,096 バイト | 18,795,520 バイト(+2.1 MB) |
| 実行ファイル(既定のビルド) | 23,965,696 バイト | 26,871,296 バイト(+2.9 MB) |

`x/text` は hcl/v2 経由で入る v0.31.0 が GO-2026-5970 に届くので、v0.39.0 に上げた。`.github/dependabot.yml` は `gomod` を対象にしているので、新しい依存も更新の対象になる。

## 期待を変えた既存のテスト

- **`internal/cli/cli_test.go`(U1):** 「まだ使えないコマンド」のテストをなくし、`iac` のテストに置き換えた(計画の Step 10)。
- **`internal/cli/messages_test.go`、`internal/display/messages_test.go`:** 文言の一覧の確かめに `iac` の文言を足し、消した文言を外した。
- **受け入れ条件(`acceptance_u8_test.go`):** Step 2 のあとに変えた期待は1か所だけ。`Owner` の行番号を `:11` から `:10` に直した(一覧が示すのは Key の行で、テストの数え違い)。

## 計画と設計からのずれ

| ずれ | 理由 |
|---|---|
| 一時ファイルの読み直しを、開き直さず同じハンドルで行う(設計 §7 は「読み直し」) | 開き直すと Windows で1ファイル約 15 ms かかり、大きなテストが全体で 55 秒になった。直して約14秒。開く回数は式より少なくなる。**原因がウイルス対策の検査だというのは推測で、測っていない。** 確かめられる範囲も少し狭まる(同期のあとに同じハンドルで読むので、ほかの開き方で読めるかは確かめない)。短い書き込みと内容の違いは、これまでどおり見つかる |
| `.tf` の入れ子の走査に、`%{if}`・`%{for}` の深さとテンプレートの部品の費用を足した(設計 §2 にない) | 下の「レビューの指摘への対応」のとおり。解析器の時間が部品の数の2乗になるため |
| 差分で、すき間のない変更をまとめて「`-` のあとに `+`」で示す | 設計のとおりだと `+` と `-` が交互に出て読みにくい |
| `.tf` のヒアドキュメント内の `${` は飛ばさず数える(設計 §2 は「飛ばす」) | パーサーがコードとして再帰するので、飛ばすとスタックを使い切る恐れがある |
| 足す要素の区切りは、最後の改行から後だけを使う | 空行まで写すと確かめの (d) で落ちる |
| マップ形の `Tags` をすべて消すと、YAML でも `Tags: {}`(設計は ` []`) | 形を変えないため |
| `walkRoot` は `WalkDir` でなく `os.ReadDir` の再帰 | 根がジャンクションでも読めるようにするため |
| CI は `go run` でなくビルドしてから `iac` を動かす | `go run` は終了コード 2 を 1 に変える |
| `Edit` に `changeId` を持たせない | 1つの Edit が複数の変更をまとめることがあり、使う場所もない |
| テスト用の差し替えの変数 `writeTextForTest` を本体に置いた | ツールの不具合が確かめで止まることを試すため |

単体テストで見つけて直した不具合は3つ。「すべて消す」が括弧を理由に常に断られていた。`Tags` のあとの空行で確かめの (d) が誤って失敗していた。1つの大きなファイルでリソース数の2乗の時間がかかっていた。

## レビュー(1回目、READY)の指摘への対応(`cfc09c2`)

判定は READY で、Major 1件と Minor 2件があった。すべて直した(記録は判定のあと)。

- **Major: `%{if}` の入れ子が深さの走査をすり抜ける。** `%{` は次の `}` で閉じるので、走査の深さが増えず、解析器の時間が2乗で延びた(1 MB で約 10 秒。4 MB は2分を超えて終わらない)。`%{if` と `%{for` の深さを数え、`%{endif}` と `%{endfor}` で戻して、256 段で外すようにした。`~` と空白の付いた形も数える。
- **同じ調べ方で見つけた、レビューにない穴:** 形を変えて32通りの入力を1・2・10 MB で測ったところ、深さと関係なく、ふつうの heredoc も爆発した(`x` の行が約50万行、1 MB で 25 秒超)。プロファイルでは `hclsyntax.meldConsecutiveStringLiteral` の時間で、隣り合う文字列の部品をつなぐたびに、後ろの部品とそれまでの文字列を写している。heredoc の1行、`$`、`%`、`$${`、`%%{` がそれぞれ部品になる。
  - 走査に費用の上限を足した。部品ごとに `2n+1` と「テンプレートの始まりからのバイト数の4分の1」を足し、ファイル全体で 20 億を超えたら外す。
  - 上限で通るのは、1つの heredoc で約1万行・1 MB まで(解析は約 1.5 秒)。ふつうのスクリプトやポリシーの heredoc は外れない(テストで確かめた)。
  - 外すときの文言を「入れ子が深すぎるか、テンプレートの部品が多すぎます」に変えた。
- **直したあとの測定(同じ入力、10 MB まで):** 爆発したすべての形が 0.1 秒以内に外れる。外れずに解析する形でいちばん遅いのは `.tf` の短い文字列の並びで 10 MB あたり約 3.6 秒(大きさに比例)。YAML と JSON の形(フロー、別名、アンカー、マージのキー、コメント、文書、キー、広い配列とオブジェクト、多くのリソース)は 10 MB で約 5 秒以内で、大きさに比例した。
- **Minor: 書く直前に通常のファイルか見ていない。** `checkBeforeWriting` で通常のファイルでなければ止めるようにし、テストを足した。所有者、ACL、ハードリンクを引き継がないことを README に書いた。
- **Minor: 同じハンドルの読み直し。** 上の「計画と設計からのずれ」に、確かめられる範囲と、原因が推測であることを書いた。原因は測っていない。

レビューが確かめきれなかった範囲も、オーケストレーターが確かめた: `ci.yml`、`verify.sh`、`u8-rules.yaml`、`u6check.go` を `cicd-pipeline.md` と読み比べ、ずれはなかった。NFR8.27 は `TestSecretsStayOutOfTheResult` が秘密の値を入れたファイルで確かめている。

## チェックポイントの検証で見つかった U7 の不具合(`rollback` の直し)

- **起きたこと(2026-11-07、1回目の検証):** U7 の手順で、必須タグを足す `apply`(5件)の2秒後の `rollback` が「戻した 4、すでに戻っている 1」で完了し、QA のキューにタグが残った。検証はここで止まり、U8 の手順は動かなかった。後片付けは警告なしで終わり、タグは残っていない。`internal/apply` はこのブランチで変えておらず、U8 の変更が原因ではない。
- **原因の見立て(推測):** `rollback` の判定は1回の読み取りを信じる。タグの API が `apply` の前のタグを答えると、書き戻さずに「すでに戻っている」とする。そのときの読み取りの答えは記録に残っておらず、直接は確かめていない。
- **直し(人が承認):** `apply` の開始から1時間以内で、その記録にまだ `rollback` がなく、項目が `applied` か `resultUnknown` のとき、読み取りの結果によらず、触ったキーをすべて書き戻す(`RollbackTarget.StaleRead`)。変更前の値を書くだけなので、読み取りが正しければタグは変わらない。
- **人に提案した案とのずれ:** 提案は「`apply` の終了から5分」だったが、記録には開始時刻しかない。契約 C4 の形を変えないよう、開始から1時間にした(`apply` の上限の30分に余裕を足した長さ)。
- **U7 の設計への追加:** U7 の BR3.2(判定)に対する例外になる。U7 の設計の文書は書き換えず、ここと README に書いた。この間の「戻した」の数には、実際には何も変わらなかったものが入りうる。
- **残る限界:** 開始の記録が読めないとき、時計が `apply` より遅れているとき、途中で落ちた `apply` の `pending`(理由なし)の項目は、これまでどおり読み取りを信じる。読み取りの遅れの長さは測っていない。

## チェックポイントの検証(2026-11-07)

検証コマンドを3回動かした。3回目で `verify: OK`(終了コード0、03:38〜03:54 UTC、検証済みとして記録)。スタックは変えていない。

| 回 | 結果 | 理由と対応 |
|---|---|---|
| 1回目(`cfc09c2` まで) | 失敗(U7 の手順) | 上の `rollback` の不具合。人の承認を得て `52163d6` で直した |
| 2回目(`52163d6` まで) | 失敗(U6 の手順) | 追加の `apply` の11秒後に作った削除の変更案が、古い読み取りで QB を拾えず(9件)、キーが残った。後片付けで消え、検証用アカウントに残っていないことを読み取りで確かめた |
| 3回目(`da2dad9` まで) | 通過 | 検証スクリプトが `apply` のあと、書いた内容が2回続けて読めるまで待つようにした(`u6_settled`、`u7_same`)。確かめの条件は緩めていない |

- **U8 の手順:** 変更案17件のうち VU6 と GU6 の2件を含むことを確かめ、`iac --write` が `u6-fixtures.yaml` の4行だけを変え、BU6R が keyNotInSource で手で直す一覧に出て、`validate-template` が通り、2回目は差分なしで `already reflected: 4`、`test/sandbox/` は変わらなかった。cfn-lint は手元にないので飛ばした(CI の lint のジョブで確かめる)。
- **`iac` の時間:** 1回目の `--write` は表示で 0 秒(秒の単位)。
- **読み取りの遅れの長さ:** スクリプトは待った秒数を出すが、記録に残るのは出力の最後の部分だけで、今回の値は残っていない。
- **製品の限界として残るもの:** `plan` も同じ API を読むので、`apply` の直後に作った変更案は、書いたばかりのリソースを見落とすことがある。`plan` は読めたものだけを対象にするので誤りにはならないが、再度 `plan` すれば拾える。README には書いていない。

## まだ確かめていないこと

- **Terraform の例は実物ではない:** `internal/iac/testdata/terraform-show-example.json` は手で作り、形を公式の文書(v1.17.x)で確かめた。各種類に `values.arn` があることは確かめていない(Registry の文書を取得できなかった)。NFR10.14 (c) はこの点で弱いまま。
- **CI と検証でだけ確かめられるもの:** `-race`(手元では race タグの型チェックだけ)、Linux と macOS(シンボリックリンクのテストを含む)、cfn-lint、`verify.sh` の U8 の手順。
- **開いた回数の上限:** 読み直しを同じハンドルで行うので、実際は式より少ない。上限はこれまでの式のままなので、減った分はテストで検出されない。
