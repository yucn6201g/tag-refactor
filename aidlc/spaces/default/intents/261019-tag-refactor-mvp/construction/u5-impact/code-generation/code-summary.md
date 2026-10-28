# コードの生成の結果: U5 影響分析

計画(`code-generation-plan.md`)の Step 1〜15 をすべて終えました。ブランチは `bolt/u5-impact`(`main` の `2fe56af` から)。実際の AWS は呼んでおらず、検証用アカウントのスタックも変えていません(U1 のスタックの R2 の更新と U5 のスタックの作成は、チェックポイントの前に変更セットを人に示して許可を得てから行います)。push とプルリクエストはまだです。

## コミット

| コミット | 内容 | 対応 |
|---|---|---|
| `7a490f5` | test: 偽の AWS に IAM のインラインポリシーと5種類のリソースポリシーを足す | Step 1 |
| `db3b4b9` | test: U5 の受け入れ条件のテスト6件(この時点で6件すべてが失敗することを確かめた) | Step 2、US3.2 |
| `42ec240` | feat: インラインポリシーとリソースポリシーを含む影響分析の完成 | Step 3〜Step 8、Step 10 |
| `e881b02` | feat: 例のポリシーと確認用のテンプレートに U5 の読み取りを足す | Step 9 |
| `369c789` | test: U5 の単体テストと検出率100%の表 | Step 11 |
| `ee82db0` | test: 影響分析の大きな偽の AWS のテスト | Step 12 |
| `145e77f` | test: U5 の検証用アカウントのテストと `verify.sh` の確認 | Step 13 |
| `793f86c` | refactor: 読めなかったものと JSON の形の変換を直接行う | Step 14 で見直し |
| `737448a` | docs: 影響分析の完成と U5 のチェックポイントの説明 | Step 15 |

## 作ったファイルと変えたファイル(60ファイル、+7,440 / −431)

| 区分 | ファイル |
|---|---|
| 新規(実装) | `internal/impact/match.go`(結び付けと判定)、`internal/impact/findings.go`(指摘のまとめと並べ方)、`internal/awsaccess/inline.go`(インラインポリシー)、`internal/awsaccess/resourcepolicies.go`(5種類のリソースポリシー)、`internal/testutil/policies.go`(偽の AWS の応答) |
| 変更(実装) | `internal/impact/impact.go`、`internal/impact/policy.go`、`internal/awsaccess/awsaccess.go`・`invoke.go`・`read.go`・`session.go`、`internal/planner/planner.go`・`saved.go`、`internal/display/messages.go`・`results.go`・`planout.go`、`internal/cli/commands.go`、`internal/testutil/fakeaws.go`・`responses.go`、`go.mod`・`go.sum` |
| 新規(テスト) | `internal/impact/` の `detection_test.go`、`rules_test.go`、`large_test.go`、`fuzz_test.go`、`reader_test.go`、ファジングの種4件(`testdata/fuzz/FuzzParsePolicy/`)。`internal/awsaccess/policies_test.go`、`internal/planner/saved_u5_test.go`、`internal/display/impact_test.go`、`internal/cli/` の `acceptance_u5_test.go`・`impact_u5_test.go`・`integration_u5_test.go`、`internal/testutil/policies_test.go` |
| 変更(テスト) | 下の「期待を変えた既存のテスト」 |
| 例のポリシー・検証 | `docs/iam/readonly-policy.json`、`test/sandbox/u1-fixtures.yaml`(R2)、`test/sandbox/u5-fixtures.yaml`(新規)、`test/sandbox/u5-rules.yaml`(新規)、`scripts/verify.sh` |
| ドキュメント | `README.md` |

削除したファイルはありません。

## 主な実装の決定

