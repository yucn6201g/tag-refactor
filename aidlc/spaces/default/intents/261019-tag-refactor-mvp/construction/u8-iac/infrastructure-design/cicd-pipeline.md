# CI/CD と検証の設計: U8 IaC への反映

U1〜U7 の CI/CD の流れ(`ci.yml` のジョブ、`ci-gate`、`race` ジョブ、関門、秘密の値、検証コマンド `scripts/verify.sh`)は、そのまま使います。U8 の `iac` は AWS を呼ばないので、検証用アカウントに新しいスタックを作らず、スタックも更新しません。`ci.yml` には2つの変更(`test` のジョブの `CGO_ENABLED=0` と、`lint` のジョブの書き換えたテンプレートの cfn-lint)を入れ、`verify.sh` の U7 の確かめのあとに、確認用のスタックから作った変更案を `test/sandbox/` の写しのテンプレートに反映する確かめを足します。

出典: NFR の設計(`security-design.md`、`logical-components.md`)、NFR 要件(NFR3.16〜NFR3.21、NFR5.25、NFR5.26、NFR6.5、NFR6.6、NFR8.27、NFR9.11、NFR9.12、NFR10.12〜NFR10.14、NFR11.10、NFR12.1)、機能設計(`rules.md`、`functional-spec.md`)、契約 C1(ルールファイル)・C2・C5、`infrastructure-design-questions.md` Q1〜Q3、U7 のインフラの設計、チームの進め方(§Way of Working、§Testing Posture)。

## `ci.yml`

ジョブは足さず、2つのジョブに手を入れます。

| 変更 | 内容 | 理由 |
|---|---|---|
| `test` のジョブに `CGO_ENABLED: "0"` を足す | 3つの OS × Go の2つの版(stable と oldstable)のテストを、配るバイナリと同じ `CGO_ENABLED=0` で動かす。`race` のジョブは別のジョブで `CGO_ENABLED: "1"` のまま | NFR9.12(新しい依存 hcl/v2 と yaml/v3 を含め、2つの版と3つの OS で `CGO_ENABLED=0` で通る)。今は `build` のジョブ(stable だけ)と `artifacts` だけが `CGO_ENABLED=0` |
| `lint` のジョブの cfn-lint のステップに、書き換えたテンプレートの確かめを足す | 元のテンプレート(`test/sandbox/*-fixtures.yaml`)の cfn-lint のあとに、`test/sandbox/` を一時フォルダーに写して git のリポジトリにし(下の「一時のリポジトリの作り方」と同じ)、テストのデータの変更案 `internal/iac/testdata/sandbox-plan.json`(AWS を呼ばずに作った、確認用のスタックの論理 ID とスタック名のタグを持つ変更案)で `go run ./cmd/tag-refactor iac --plan ... --dir <写し> --write` を動かす。終了コードは 0 と 2(手で直すものがある)だけを受け付け、ほかは失敗にする。続けて `git -C <写し> diff --numstat` で `u6-fixtures.yaml` が4行の追加と4行の削除であることを確かめ(書き換えが0件のまま元のテンプレートだけを確かめることを防ぐ)、書き換えたテンプレートを cfn-lint で確かめる | NFR3.21 の「書き換えたテンプレートで cfn-lint が通る」を、手元に cfn-lint がなくても CI で確かめる |

U8 のテストは、今のジョブの中で動きます。

