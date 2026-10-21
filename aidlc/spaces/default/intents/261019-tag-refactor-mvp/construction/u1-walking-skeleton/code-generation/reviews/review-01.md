## Review

**Verdict:** READY
**Reviewer:** aidlc-architecture-reviewer-agent
**Iteration:** 1

### 確認した範囲と結果の概要

- 手元で `gofmt -l`(差分なし)、`go vet ./...`(問題なし)、`go test -mod=readonly -count=1`(10パッケージすべて ok)、カバレッジ(`-coverpkg=./cmd/tag-refactor/...,./internal/...`)を再実行しました。合計は 97.1%(`internal/testutil` を除くと 97.5%)で、要約の主張と一致します。`integration` タグのテストと実際の AWS は呼んでいません。
- 受け入れ条件のテスト(`2c97588`)は実装(`3dc58e7`)より前にコミットされており、`acceptance_test.go` はその後 `094f34e` で整形(gofmt)だけが変わっています。テストを弱めた形跡はありません。
- BR1.1〜BR6.3 を `planner.go`、`impact.go`、`policy.go`、`rules.go`、`schema.go`、`inventory.go`、`display`、`cli` と突き合わせました。BR4.2 の併合と衝突、`mergesIntoExisting`、BR5.2〜BR5.4 の条件キーの解析、大文字・小文字、ワイルドカード、`undecided`、BR6.3 の終了コードの優先順位、BR1.7 のオプションは、設計どおりでした。
- 読み取り専用の強制は、型(`readers` が読み取りの口だけを持つ。`TagWriter` に実装がない)、偽の AWS が読み取り以外の操作を失敗させること、`docs_test.go` が操作の表と例のポリシーを突き合わせることの3つで担保されています。
- 認証情報の漏れの経路(SDK のエラー文、`--debug`、SDK のログ、cobra の出力、panic)は、エラーの文言が道具側の項目だけから作られ、SDK のログが無効で、panic が固定の1行になっているため、閉じています。表示の無害化は `line()` が文字列の引数をすべて通しており、抜けは見つかりませんでした。
- 確認用の CloudFormation には、設計にない権限や公開のバケットはありません。信頼の設定は、パラメータの文字種で `*` を拒みながら、アカウントの SSO ロールに限っています。バケットは公開をすべて遮断しています。
- 要約の「計画・設計からの違い」は、安全側の違いか表示の違いで、受け入れられると判断します。ただし C9・C6 の形の変更は R-03 を参照してください。
- 高(high)の問題は見つかりませんでした。中(medium)が2件あるため、承認の前に重みを見てください。

### Prior findings

| ID | Now | Severity | Note |
|---|---|---|---|
| なし(iteration 1 のため、以前の指摘はありません) | - | - | - |

### New findings