- **読み取りの口:** U1 の `PolicyReader` を `IAMPolicyReader` と `ResourcePolicyReader` に分け、`ListAttachedCustomerPolicies` を `ListCustomerPolicies` に改めた。付いているものだけに絞る指定を外し、付いている数と権限の上限の数を返す(機能設計 BR1.1)。
- **NotFound:** 操作ごとの誤りのコードの表(`opErrorCodes`)にだけ足した。IAM の `NoSuchEntity` は U5 で足した6つの操作だけで NotFound にし、U1 の `GetPolicyVersion` の分類は変えていない。S3 の `NoSuchBucketPolicy` は誤りにせず「ポリシーなし」として返す。
- **同時に読む作り:** 仕事を足すのは配り手だけ。`errgroup` はエラーを返さない使い方にし、失敗は各仕事の結果に集める。上限は IAM 10(チェックポイントで4から改めた。下の「チェックポイントの検証」)、S3 の場所 4、リージョン 8、リージョンの中 4 で、テストからだけ差し替えられる。
- **ワイルドカードの使い回し:** ロックで守る入れ物で、上限は1,024個。設計にない守りを2つ足した: 4,096 バイトを超えるパターンは入れ物に入れない(メモリを守る)。`*` を除いて 4,096 文字を超えるパターンは「当てはまらない」とする(AWS のタグのキーは最大128文字、値は256文字なので判定は変わらず、正規表現が大きくなりすぎるのを防ぐ)。続く `*` は1つにまとめる。
- **policyRef:** ユーザー・ロール・グループのインラインポリシーの名前の一覧を読めないときは、そのユーザー・ロール・グループの ARN を policyRef にした(`#` なし。表示は「IAM inline policies of <ARN>」)。キューの属性を読めないときは、キューの URL から ARN を作る。
- **表示と保存:** 種類ごとの表示名を足した(管理ポリシーは U1 の受け入れ条件のテストに合わせて `IAM policy` のまま)。`?` の理由を、ポリシー変数なら `cannot decide (policy variable)`、それ以外は `cannot decide (condition operator)` と分けた。Markdown の「読めなかったもの」の表に Region の列を足した。保存する JSON では `conditionOperator` を常に書き、`unreadPolicies` の `policyRef` と `region` は空のとき省く(契約 C2 の定義のとおり)。

## テストの結果(手元、Windows)

| 確かめたこと | 結果 |
|---|---|
| `gofmt -l` | 出力なし |
| `go vet ./...`、`go vet -tags integration ./...` | 問題なし |
| `golangci-lint run`(`integration` のタグあり・なし) | 0 issues |
| `go test ./...`(Go 1.27.0) | すべて通る |
| `go test ./...`(Go 1.26.0。CI の oldstable と同じ系列) | すべて通る |
| カバレッジ(`-coverpkg` で `cmd` と `internal` のすべて) | 97.1%(下限80%) |
| 検出率100%の表(NFR2.1) | 252行と32行がすべて期待どおり。行の数もテストで確かめる |
| 呼び出しの回数(NFR1.16) | 測る条件で 4,913 回。式から計算した上限 4,913 回以下で、操作ごとにも上限以下 |
| `Analyze` の時間(NFR1.17) | 0.90 秒(上限10秒) |
| 同時の数(NFR1.18) | IAM 10、偏ったリージョンの中 4、リージョン 8 まで使い、超えない。枠をすべて1にしても終わる |
| U5 の大きなテストの合計の時間(NFR10.9) | 約2.5秒(CI の各 OS で2分以内) |
| ファジング `FuzzParsePolicy` 5分(NFR8.21) | 12,686,126 回、コーパス512件、失敗なし |
| `govulncheck` | 外部のモジュール(新しく足した SDK のモジュールを含む)に該当なし。手元の Go 1.27.0 の標準ライブラリの既知の脆弱性だけ(1.27.2 以降で直っている) |

- 受け入れ条件のテスト6件は Step 2 で先に書いてコミットし、失敗することを確かめてから実装した(Testing Contract の custom の順)。
- `-race` は、手元に cgo と gcc が無いため動かしていない。CI の `race` ジョブで確かめる。
- `cfn-lint` は手元に無いため動かしていない。CI の `lint` ジョブで確かめる。

