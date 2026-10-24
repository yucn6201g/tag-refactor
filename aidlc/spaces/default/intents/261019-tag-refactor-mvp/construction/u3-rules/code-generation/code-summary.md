# コードの要約: U3 ルール

計画(`code-generation-plan.md`)の Step 1〜15 をすべて終えました。ブランチは `bolt/u3-rules`(`f7a7b4e` の後の7件のコミット)。まだ push していません。

## コミット

| コミット | 内容 | 計画の手順 |
|---|---|---|
| `925ef1f` | test: U3 の受け入れ条件のテスト10件(実装より先に単独でコミットし、失敗することを確かめた) | Step 1、Step 2 |
| `e882b08` | feat: ルールファイルのすべての項目の読み込みと、`rules suggest` | Step 3〜9 |
| `b93be98` | test: U1 のテストの期待を U3 の振る舞いに合わせる | Step 10 |
| `926a02a` | test: U3 の単体テスト、ファジング、読み戻し、性能のテスト | Step 11、Step 12 |
| `64686e0` | test: `verify.sh` で検証用アカウントに対する `rules suggest` を確かめる | Step 13 |
| `52fffdc` | docs: README に `rules suggest` とルールファイルの書き方を書く | Step 15 |
| `f599b6e` | fix: `verify.sh` の読み戻しで終了コードも確かめ、ハードリンクを使えないときの下書きの書き込みをテストする | Step 13、Step 15 の補い |

Step 14(すべての確認と5分のファジング)はコミットを伴いません。結果は下の「テストの結果」に書きました。

## 作ったファイルと変えたファイル

- **追加:** `internal/rules/encoding.go`、`internal/rules/consistency.go`、`internal/rules/query.go`、`internal/rules/suggest.go`、`internal/cli/suggest.go`、`internal/display/draft.go`、`internal/rules/testdata/suggest/draft.yaml`、`internal/rules/testdata/fuzz/FuzzParse/` の種の8ファイル。テストは `internal/rules/`(encoding_test.go、load_test.go、consistency_test.go、query_test.go、suggest_test.go、fuzz_test.go、roundtrip_test.go、performance_test.go、norace_test.go、race_test.go)、`internal/cli/`(acceptance_u3_test.go、suggest_test.go、suggest_cli_test.go)、`internal/display/draft_test.go`。
- **変更:** `.gitattributes`、`README.md`、`scripts/verify.sh`、`internal/rules/`(rules.go、schema.go)、`internal/cli/`(cli.go、commands.go、messages.go)、`internal/inventory/types.go`、それぞれのテスト(`internal/rules/rules_test.go`、`internal/cli/cli_test.go`、`internal/inventory/inventory_test.go`)。
- **削除:** なし。

## 主な実装の決定

- **読み込みの口:** `Load(path, opts)` と `Parse(path, data, opts)`、`Options{TypeHint}`。文字コードの見分け(UTF-8、BOM 付きの UTF-16、BOM のない UTF-16 は誤り)と 64 KiB の判定は `Parse` の最初で行う。`Load` は 128 KiB+1 バイトまでしか読まない。
- **項目どうしの矛盾:** `consistency.go` にまとめた。別名と正しいキー名の重なり、値の別名の重なり、`remove` と `keys`・`required` の重なり、`when` の条件の書き方(`service` または `service:type`、CloudFormation の形と `unknown` は誤り)を確かめ、最初の誤りで止める。
- **照会:** `RequiredValue`(`when` を上から評価して最初に合ったもの、なければ `default`、なければ未指定)、`CanonicalValue`、`IsRemoved`、`UnifiedTags`(別名と値の別名を置き換え、消すキーを除いたタグ)、`CanonicalKeyOf`。グローバルなリソースはリージョンの条件に合わない。
- **下書き:** `Suggest(SuggestInput{Resources, KeyGroup, Preferred})`。`internal/rules` は `inventory` を読み込まず、まとめる関数を受け取る。正しいキー名は最も多く使われている綴り。値の候補は20種類までのキーだけに書く。キーと値は二重引用符と `\uXXXX`・`\UXXXXXXXX` で書く。64 KiB を超えないよう値の候補を減らす。リソースがないときは `keys: {}`。
- **`rules suggest`:** 接続を二段に分け、`--output` は一時ファイルに書いてから名前を変える。既にあるファイルは上書きしない。標準出力に書くときは表の出力と同じ口(`display/draft.go`)を使う。
- **種類の照会:** `inventory.ARNFormOf` で、ARN からルールファイルの `service:type` の形を返す。

## テストの結果(手元、Windows)