| ID | Severity (high/medium/low) | Location | Finding | Suggested fix |
|---|---|---|---|---|
| R-01 | medium | internal/rules/rules.go の `Parse`(`parser.ParseBytes` の呼び出しと `checkTree` の `maxDepth` の判定の順序) | 深さの上限 32 は、YAML の構文木を作った後でしか効きません。構文木を作る段階の goccy/go-yaml が、入れ子に対して 2 乗の時間とメモリを使います。実測では、`keys: ` の後に `[` を並べた入力が 3 万個で 0.5 秒、10 万個(約 100 KB)で 6.7 秒、20 万個で Go ランタイムの `fatal error: out of memory` になりました。どれも 1 MiB の上限より小さい入力です。致命的エラーは `main.go` の panic の捕捉を通らず、スタックトレースが標準エラーに出ます。Go の致命的エラーの終了コードは 2 で、BR6.3 の「要確認」と同じ値です。NFR8.4(悪意のあるルールファイル)の想定の脅威に当たります。README の「1 MiB を超えるファイルは受け付けません」だけでは、このファイルを止められません。 | `ParseBytes` の前に、バイト列を1回走査して、フロー形式の括弧(`[`、`{`)の深さと、インデントの深さを数え、32 を超えたら `InvalidInput` にします。あわせて、ルールファイルの上限を、ルールファイルに必要な大きさ(例 64 KiB)まで下げることを検討します。深い入れ子のファイルが数秒以内に `InvalidInput` で終わるテストを足します。 |
| R-02 | medium | internal/awsaccess/session.go の `Resolve`(STS の `GetCallerIdentity` が `checkRegionEnabled` より先に呼ばれる)と invoke.go の `errorCodes`(`InvalidClientTokenId` を `CredentialsMissing` にしている) | BR1.1 は、有効でないリージョンを「そのリージョンは使えない」と案内することを求めています。コードは、リージョンの確認(`ec2:DescribeRegions`)より先に、指定したリージョンの STS の口を呼びます。有効にしていないオプトインのリージョンの STS の口は `InvalidClientTokenId` を返すと考えられます(AWS の動作の知識による見立てで、実際のアカウントでは確かめていません)。その場合、「AWS rejected the credentials」という、利用者を誤った方向に導く案内になります。単体テストが確かめているのは、DNS 解決の失敗と `RegionDisabledException` の変換だけです。 | リージョンの有効性の確認を、STS の呼び出しより前に行います。たとえば既定のリージョンか `us-east-1` で `DescribeRegions` を呼びます。それができない場合は、指定したリージョンでの STS の `InvalidClientTokenId` のときに、リージョンの確認をやり直して、使えないリージョンなら `ReasonRegionUnavailable` にします。U1 のチェックポイントで、有効でないオプトインのリージョンを使った確認を1回入れます。 |
| R-03 | low | contract-summary.md の C9(`Analyze(ctx, PolicyReader, []TagChangeInput)`)と C6(`PolicyReader`)に対し、internal/planner/planner.go の `ImpactAnalyzer` と internal/awsaccess/awsaccess.go の `PolicyReader` | 実装は契約から外れています。C9 の `Analyze` は `PolicyReader` を引数に取らず、`impact.New(reader)` で持ちます。C6 の `PolicyReader` は一覧と1件の読み取りの2つに分かれています。理由(依存の向き、BR5.5 の部分的な失敗)は妥当ですが、共有の契約には反映されておらず、要約が「U2、U4、U5 が引き継ぐ」と書くだけです。U4 と U5 が契約の文面のまま実装を始めると、手戻りになります。 | C6・C9 の変更を、契約の改訂としてチェックポイントで承認してもらい、次の Unit の入力(契約の要約か引き継ぎのメモ)に実際の署名を書きます。 |
| R-04 | low | internal/display/results.go の `PlanResult` と `ScanResult`、README | 出力のどこにも、調べた範囲が書かれていません。`plan` の「0 affected」は、付いている、利用者が作った管理ポリシーだけを調べた結果です。インラインポリシー、リソースポリシー、SCP は調べていません。`scan` の「No issues found」は、タグ API が返すリソースだけを対象にしています(リージョンをまたぐグローバルなサービスは対象外の可能性があります)。README には書いてありますが、利用者が見る出力にはなく、見逃さない側に倒す設計の意図と合いません。 | 標準エラーに、調べた範囲を1行出します(例 `Checked: customer managed IAM policies attached to a user, role or group`)。README の「できること」に、タグ API の範囲を足します。 |
| R-05 | low | traceability.json の BR4.4 と BR5.6 の target(`internal/display/results.go`、`internal/impact/policy.go`) | 対象のファイルが、規則を実装している場所からずれています。BR4.4(影響分析をしない)の判断は `internal/cli/commands.go` の `runPlan` にあり、`results.go` は文言を出すだけです。BR5.6(影響の単位の作成)は `internal/impact/impact.go` の `match` にあり、`policy.go` は解析だけです。BR1.6 も、拒否の分類は `invoke.go`、案内は `messages.go` に分かれています。 | BR4.4 を `internal/cli/commands.go`、BR5.6 を `internal/impact/impact.go` に直します。 |
| R-06 | low | docs/iam/readonly-policy.json の `iam:GetPolicyVersion` の `Resource: "*"` | 読み取りの例のポリシーとして、`iam:GetPolicyVersion` が全 ARN を対象にしています。このツールが読むのは利用者が作った管理ポリシーだけです。`iam:ListPolicies` と `tag:GetResources`、`ec2:DescribeRegions` はリソースを絞れないので、`*` のままで構いません。 | `iam:GetPolicyVersion` だけを、`arn:aws:iam::<アカウント ID>:policy/*` に絞った別の文にすることを検討します。その場合は `docs_test.go` と、確認用のテンプレートの R2 の同一性の検査も合わせて直します。 |
| R-07 | low | internal/rules/schema.go の `loadRoot`(`keys` が必須でない) | `version: 1` だけのファイル、または `keys` を書き間違えたファイルが、エラーにも警告にもならず、`plan` が「No changes needed」で終了コード 0 になります。タイプミスは、知らないキーとして止まります。ただ、`keys` を書き忘れたファイルは、何も変えない計画を正常として扱います。 | `keys` が無いか空のときは、警告(例 `no key rules`)を出します。 |
| R-08 | low | .github/workflows/ci.yml と scheduled.yml、test/sandbox/u1-fixtures.yaml の `P2`(`PolicyVariableKey`)、internal/cli/integration_test.go | 確かめられていない点が3つあります。(a) 固定した Action の SHA とコメントの版(`v7.0.1` など)が実在し、一致することを、このレビューでは確かめていません。(b) `oldstable`(1.26.10)での実行は未確認です。(c) キーの側にポリシー変数を置いた条件を IAM が受け付けるかは未確認です。受け付けない場合の外し方は設計にあります。また、`integration_test.go` は P1 が `!` で出ることを、(`Statement "AllowDevQueues"` の行の有無で見るだけで)判定の記号と `?` の件数までは確かめません。 | 最初の push の CI で、(a)(b)を確かめます。チェックポイントで、スタックの作成が通るか、P2 の変数の条件が拒まれないかを確かめます。余裕があれば、統合テストに `! IAM policy` の行の検査を足します。 |
| R-09 | low | internal/display/results.go の `planFindings` と functional-spec.md の出力の例 | 影響の行に条件の演算子(`Condition StringEquals aws:ResourceTag/env = "dev"`)が入っており、機能仕様の出力の例(`Condition aws:ResourceTag/env = "dev"`)と違います。BR5.6 の影響の単位に演算子があるため、設計の意図には合います。ただし、機能仕様の例は更新されていません。 | 機能仕様の出力の例を、実際の出力の形に合わせて直します。 |

### Validation Tool Results

| Tool | Result | Interpretation |
|---|---|---|
| gofmt -l . | 差分なし | 整形済み |
| go vet ./... | 問題なし | 問題なし(`integration` タグのコンパイルは CI の手順で確認される) |
| go test -mod=readonly -count=1 ./... | 10パッケージすべて ok | 受け入れ条件、単体、性能、出力の守りが通っている |
| go test -coverpkg=./cmd/tag-refactor/...,./internal/... | 合計 97.1% | 80% の下限を満たす。要約の主張と一致 |
| 追加の確認(仮想のテストファイル。リポジトリのファイルは変更していない) | 入れ子の深い YAML で、2 乗の時間と、20 万個での致命的エラー | R-01 の根拠 |

### Summary

U1 の規則とセキュリティの設計は、コードとテストでよく支えられており、高の問題はないため READY とします。承認の前に、ルールファイルの入れ子による停止・メモリ不足(R-01)と、有効でないオプトインのリージョンで案内が誤る可能性(R-02)を見てください。

### Findings

| ID | Severity | Location | Finding | Required action | Status |
|---|---|---|---|---|---|
| - | - | - | No findings | No action required | Resolved |