## 作りながら確かめたこと

- **SDK のモジュールの版:** 今の `go.mod` の SDK(core v1.47.3、smithy v1.28.5)と同じ時期のもの(s3 v1.114.3、kms v1.61.4、sqs v1.52.4、sns v1.47.5、secretsmanager v1.50.4)。core と smithy の版は変わらない。
- **`ListBuckets` の `BucketRegion`:** SDK の型(`types.Bucket.BucketRegion`)にあり、SDK を通したテストで読めることを確かめた。`GetBucketLocation` の空は us-east-1、`EU` は eu-west-1 に読み替える。
- **誤りのコードの名前(SDK の型で確認):** KMS `NotFoundException`・`KMSInvalidStateException`、SNS `NotFound`・`AuthorizationError`、Secrets Manager `ResourceNotFoundException`、S3 `NoSuchBucket`、SQS は型の名前が `QueueDoesNotExist` で、応答の `X-Amzn-Query-Error` ヘッダーがあると `AWS.SimpleQueueService.NonExistentQueue` に置き換わるため、両方を表に入れた。

## 期待を変えた既存のテスト

- **`internal/impact` の U1 のテスト:** `New(reader, regions)` と広げた口に合わせた。U5 で照合の対象が広がったため、`aws:RequestTag` と `aws:TagKeys` の条件も affected になる期待を足した(機能設計 BR2.1)。U1 で決めた振る舞い(付いている管理ポリシー、大文字・小文字だけの変更、変数、ワイルドカード、読めなかったとき)の期待は変えていない。
- **`internal/awsaccess`:** 管理ポリシーの一覧で「付いているものだけ」の指定を送らないこと、必要な権限の一覧、`ListBuckets` の IAM の操作の名前が API の名前と違うことを期待に入れた。例のポリシーの検査を、「`GetPolicyVersion` 以外は `*`」から操作ごとの `Resource` の表に置き換えた。
- **`internal/cli`:** AC1.2.1 の操作の一覧と U2 の `apiOperations` に U5 の操作を足した。U2 の性能のテストの呼び出しの回数の上限に、U5 の式の空のアカウントの分(3 + 1 + 4 × リージョンの数)を足した(下限を緩めたのではなく、読む操作が増えた分)。
- **`internal/planner`:** U4 までは保存していなかった `conditionOperator` と `policyRef` を保存するようになったため、往復の期待と項目の一覧を直した。
- **`internal/display`:** U5 より前に保存した変更案(`conditionOperator` なし)の表示を確かめるテストに改めた。期待のファイル5件を作り直し、差分を確かめた。

## 計画と設計からのずれ

- ワイルドカードの使い回しに、上の2つの守り(4,096 バイト・4,096 文字)を足した。照合の結果は変わらない。
- インラインポリシーの名前の一覧を読めないときの policyRef を、ユーザー・ロール・グループの ARN にした(機能設計 BR1.6 は1つのポリシーの形だけを決めていた)。
- **U5 の確認用のスタックから、キー KU5 とキューポリシー QU5Policy を外した(`2bab29b`)。** 変更セットの実行で、KMS がキーポリシーの `aws:ResourceTag` を拒み(「These condition keys are not permitted in a key policy」)、SQS がキューポリシーの `aws:ResourceTag` を拒んだ(「Conditions must be from Global context key list」)。どちらも ValidatePolicy では誤りにならず、作成のときに初めて拒まれた。インフラの設計(`cicd-pipeline.md`)の取り決めどおり、拒まれたものだけをテンプレートから外し、この2種類は偽の AWS のテスト(NFR2.1)だけで確かめる。キュー QU5 は RU5 のインラインポリシーが指すので残した。テンプレートのテスト、結合テスト、`verify.sh`、README を合わせて直した。U5 のスタックは3回目の作成で `CREATE_COMPLETE`(8個のリソース)。失敗した2回はどちらも自動で巻き戻り、リソースは残っていない。
- 作るときに、ファイルの書き込みをシェルで行った場面が3回あった(`internal/testutil/policies.go` の一部を node で書き換え、`internal/impact/impact_test.go` の `New` の呼び出しを `sed -i` で直し、`verify.sh` の awk の試しに一時ファイルを書いた)。中身はコミットの前に見直した。