| 確かめ | 動くジョブ | 注 |
|---|---|---|
| U8 の受け入れ条件と単体テスト(CloudFormation の YAML・JSON、Terraform、Edit、引用符、差分、上限と深さ、出さないもの、表示) | `test`(3つの OS × 2つの版)、`coverage` | AWS の認証情報は要らない(NFR3.16、NFR10.12) |
| git の確かめ(一時のリポジトリ。fsmonitor とフィルターの印、特殊な名前、未追跡・ignore・変更あり・`git add` 済み) | `test`(3つの OS) | CI のランナーには git がある。git がないことの確かめは差し替えで行う |
| リンクとジャンクション(NFR3.18) | `test` | Linux と macOS はシンボリックリンク、Windows はジャンクション |
| 書き出しと Windows の名前の変更のやり直し(NFR6.5、NFR6.6、NFR9.11) | `test`(3つの OS) | U4・U6・U7 の保存のテストが変わらず通ること(`fileout` と `journal` の変更の回帰) |
| 大きなテスト(NFR5.25、NFR5.26、NFR3.19) | `test`、`race`(件数10分の1)、`coverage` | 3つの OS のそれぞれで 60 秒以内、ヒープの最大 2 GB 以下。`coverage` のジョブ(`-coverpkg` で遅くなる)では、`testing.CoverMode()` が空でないときに時間の上限を3倍にする(件数とメモリの上限は同じ)。git の呼び出しは差し替える |
| 3つの OS で同じ差分と結果(NFR9.11) | `test` | 期待の出力のファイルと比べる |
| U8 のルールファイルが読める(`TestSandboxU8RuleFilesAreValid`: `u8-rules.yaml` が「キーの規則がない」以外の警告なしで読め、`renameKey` と `changeValue` の変更案になること) | `test` | U6・U7 と同じ作り。AWS に触れる前に誤りが分かる |
| 新しい依存のビルドと脆弱性 | `build`、`test`(2つの版、`CGO_ENABLED=0`)、`vuln`、`modcheck` | `go.mod` の `go` の行は、Code Generation で依存を足す前と後の値を `code-summary.md` に記録して変わらないことを確かめる(変わるなら人に相談する。NFR9.12)。`modcheck` は `go mod tidy` の差分を今までどおり見る |
| `test/sandbox/` のテンプレートの cfn-lint と、書き換えたテンプレートの cfn-lint | `lint` | 上の表の2つ目の変更 |

### 手元の、実際の git を使う大きなテスト(NFR5.25)

大きなテストは、環境変数 `TAG_REFACTOR_LARGE_REAL_GIT=1` があるときだけ、git を差し替えずに、偽のファイルを一時のリポジトリにコミットしてから `--write` を動かす(CI では動かさない)。Code Generation で手元(Windows)で1回動かし、時間を `code-summary.md` に記録する。

## 検証用アカウントのリソース

- **新しいスタックは作らず、スタックも更新しない**(NFR3.21)。確認用のスタック(U1・U2・U5・U6)はどれも、スタックのタグ `project=tag-refactor-fixture` で作ったので、すべての確認用のリソースに `project` のタグが付いている。テンプレートに `project` を文字列で直接書いているのは、U6 の `test/sandbox/u6-fixtures.yaml` の VPC(VU6)とセキュリティグループ(GU6)の2つだけ。ほかのリソース(U6 のバケット BU6R を含む)の `project` はスタックのタグから来る。
- **確かめる場面(インフラの設計の質問のまとめからの訂正):** まとめでは「U6 の3つのリソース、6行」としたが、上のとおり、反映されるのは VU6 と GU6 の2つ(4行)で、ほかの確認用のリソースは機能設計 BR4.3 の keyNotInSource(スタックのタグかもしれない)として手で直す一覧に出る。スタックのタグを扱う大事な場面を確かめられるので、この期待で確かめる。
- **AWS の呼び出し:** 変更案を作る `plan`(読み取り用のプロファイル `tag-refactor-readonly`)、VU6 の ARN を作るための `describe-stack-resource`、書き換えたテンプレートの `validate-template`(どちらも管理用のプロファイル。読み取りだけの API)。AWS には書かない。
- 費用: 0(読み取りの API だけ)。

## 検証のスクリプト(`scripts/verify.sh` に足す)

### 差し込む位置と順序