| 確認 | 結果 |
|---|---|
| `go test -mod=readonly -count=1 ./...` | 10パッケージすべて成功。U3 の4パッケージで401件(サブテストを含む)、U3 の受け入れ条件のテストは10件 |
| U1・U2 の受け入れ条件のテスト | すべて成功 |
| カバレッジ(文単位、`cmd` と `internal`) | 97.5%(下限80%) |
| `gofmt -l`、`go vet`(通常、`-tags integration`、`-tags race`)、`golangci-lint run` | 差分なし、指摘なし |
| 性能 NFR1.11(1万件、キー500個の下書き) | 約43ミリ秒(上限5秒) |
| 性能 NFR1.12(1万件の照会) | 約330ミリ秒(上限2秒) |
| ファジング `FuzzParse`(手元、5分) | 5分0秒、16,339,661回。新しい入力の候補1,461件(Go のキャッシュ)、クラッシュと1秒を超える入力は0件。`testdata` に足した入力はなし |
| `.gitattributes` | `git check-attr text` は種のファイルで `unset`、`testdata/suggest/draft.yaml` は `eol: lf` |
| `scripts/verify.sh` | `bash -n` の構文の確認だけ。検証用アカウントでの実行はチェックポイントで行う |
| `-race` | 手元に C のコンパイラがなく未実行。CI の `race` ジョブで確かめる |

- 読み戻しのテスト(乱数の種を固定)で、`\UXXXXXXXX`、NUL、1024文字を超える暗黙のキーも goccy/go-yaml で読めることを確かめた。

## 計画からのずれ

- **`verify.sh` の一時ファイル:** `mktemp` が作るファイルは既にあるため、`--output` が上書きを拒んで誤りになる。`mktemp -d` で一時フォルダを作り、その中の `tagrules.yaml` に書くようにした。フォルダは `cleanup` 関数で消す。
- **`verify.sh` の実行ファイル:** 確認の前に `go build` で実行ファイルを作る。Application Control にふさがれたときは3回まで動かし直し、3回ともふさがれたら失敗にする。`scan --rules` での読み戻しは、終了コードが 0 か 2 で、標準エラーに `Error:` がないときだけ通す。
- **ハードリンクを使えないとき:** `--output` はハードリンクで名前を付け、使えないファイルシステムでは確かめてから名前を変える。この経路では、確かめてから名前を変えるまでの間にできたファイルを置き換えることがある。README に書き、この経路(名前が空いている、既にある、名前を付けられない)を単体テストで確かめた。下書きのファイルの権限は一時ファイルのまま(Unix の 0600)で、README に書いた。
- **エスケープの範囲:** 設計の Cc・Cf・Zl・Zp に加えて、印字できないすべての文字(未割り当てと私用の文字を含む)と `[`・`{` もエスケープする。YAML がそのまま受け付けない文字と、読み込み側の括弧の数の上限への対策。読み戻せることはテストで確かめた。README の下書きの説明にも書いた(NFR 設計の `security-design.md` §3 の表には載っていない)。
- **不正な UTF-8:** キーと値の不正な UTF-8 は、まとめる前に U+FFFD に置き換える。下書きに同じキーが2つ出ないようにするため。
- **plan の1行:** 「values / required / remove are read but not applied」の行は、ルールファイルにそれらの項目があるときだけ出す。
- **設計で決まっていなかった点:** 下書きの先頭のツールの版はビルドの情報から取る(手元のビルドは `devel`)。別名が2つあって正しいキー名がないリソースでは、ファイルで先に書かれた別名の値を使う。ツールの版、アカウント、リージョン名が決まった形でないときは `unknown` と書く。
- **U1 のテストの期待を直した(確かめる振る舞いは変えていない):** `Load`・`Parse` の引数が増えたためテスト用の手助けを足した。「not used」の警告がなくなったため、その期待を消した。入れ子のテストの入力を 64 KiB 未満にした(`Parse` が先に 64 KiB の判定をするため)。`rules suggest` は「この版では使えない」コマンドではなくなったので、そのテストを使い方の誤りのテストに入れ替えた。

## まだ確かめていないこと

- `scripts/verify.sh` の U3 の確認を検証用アカウントで動かすこと(チェックポイントの検証コマンドで確かめる)。
- 手元の Windows の PowerShell で `rules suggest --output tagrules.yaml` を動かし、`scan --rules tagrules.yaml` で読めること(NFR9.5。チェックポイントで人が確かめる)。
- `race` のビルドタグのときの性能のテスト(CI の `race` ジョブで確かめる)。