## チェックポイントの検証(2026-10-28)

- `verify.sh` は2回とも `verify: OK`(終了コード0)。1回目はエンジンの検証として記録され、出力の最後の部分しか残らないため、同じコマンドをもう1回動かして全体を残した。以下の値は2回目のもの。
- `TestIntegrationU5ImpactSandbox` が通った。指摘7件、読めなかったもの0件で、影響分析は完全。RU5 は ap-northeast-1 の収集に出なかったため、PrincipalTag と Resource の確かめは偽の AWS のテストに任せた(条件付きの確かめのとおり)。
- `--debug` 5回の L95: L95(IAM) 611 ms(`iam:ListPolicies`、標本5で最大)、L95(リソースポリシー) 128 ms(`s3:ListBuckets`)、再試行0回。IAM の仕事の操作は `iam:ListRolePolicies` 220 ms(標本150)、`iam:GetRolePolicy` 217 ms(標本105)、`iam:GetPolicyVersion` 598 ms(標本15)。
- **10分の見積もり(NFR1.19)は上限を超えた。**
  - 要件の式: IAM の分 = 2,504 ÷ 4 × 0.611 = 383 秒、リソースポリシーの分 = 2,408 ÷ 8 × 0.128 × 1.5 = 58 秒。影響分析の時間は 383 秒。
  - 設計の式(`logical-components.md`): IAM の分 = (4 + 2,500 ÷ 4)× 0.611 × 1.2 = 461 秒、リソースポリシーの分 = 57 秒(すべてが1つのリージョンにある場合は 114 秒)。影響分析の時間は 461 秒。
  - U2 の NFR1.8 の見積もりは 469 秒で、そのうち IAM の分は 101 × 0.603 = 61 秒。IAM の分を影響分析の時間に置き換えると、要件の式で 791 秒、設計の式で 869 秒になり、600 秒を超える。
  - NFR1.19 の取り決めでは、10分を超えたときは NFR1.18 の同時の数を見直し、測り直してから人の承認を得る。
- **IAM の同時の数を10に改めて測り直した(`aa163f1`)。** `verify.sh` は2回とも `verify: OK`、指摘7件、読めなかったもの0件、再試行0回。
  - L95(IAM) 808 ms(`iam:ListPolicies`、標本5で最大)、L95(リソースポリシー) 156 ms。IAM の仕事の操作は、同時の数を上げると遅くなった: `iam:ListRolePolicies` 220 → 599 ms(標本150)、`iam:GetPolicyVersion` 598 → 724 ms、`iam:GetRolePolicy` 217 → 227 ms。
  - 要件の式: IAM の分 = 2,504 ÷ 10 × 0.808 = 202 秒、リソースポリシーの分 = 2,408 ÷ 8 × 0.156 × 1.5 = 70 秒。
  - 設計の式: IAM の分 = (4 + 2,500 ÷ 10)× 0.808 × 1.2 = 246 秒、リソースポリシーの分 = 70 秒(すべてが1つのリージョンにある場合は 140 秒)。
  - U2 の見積もりは 483 秒で、そのうち IAM の分は 101 × 0.588 = 59 秒。置き換えると、要件の式で 626 秒、設計の式で 670 秒。**まだ 600 秒を超える。**