U7 の確かめと U7 の測る値の表示のあと、人の確認の表示の前に、U8 の手順 1〜6 を置く。U8 の手順は AWS に書かないので、後片付けの旗(`u7_added` など)には加えない。

### 使うルールファイル

`test/sandbox/u8-rules.yaml`(新しい。Q1):

```yaml
version: 1
keys:
  Project:
    aliases: [project]
    values:
      tag-refactor-fixture-u8: [tag-refactor-fixture]
```

`project` を `Project` に名前を変え(renameKey)、値 `tag-refactor-fixture` を `tag-refactor-fixture-u8` に変える(changeValue)。変更案の範囲は、`project` のタグを持つすべてのリソース(確認用のスタックのすべてのリソースと、ほかにあればそれも)になる。

### 前提(Code Generation で合わせること)

- 種類の表(`internal/restype`)の resourceType の書き方は、scan が返す書き方に合わせる。たとえば S3 のバケットは、U6 のルールファイルの `s3` の書き方と同じ形にする(NFR 設計の種類の表の `s3:bucket` は仮の書き方)。合っていないと、BU6R が keyNotInSource でなく templateNotFound になり、手順 3 で止まる。
- `iac` の英語の結果の文言のうち、検証が読む2つを Code Generation で決め、`verify.sh` と表示のテストの両方で同じ文字列を使う: 手で直す一覧の keyNotInSource の行の理由の文言と、反映済みの数の行の形(例: `already reflected: 4`)。

### 一時のリポジトリの作り方

検証の git のコマンドは、利用者の設定に頼らない: `GIT_CONFIG_GLOBAL=/dev/null`、`GIT_CONFIG_NOSYSTEM=1` を付け、`init` は空のテンプレートのフォルダー(`--template=<空のフォルダー>`)で行い、`add` と `commit` には `-c core.autocrlf=false -c commit.gpgsign=false -c core.hooksPath=<空のフォルダー> -c user.name=tag-refactor-verify -c user.email=verify@example.invalid` を付ける。`iac` 自身の git の呼び出しは、`security-design.md` §6 のとおり(`GIT_` の環境変数を除くので、利用者の設定を読むが、ハッシュを自分で比べるので `core.autocrlf` の違いにも耐える)。

### 手順

1. **変更案を作る:** 前回の `<u3_dir>/u8-plan.json` を消してから、`tag-refactor plan --lang en --profile tag-refactor-readonly --region ap-northeast-1 --rules test/sandbox/u8-rules.yaml --out <u3_dir>/u8-plan.json` を動かす。終了コードが 0 か 2(U1 の P2 などの影響の指摘)で、ファイルができていれば進み、ほかは止める。VU6 の ARN は、管理用のプロファイルで `aws cloudformation describe-stack-resource --stack-name tag-refactor-u6-fixtures --logical-resource-id VU6` の `PhysicalResourceId`(VPC の ID)から `arn:aws:ec2:ap-northeast-1:<アカウント>:vpc/<ID>` と組み立てる。GU6 の ARN はスタックの出力から今までどおり作る。`scripts/u6check.go` に `includes` の確かめを足し(変更案に、指定した ARN が名前の変更と値の変更の両方を持って入っていること。ほかのリソースが入っていてもよい。`iac` は影響の指摘を使わないので、影響は確かめない)、VU6 と GU6 の ARN で確かめる。
2. **写しを作る:** `test/sandbox/` のすべてのファイルの SHA-256 の一覧を取ってから、`<u3_dir>/u8-repo` に写し、上の作り方で git のリポジトリにして1回コミットする。
3. **反映する:** `tag-refactor iac --lang en --plan <u8-plan.json> --dir <u3_dir>/u8-repo --write` を動かし、終了コード 2(手で直すものがある。スタックのタグのリソース)を確かめる。続けて次を確かめる。
   - `git -C <u8-repo> diff --numstat` が `u6-fixtures.yaml` の1行だけで、4行の追加と4行の削除であること(VU6 と GU6 の `Key` と `Value` の2行ずつ。NFR3.17)。ほかのテンプレートは変わらない。
   - 書き換えた `u6-fixtures.yaml` に、`Key: Project` が2つ、`Value: tag-refactor-fixture-u8` が2つあり、`Key: project` がないこと。
   - 標準出力の手で直す一覧に、VU6 と GU6 の ARN がなく、BU6R の ARN の行があり、その行の理由が keyNotInSource の文言であること(スタックのタグから来た `project`)。