- **人の判断で、IAM の読み方を `GetAccountAuthorizationDetails` に改めた(`7043073`、`0864396`)。** 設計の記録(機能設計 BR1.1・BR1.2・BR1.5、NFR 要件 NFR1.16・NFR1.18・NFR1.19、NFR の設計 `security-design.md` §1・`logical-components.md`、インフラの設計 `cicd-pipeline.md`)を合わせて改めた。
  - `IAMPolicyReader` の5つの口を `ReadIAMPolicies` の1つにした(絞り込み User・Role・Group・LocalManagedPolicy、1ページ 1,000 件、すべてのページを `invoke` で順に)。既定の版の文書は `IsDefaultVersion` で選び、文書が無い管理ポリシーとインラインポリシーは、それぞれそのポリシーを読めなかったものにする。どのページかを読めなければ、`iamManaged` と `iamInline` を一覧ごと読めなかったものにし、IAM は1つも照合しない。`iamWorkers` と `inline.go`、使わなくなった IAM の操作と NotFound の表の項目を消した。
  - 例のポリシーと R2(テンプレート)は、外した IAM の操作の代わりに `iam:GetAccountAuthorizationDetails` を `"*"` で許す。`verify.sh` の R2 の確かめもこの操作に変えた。
  - 大きなテストの呼び出しの回数は 2,411 回(上限 2,411。以前は 4,913)、`Analyze` は約1.1秒、同時の数は IAM 1・リージョンの中 4・リージョン 8、カバレッジ 97.1%。
  - U2 の結合テストの NFR1.8 の見積もりの IAM の項を、101 回(`ListPolicies` と `GetPolicyVersion`)から1回(1ページ)にした。U2 の性能の設計の「101 回の IAM の読み取り」は、U5 の改訂で1回になった。
  - 表示のテストのうち、以前に保存した変更案に入っている `iam:ListRolePolicies` の読めなかったものを表示する行は残した(古い変更案を読み込むと今も出るため)。
- **R2 を変更セット `u5-gaad` で更新し(R2 の `Policies` だけ、Replacement False)、測り直した。**
  - 1回目(エンジンの検証): `verify: OK`、検証済みとして記録。U2 の見積もりは 405 秒(IAM の項は1ページ × 1,188 ms)。
  - 2回目(出力の全体を残す): U5 の確認はすべて通った(指摘7件、読めなかったもの0件、再試行0回)が、最後の U2 の見積もりのテストが 848 秒で失敗した。原因は `sqs:ListQueues` の L95 が 2,543 ms(標本17。遠いリージョンの1回の遅い呼び出し)になったことで、U5 の変更とは関係しない。U2 の見積もりは、これまでの実行で 405〜848 秒とばらついている(標本の少ない操作の最大値を、214回の呼び出しに掛けるため)。
  - U5 の `--debug` 5回の L95: `iam:GetAccountAuthorizationDetails` 1,491 ms(標本9。検証用アカウントでは1回の実行が1〜2ページ)、L95(リソースポリシー) 100 ms、再試行0回。
  - 設計の式: IAM の分 = 2 × 1.491 × 1.5 = 4.5 秒(1ページ30秒としても60秒)、リソースポリシーの分 = 0.1 + 296.5 × 0.100 × 1.5 = 45 秒(すべてが1つのリージョンにある場合は 596 × 0.100 × 1.5 = 89 秒)。影響分析の時間は最大で 89 秒。要件の式では IAM 3 秒、リソースポリシー 45 秒。
  - U2 の見積もりの IAM の分を影響分析の時間(89 秒)に置き換えると、1回目の値で 405 − 1 + 89 = **493 秒(600 秒以内)**、2回目の値で 848 − 1 + 89 = 936 秒(U2 の見積もり自体が 600 秒を超えているため)。

## まだ確かめていないこと

- **検証用アカウント:** `verify.sh` と結合テスト `TestIntegrationU5ImpactSandbox` は、U1 のスタックの R2 の更新と U5 のスタックの作成のあとに動かす(環境変数が無いときにテストが飛ばされることだけは確かめた)。そのとき、RU5 の変更が収集に出たか、操作ごとの L95 と10分の見積もりを記録する。
- **ValidatePolicy と変更セット:** 人の許可を得る前の確かめとして行う(`cicd-pipeline.md`)。
- **CI:** `-race`、`cfn-lint`、3つの OS は、プルリクエストの CI で確かめる。