4. **テンプレートを確かめる(Q2):** 書き換えた `u6-fixtures.yaml` を、管理用のプロファイルで `aws cloudformation validate-template --region ap-northeast-1 --template-body "file://<パス>"` で確かめる。パスは `cygpath` があれば `cygpath -m` で Windows の形(`C:/Users/...`)に直して渡す(Git Bash は `file://` の中のパスを直さないため)。大きさが 51,200 バイトを超えるときは止める(u6-fixtures.yaml は約 10 KB)。失敗したら AWS CLI の誤りの文を示して止める。`cfn-lint` が `PATH` にあれば、同じファイルを `cfn-lint` でも確かめる。なければ「cfn-lint がないので飛ばす(CI の lint のジョブで確かめている)」と表示する。
5. **2回目:** 同じ変更案で `iac --write` をもう一度動かし、終了コード 2(手で直すものは残る)、標準出力に差分(`--- a/` の行)がないこと、反映済みの数の行が 4 であること、`git -C <u8-repo> diff --numstat` が手順 3 と同じことを確かめる(反映済み。機能設計 BR4.3)。1回目の `iac` の時間を「git を呼ぶ時間の目安」として表示する。最後に、`test/sandbox/` のファイルの SHA-256 の一覧が手順 2 で取ったものと同じことを確かめる(元のファイルは変わっていない)。
6. **人の確認の準備(すべての確かめが通ったあとだけ):** `u3_dir` の外の一時フォルダー(`mktemp -d` で作る `tag-refactor-u8-check-XXXXXX`)に、手順 2 と同じ作り方のコミット済みの写し(`repo`)と `u8-plan.json` を置く。手順 1〜5 のどこかで失敗した場合は、このフォルダーを作らない。

### 後片付け

- 手順 2 の写し `<u3_dir>/u8-repo` は、`u3_dir` と一緒に今までどおり消える。
- 手順 6 のフォルダーは、人の確認のために消さずに残す。表示で場所と消すコマンドを示す(下の節)。このフォルダーの `u8-plan.json` にはアカウント ID と ARN が入ることも表示する。
- AWS に書かないので、AWS の後片付けは要らない。

## チェックポイントでの人の確認(Q3)

- 検証のスクリプトは最後に、手順 6 のフォルダーの場所と、PowerShell で動かすコマンドを表示する。パスは `cygpath` があれば `cygpath -w` で Windows の形に直す。
  - 確かめるコマンド: `go run ./cmd/tag-refactor iac --plan <フォルダー>\u8-plan.json --dir <フォルダー>\repo --lang ja`(`--write` なし。リポジトリのフォルダーで動かす)
  - 消すコマンド: `Remove-Item -Recurse -Force <フォルダー>`
- 人は確かめるコマンドを動かし、日本語の結果(VU6 と GU6 の差分、反映できる件数、スタックのタグのリソースが「ソースにない(スタックのタグかもしれない)」として手で直す一覧に出ること)を目で確かめ、消すコマンドでフォルダーを消す。
- 確かめた結果と、手順 5 の時間の目安を記録に残し、承認する。

## AWS への変更と費用

- 検証用アカウントのスタックとリソースは変えない(変更セットは要らない)。AWS の呼び出しは読み取りだけ(`plan` の読み取り、`describe-stack-resource`、`validate-template`)。
- 認証情報は表示しない(U2 と同じ)。
- 費用: 0。
