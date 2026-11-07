# Build and Test の要約

## 全体の状態

| 項目 | 状態 |
|---|---|
| ビルド | 成功(6つの組み合わせ、`CGO_ENABLED=0`) |
| テスト | すべて通過(失敗 0) |
| カバレッジ | 96.1%(下限 80%) |
| CI | プルリクエスト #20 で 15 件通過、失敗 0(`-race`、Linux、macOS、Windows、cfn-lint を含む) |
| 検証用アカウント | `scripts/verify.sh` が `verify: OK`(2026-11-07、3回目) |
| 対象の判定 | Met 160 件、Not Met 0 件、Unverified 1 件 |
| 作業単位をまたぐ対応 | 不合格(範囲の外の ID が未カバー。`cross-unit-traceability.md`) |

**この段階は、そのままでは成功ではありません。** `NFR12`(配布)が Unverified で、`AC6.2.1`〜`AC6.2.3`、`AC7.2.1`、`AC7.3.1`、`FR8.3`、`FR8.4` が未カバーです。どれも、これまでに人が範囲の外と決めたものです。

> **追記(2026-11-07):** そのあと `v0.1.0` を公開し、`NFR12` と `AC6.2.1`〜`AC6.2.3` を公開物で確かめました(`verification/phase-check-construction.md` の「公開のあとの追記」)。この要約の判定は、この段階の時点のまま残しています。

## 準備の状態

| 項目 | 判定 | 理由 |
|---|---|---|
| ビルドできる | はい | `build-instructions.md` のとおり |
| テストできる | はい | `integration-test-instructions.md` のとおり |
| 配布できる | **いいえ** | 公開の仕組みがない。U8 のプルリクエストもまだマージしていない |

## テストの種類

Test Strategy は Standard なので、結合テストの手順書だけを作りました。性能とセキュリティの確かめは、`integration-test-instructions.md` の表のテストに含まれ、別の手順書は作っていません。

| 種類 | 手順 | 結果 |
|---|---|---|
| 単体 | 各作業単位の `unit-test-instructions.md` | 通過 |
| 受け入れ条件 | `TestAcceptance*` | 通過 |
| 結合 | `integration-test-instructions.md` | 通過 |
| 性能(大きなテスト) | `TestPerformance*`、`TestLarge*` | 通過 |
| ファズ | `FuzzParse`、`FuzzLoadPlan`、`FuzzParsePolicy` を各5分 | 通過 |
| 検証用アカウント | `scripts/verify.sh` | 通過 |

## 作業単位ごとのカバレッジ

各作業単位で下限は同じ 80%(`cmd` と `internal` のすべてで測る)。今の値は全体で 96.1% です。作業単位ごとの当時の値は各 `code-summary.md` にあります。

## 判定の読み方と限界

- **Met の根拠:** 細かい要件(`NFR1.1` など)の行は、各作業単位の対応表の対象のファイルが存在し、そのテストや検証が通ったことで Met としています。Actual の「この対象の確かめが通る」はその意味です。**1行ずつ、テストが要件を十分に確かめているかを読み直してはいません。** その確かめは、各作業単位のコードのレビューで行いました。
- **読み取りの遅れ:** 2026-11-07 の検証で、タグの API が `apply` の前の答えを返すことが分かりました(11秒後でも)。`rollback` は直しましたが、`plan` は読めたものだけを対象にするので、`apply` の直後の変更案は、書いたばかりのリソースを見落とすことがあります。README にはまだ書いていません。
- **Terraform の例:** `internal/iac/testdata/terraform-show-example.json` は手で作り、公式の文書で形を確かめたものです。実物の `terraform show -json` の出力とは照らしていません。
- **確かめていないリソースの種類:** IAM、EC2 のインスタンス、スナップショット、AMI、KMS、Lambda、DynamoDB、CloudWatch Logs、RDS のタグは、検証用アカウントで書き換えていません(U6)。
- **マージ前の値:** すべてブランチ `bolt/u8-iac`(`da2dad9`)の値です。`main` は `82bb6f3` のままで、squash マージ後の `main` の CI は確かめていません。
- **配るバイナリの Go の版:** 手元の Go 1.27.0 では、`govulncheck` が標準ライブラリの脆弱性9件を指摘します。Go 1.27.3 では指摘がありません。配るときは 1.27.3 以上でビルドします。

## Target Verification Matrix

| Target ID | Source | Expected | Actual | Evidence | Owning Stage | Verdict |
|---|---|---|---|---|---|---|
| NFR1 | inception/requirements-analysis/requirements.md | scan と plan を合わせて10分以内(1アカウント、全リージョン、1万リソース。少ない件数で見積もるなら根拠を示す) | 見積もり 493 秒(U5 の測定、2026-10-28)。U6 の日の U2 の見積もりは 408〜409 秒。今回の検証コマンドの中の見積もりのテストも 600 秒以内で通った。1万リソースを実際に動かした値ではなく、遅い呼び出しが1回混ざった回では 936 秒の見積もりも出ている | construction/u5-impact/code-generation/code-summary.md、scripts/verify.sh の TestIntegrationU2LatencyEstimate | build-and-test | Met |
| NFR2 | inception/requirements-analysis/requirements.md | 影響分析がテスト用の参照を100%検出 | 検出の表のテストが通る | internal/impact/detection_test.go(TestDetectionTable) | build-and-test | Met |
| NFR3 | inception/requirements-analysis/requirements.md | scan、rules suggest、plan は読み取りだけで、読み取り用の権限で動く | 読み取り用のプロファイルで検証コマンドが通る。偽の AWS のテストで書き込みの操作がない | scripts/verify.sh、internal/awsaccess/docs_test.go | build-and-test | Met |
| NFR4 | inception/requirements-analysis/requirements.md | apply のあと、rollback でタグが完全に一致(差分0) | 自動テストと検証用アカウントで差分0。1回目の検証で不具合が出て直した(`52163d6`)。タイミングで出る問題なので、1回の通過で直ったとは言い切れない | internal/apply/rollback_scenario_test.go、scripts/verify.sh | build-and-test | Met |
| NFR5 | inception/requirements-analysis/requirements.md | すべての API 呼び出しに時間の上限。回数の上限では待って再試行 | 偽の AWS で再現するテストが通る | internal/awsaccess/awsaccess_test.go | build-and-test | Met |
| NFR6 | inception/requirements-analysis/requirements.md | 書き換えの前の記録を、壊れない方法で先に保存 | 保存中に止めるテストが通る | internal/journal/journal_test.go | build-and-test | Met |
| NFR7 | inception/requirements-analysis/requirements.md | apply や rollback が同時に動いても記録が壊れない | 印のテストが通る | internal/journal/journal_test.go | build-and-test | Met |
| NFR8 | inception/requirements-analysis/requirements.md | 認証情報を表示、記録、ログに出さない | 出力の確かめのテストが通る | internal/iac/write_test.go ほか、各作業単位の出力の確かめ | build-and-test | Met |
| NFR9 | inception/requirements-analysis/requirements.md | Linux、macOS、Windows で動く。単一の実行ファイルで配れる | プルリクエスト #20 の CI で3つの OS × Go 2版のテストが通る。手元で6つの組み合わせを CGO_ENABLED=0 でビルドできた | .github/workflows/ci.yml、test-results.md | build-and-test | Met |
| NFR10 | inception/requirements-analysis/requirements.md | カバレッジ(Go の文単位)80%以上 | 96.1% | test-results.md | build-and-test | Met |
| NFR11 | inception/requirements-analysis/requirements.md | 色が見えなくても意味が分かる | NO_COLOR の期待の出力のテストが通る | internal/display/color_test.go ほか | build-and-test | Met |
| NFR12 | inception/requirements-analysis/requirements.md | GitHub Releases と go install で入手できる。リリースにチェックサムのファイルが含まれる | 確かめられない。公開の仕組みがなく、タグもない。実行計画に、これを受け持つ後の段階もない | なし(units-generation の Q3、Q4 で別の作業に決めた) | 別の作業(配布と v0.1.0) | Unverified |
| TC-COVERAGE | team.md Testing Posture、code-generation-plan.md Testing Contract | カバレッジ80%以上、配線の部分も外さない | 96.1%(-coverpkg で cmd と internal のすべて) | test-results.md | build-and-test | Met |
| TC-CI | team.md Testing Posture、code-generation-plan.md Testing Contract | マージの前に CI でテストを実行し、3つの OS で動かす | プルリクエスト #20 で 15 件通過、失敗 0、artifacts は main 以外で飛ばす | プルリクエスト #20 の CI | build-and-test | Met |
| TC-NO-AWS-IN-CI | team.md Testing Posture、code-generation-plan.md Testing Contract | CI から実際の AWS を呼ばない | ci.yml に認証情報の設定がない。テストは偽の AWS を使う | .github/workflows/ci.yml | build-and-test | Met |
| TC-ORDER | team.md Testing Posture、code-generation-plan.md Testing Contract | 受け入れ条件のテストを先に書く | U8 は `2296959` で先にコミット。U1〜U7 は各 code-summary.md の記録のとおり | construction/*/code-generation/code-summary.md | build-and-test | Met |
| TC-VOLUME | team.md Testing Posture、code-generation-plan.md Testing Contract | Standard: 部品ごとに5〜8件の単体テストと、主な境目の結合テスト | U8 だけで最上位のテスト 190 件超。作業単位ごとの件数は各 code-summary.md | construction/*/code-generation/code-summary.md | build-and-test | Met |
| NFR1.1 | u1-walking-skeleton/nfr-requirements/performance-requirements.md | scan と plan は、終わりに、かかった時間と調べたリソースの件数を標準エラーに出す / 検証用アカウントでの実行で、時間と件数が表示される | この対象の確かめが通る | internal/display/display.go | build-and-test | Met |
| NFR1.2 | u1-walking-skeleton/nfr-requirements/performance-requirements.md | 検証用アカウントでの U1 の検証の結果(リージョン、リソースの件数、かかった時間)を、U1 のチェックポイントの記録に残す。U2 が NFR1 を見積もる材料にする / チェック… | この対象の確かめが通る | internal/cli/integration_test.go | build-and-test | Met |
| NFR1.3 | u1-walking-skeleton/nfr-requirements/performance-requirements.md | タグの読み取りと、IAM のポリシーの読み取りは、ページが分かれていても最後まで読む。1件ずつ取り直すような読み方はしない(1ページで取れるものはまとめて取る) / 差し替えた A… | この対象の確かめが通る | internal/awsaccess/read.go | build-and-test | Met |
| NFR1.4 | u1-walking-skeleton/nfr-requirements/scalability-requirements.md | U1 は、1つのリージョンの、タグの付いたリソースを件数の上限なく最後まで読む。メモリの使い方は、リソースの件数に比例する程度にとどめる / 差し替えた AWS で、1万件のリソー… | この対象の確かめが通る | internal/cli/cli_test.go | build-and-test | Met |
| NFR1.5 | u1-walking-skeleton/nfr-requirements/scalability-requirements.md | 影響分析は、付いている管理ポリシーの数に上限を設けずに読む。照合の手間は、ポリシーの条件の数と変更の数の積に比例する程度にとどめる / 差し替えた AWS で、ポリシー500件と変… | この対象の確かめが通る | internal/impact/large_test.go | build-and-test | Met |
| NFR1.6 | u2-scan/nfr-requirements/performance-requirements.md | 1万件のときの AWS の呼び出しの回数が、決まった上限以下である。上限は、リージョンごとに max(1, ⌈n(r) ÷ P_tag⌉) + k + (Config の経路なら … | この対象の確かめが通る | internal/cli/performance_u2_test.go | build-and-test | Met |
| NFR1.7 | u2-scan/nfr-requirements/performance-requirements.md | ツール自身の処理の時間(AWS の待ち時間を除いたもの)が短い / NFR1.6 と同じ偽の AWS(遅れなし)で、scan と plan の合計が CI の各 OS で30秒以内… | この対象の確かめが通る | internal/inventory/discover.go | build-and-test | Met |
| NFR1.8 | u2-scan/nfr-requirements/performance-requirements.md | 検証用アカウントで、1回の呼び出しの時間の95パーセンタイル(L95)を、操作ごとに測る(`--debug` の行の時間から求める)。次の式で1万件の時間を見積もり、U2 のチェッ… | この対象の確かめが通る | scripts/verify.sh | build-and-test | Met |
| NFR1.9 | u2-scan/nfr-requirements/performance-requirements.md | リージョンを同時に読む数は、ツールが決めた固定の数とし、利用者は変えられない(Q2)。L95 の実測はコードの生成のあとでしか得られないため、NFR の設計では、仮の1回の呼び出し… | この対象の確かめが通る | internal/impact/impact.go | build-and-test | Met |
| NFR1.10 | u2-scan/nfr-requirements/performance-requirements.md | 一度もタグが付いていないリソースの検出は、件数の上限なく最後まで読む。Resource Explorer では、件数に上限のない一覧の操作 `ListResources` を使い、… | この対象の確かめが通る | internal/inventory/discover.go | build-and-test | Met |
| NFR1.11 | u3-rules/nfr-requirements/performance-requirements.md | `rules suggest` の、収集を除いたツール自身の処理(キーと値のまとめ、下書きの組み立てと大きさの調整)は、件数に比例する手間で行う / 測るのは、収集の結果(`Inv… | この対象の確かめが通る | internal/rules/performance_test.go | build-and-test | Met |
| NFR1.12 | u3-rules/nfr-requirements/performance-requirements.md | ルールファイルの読み込みと照会(必須タグの値、正しい値、消すかどうか)は、U4 の plan が1万件に使っても時間を食わない / 正しいキー名100個・別名300個・値の別名50… | この対象の確かめが通る | internal/rules/performance_test.go | build-and-test | Met |
| NFR1.13 | u4-plan/nfr-requirements/performance-requirements.md | 変更案の作成(U3 の照会を含む、変更、衝突、値が未指定、消す候補、集計)は、上の条件で2秒以内 / 上の条件のテストで、作成の時間が2秒以内であることを確かめる(照会の時間を含め… | この対象の確かめが通る | internal/planner/performance_test.go | build-and-test | Met |
| NFR1.14 | u4-plan/nfr-requirements/performance-requirements.md | 一覧、差分、JSON、Markdown のそれぞれの出力と、`--out` の保存は、上の条件(置き換えの要る値を半数含む)で3秒以内 / 上の条件のテストで、形式ごとと保存の時間… | この対象の確かめが通る | internal/planner/performance_test.go | build-and-test | Met |
| NFR1.15 | u4-plan/nfr-requirements/performance-requirements.md | 上の条件で保存した変更案(約20 MiB)の読み込みは3秒以内で、読み込みで増えるメモリ(Go の `runtime.MemStats` の `TotalAlloc` の増え)はフ… | この対象の確かめが通る | internal/planner/performance_test.go | build-and-test | Met |
| NFR1.16 | u5-impact/nfr-requirements/performance-requirements.md | 影響分析の呼び出しの回数が、上の式の上限以下である。同じポリシーを2回読まない / 上の条件の偽の AWS で、影響分析の呼び出しの回数を操作ごとに数え、式の上限以下であることを確… | この対象の確かめが通る | internal/impact/large_test.go | build-and-test | Met |
| NFR1.17 | u5-impact/nfr-requirements/performance-requirements.md | ツール自身の処理の時間(AWS の待ち時間を除いたもの)が短い / 上の条件の偽の AWS(遅れなし)で、影響分析が CI の各 OS で10秒以内に終わる | この対象の確かめが通る | internal/impact/large_test.go | build-and-test | Met |
| NFR1.18 | u5-impact/nfr-requirements/performance-requirements.md | ポリシーを同時に読む数は、ツールが決めた固定の数で、利用者は変えられない。IAM は1つの読み取り(`GetAccountAuthorizationDetails`)のページを順に… | この対象の確かめが通る | internal/impact/large_test.go | build-and-test | Met |
| NFR1.19 | u5-impact/nfr-requirements/performance-requirements.md | 検証用アカウントで、U5 で足した操作の1回の呼び出しの時間の95パーセンタイル(L95)を測り(U2 の NFR1.8 と同じく `--debug` の行から求める)、次の式で上… | この対象の確かめが通る | scripts/verify.sh | build-and-test | Met |
| NFR2.1 | u5-impact/nfr-requirements/performance-requirements.md | 影響分析は、機能設計の「検出率100%のテスト用の参照」(ポリシーの種類7 × 参照7 × 変更4、演算子8)のすべてで、期待する判定(affected、undecided、影響な… | この対象の確かめが通る | internal/impact/detection_test.go | build-and-test | Met |
| NFR2.2 | u5-impact/nfr-requirements/performance-requirements.md | 検証用アカウントに置いたテスト用のポリシー(IAM のロールのインラインポリシー、利用者が作った管理ポリシー、少なくとも1つのリソースポリシー)と、それを参照するタグの変更で、pl… | この対象の確かめが通る | internal/cli/integration_u5_test.go | build-and-test | Met |
| NFR3.1 | u1-walking-skeleton/nfr-requirements/security-requirements.md | scan と plan は、読み取りの口(タグの読み取り、ポリシーの読み取り、リージョンと呼び出し元の確認)だけを使い、書き込みの口を作らない(BR1.5) / scan と pl… | この対象の確かめが通る | internal/cli/apply_internal_test.go | build-and-test | Met |
| NFR3.2 | u1-walking-skeleton/nfr-requirements/security-requirements.md | U1 が呼ぶ API だけを並べた「読み取り用の IAM ポリシーの例」をリポジトリに置く。U1 では、タグの読み取り(`tag:GetResources`)、リージョンの確認(`… | この対象の確かめが通る | docs/iam/readonly-policy.json | build-and-test | Met |
| NFR3.3 | u2-scan/nfr-requirements/security-requirements.md | 上の表の IAM の操作を、読み取り用の IAM ポリシーの例(`docs/iam/readonly-policy.json`)と、確認用のテンプレートの読み取り用ロール R2 に… | この対象の確かめが通る | internal/awsaccess/docs_test.go | build-and-test | Met |
| NFR3.4 | u2-scan/nfr-requirements/security-requirements.md | scan と plan は、上の表の API の操作だけを呼び、AWS Config や Resource Explorer の設定を変える操作(記録の開始、索引やビューの作成など… | この対象の確かめが通る | internal/testutil/fakeaws.go | build-and-test | Met |
| NFR3.5 | u2-scan/nfr-requirements/security-requirements.md | 検証用アカウントで、新しく足した API の操作がすべて、例だけを付けたロールで少なくとも1回成功する / U2 のチェックポイントの前に、人が検証用アカウントで Resource… | この対象の確かめが通る | internal/cli/integration_u2_test.go | build-and-test | Met |
| NFR3.6 | u3-rules/nfr-requirements/security-requirements.md | `rules suggest` は、上の3つの読み取りの操作だけを呼び、書き込みの操作、AWS Config と Resource Explorer の操作を呼ばない / HTTP… | この対象の確かめが通る | internal/cli/suggest.go | build-and-test | Met |
| NFR3.7 | u3-rules/nfr-requirements/security-requirements.md | ルールファイルに誤りがあるときは、AWS を1回も呼ばずに終える(scan、plan) / 誤りのあるルールファイルで scan と plan を動かし、偽の AWS が要求を1つ… | この対象の確かめが通る | internal/cli/suggest.go | build-and-test | Met |
| NFR3.8 | u4-plan/nfr-requirements/security-requirements.md | `--format`、`--lang`、`--out` を付けても、呼ぶ AWS の操作は付けないときと同じで、書き込みの操作を呼ばない。`--format`・`--lang` の… | この対象の確かめが通る | internal/cli/options_u4_test.go | build-and-test | Met |
| NFR3.9 | u5-impact/nfr-requirements/security-requirements.md | 影響分析は、上の表と U1 の操作だけを呼び、書き込みの操作を呼ばない。上の表の操作を、読み取り用の IAM ポリシーの例(`docs/iam/readonly-policy.js… | この対象の確かめが通る | internal/awsaccess/docs_test.go | build-and-test | Met |
| NFR3.10 | u6-apply/nfr-requirements/security-requirements.md | 書き込みの操作(`TagResources`、`UntagResources`、S3 への書き込みと削除)は `apply` と `rollback`(U7)だけが呼ぶ。`scan… | この対象の確かめが通る | internal/cli/iac_internal_test.go | build-and-test | Met |
| NFR3.11 | u6-apply/nfr-requirements/security-requirements.md | 書き換え用の IAM ポリシーの例 `docs/iam/apply-policy.json` を作る(Q1)。正本は、NFR の設計で作る「書き換えの対象の種類の表」(種類、付ける… | この対象の確かめが通る | docs/iam/apply-policy.json | build-and-test | Met |
| NFR3.12 | u6-apply/nfr-requirements/security-requirements.md | TagWriter は、`aws:` で始まるキー(大文字と小文字を区別しない。`AWS:`、`Aws:` も)を渡されたら、AWS を呼ばずに InvalidInput を返す(… | この対象の確かめが通る | internal/awsaccess/awsaccess.go | build-and-test | Met |
| NFR3.13 | u7-rollback/nfr-requirements/security-requirements.md | rollback は U6 の書き換え用の例 `docs/iam/apply-policy.json` で動く。新しい例は作らない。例の説明と README に「apply と r… | この対象の確かめが通る | internal/cli/iac_internal_test.go | build-and-test | Met |
| NFR3.14 | u7-rollback/nfr-requirements/security-requirements.md | rollback が書くのは、記録の `touchedKeys` のキーだけ。ほかのキーと `aws:` で始まるキーは書きも消しもしない(機能設計 BR3.4)。TagWrite… | この対象の確かめが通る | internal/apply/rollback_target.go | build-and-test | Met |
| NFR3.15 | u7-rollback/nfr-requirements/security-requirements.md | rollback は記録の中身を、書き込みの前に確かめる。記録の置き場所(S3 や共有のフォルダー)に書ける別の人が、書き換えの権限を持つ利用者に任意のタグを付けさせないため。確か… | この対象の確かめが通る | internal/apply/rollback_check.go | build-and-test | Met |
| NFR3.16 | u8-iac/nfr-requirements/security-requirements.md | `iac` は AWS を呼ばず、AWS の認証情報を読まない。AwsAccess の接続を作らず、書き込みの口(TagWriter、ObjectStore)も受け取らない(U6 … | この対象の確かめが通る | internal/cli/iac_internal_test.go | build-and-test | Met |
| NFR3.17 | u8-iac/nfr-requirements/security-requirements.md | 書き換えたファイルで変わるのは、Edit の範囲の文字だけ。範囲の外のバイト(コメント、ほかの行、インデント、改行の種類、BOM、ファイルの終わりの改行の有無)は1バイトも変えない… | この対象の確かめが通る | internal/iac/changes_test.go | build-and-test | Met |
| NFR3.18 | u8-iac/nfr-requirements/security-requirements.md | 書き換えるのは `--dir` の下の通常のファイルだけ。`--dir` の下のシンボリックリンクと Windows のジャンクション(ファイルもフォルダーも)はたどらず書かない。… | この対象の確かめが通る | internal/iac/walk_test.go | build-and-test | Met |
| NFR3.19 | u8-iac/nfr-requirements/security-requirements.md | `--dir` のファイルの中身は信用せず、時間とメモリを使い切らない。(a) 数えるのは候補の拡張子(.yaml、.yml、.json、.template、.tf)のファイルだけ… | この対象の確かめが通る | internal/iac/walk_test.go | build-and-test | Met |
| NFR3.20 | u8-iac/nfr-requirements/security-requirements.md | `git` のコマンドは、シェルを通さず、決まった引数と環境で呼ぶ。(a) 環境から `GIT_` で始まる変数をすべて除き、`GIT_OPTIONAL_LOCKS=0`(インデッ… | この対象の確かめが通る | internal/iac/git_test.go | build-and-test | Met |
| NFR3.21 | u8-iac/nfr-requirements/security-requirements.md | 検証用アカウントでも反映を確かめる。U8 のチェックポイントで、U5・U6 の確認用のスタック(`test/sandbox/` のテンプレートから作ったもの)に対して、確かめ専用の… | この対象の確かめが通る | scripts/verify.sh | build-and-test | Met |
| NFR4.1 | u6-apply/nfr-requirements/security-requirements.md | 記録には、U7 が書き換え前のタグに完全に戻すのに必要なもの(`touchedKeys`、書き換える直前に読んだ `beforeTags`(`aws:` を含む全体)、`plann… | この対象の確かめが通る | internal/journal/journal_test.go | build-and-test | Met |
| NFR4.2 | u7-rollback/nfr-requirements/security-requirements.md | apply のあと別の変更が入っていなければ、rollback でタグが apply の前と完全に一致する(差分0。`aws:` のキーを含むすべてのタグ)。偽の AWS で次のす… | この対象の確かめが通る | internal/apply/rollback_scenario_test.go | build-and-test | Met |
| NFR4.3 | u7-rollback/nfr-requirements/security-requirements.md | 検証用アカウントでも差分0を確かめる。U7 のチェックポイントで、確かめ専用のキーの追加、値の変更、キー名の変更を apply してから rollback し、5つの種類のリソース… | この対象の確かめが通る | scripts/verify.sh | build-and-test | Met |
| NFR4.4 | u7-rollback/nfr-requirements/security-requirements.md | apply のあとに他人が変えた項目は、`--force` なしでは書かない。判定は、印を取ってから読んだ記録と、読み直しの単位ごとの読み直しで行う(機能設計 BR2.1、BR3.… | この対象の確かめが通る | internal/apply/rollback_target.go | build-and-test | Met |
| NFR5.1 | u1-walking-skeleton/nfr-requirements/reliability-requirements.md | すべての AWS の呼び出しに時間の上限を設ける。U1 の上限は、1回の呼び出しにつき30秒とする / 応答しない AWS を差し替えたテストで、上限の後に Transient で… | この対象の確かめが通る | internal/awsaccess/session.go | build-and-test | Met |
| NFR5.2 | u1-walking-skeleton/nfr-requirements/reliability-requirements.md | 一時的な失敗は、AWS へのアクセスの部品の既定の再試行で扱い、使い切ったら Transient として終了コード 1 で終える。待ち時間を延ばす再試行と、呼び出し回数の上限への本… | この対象の確かめが通る | internal/awsaccess/invoke.go | build-and-test | Met |
| NFR5.3 | u1-walking-skeleton/nfr-requirements/reliability-requirements.md | 結果が不完全なとき(ポリシーを読めなかったなど)は、黙って「影響なし」にせず、不完全であることを示して終了コード 1 にする(BR5.5、BR6.3) / ポリシーの読み取りを拒否… | この対象の確かめが通る | internal/impact/impact.go | build-and-test | Met |
| NFR5.4 | u2-scan/nfr-requirements/performance-requirements.md | すべての AWS の呼び出し(タグの取得、AWS Config、Resource Explorer、IAM)で、呼び出し回数の上限の応答のとき、待ち時間を延ばしながら(揺らぎを入れ… | この対象の確かめが通る | internal/awsaccess/invoke.go | build-and-test | Met |
| NFR5.5 | u2-scan/nfr-requirements/performance-requirements.md | 時間の上限は、1回の試行に30秒(U1 の NFR5.1 のまま)、再試行を含めた1回の呼び出しに5分とする。U1 の NFR の設計(`construction/u1-walki… | この対象の確かめが通る | internal/awsaccess/session.go | build-and-test | Met |
| NFR5.6 | u2-scan/nfr-requirements/performance-requirements.md | 1つのリージョンの失敗は、他のリージョンの収集を止めない(BR4.1)。一度もタグが付いていないリソースの検出の失敗は、タグの取得の結果を不完全にしない(BR3.2)。Resour… | この対象の確かめが通る | internal/inventory/discover.go | build-and-test | Met |
| NFR5.7 | u2-scan/nfr-requirements/performance-requirements.md | 同時に読んでも、標準出力の結果は実行のたびに同じになる。表と Summary の並びは決まった鍵で並べ、Config と Resource Explorer が同じ ARN を返し… | この対象の確かめが通る | internal/impact/large_test.go | build-and-test | Met |
| NFR5.8 | u2-scan/nfr-requirements/performance-requirements.md | 認証情報の誤り(なし、期限切れ)は、どの段階(タグの取得、検出、plan の IAM の読み取り)で起きても全体を止め、ほかのリージョンの読み取りも取り消し、標準出力に何も出さない… | この対象の確かめが通る | internal/inventory/discover.go | build-and-test | Met |
| NFR5.9 | u3-rules/nfr-requirements/performance-requirements.md | `rules suggest` は、U2 の収集と同じ再試行と時間の上限で動く。呼び出し回数の上限が続いても最後まで終わり、1つのリージョンが読めなくても下書きを出して終了コード … | この対象の確かめが通る | internal/cli/suggest.go | build-and-test | Met |
| NFR5.10 | u5-impact/nfr-requirements/security-requirements.md | 一覧や1つのポリシーを読めなかったことで、ほかの種類、リージョン、ポリシーの読み取りを止めない。止めるのは、認証情報の誤りと、Ctrl+C などでの取り消しだけとする(U2 の B… | この対象の確かめが通る | internal/cli/impact_u5_test.go | build-and-test | Met |
| NFR5.11 | u5-impact/nfr-requirements/performance-requirements.md | 同時に読んでも、指摘と読めなかったものの順、標準出力、保存した変更案(`planId` と `createdAt` を除く)は、実行のたびに同じになる(機能設計 BR5.2) / … | この対象の確かめが通る | internal/impact/large_test.go | build-and-test | Met |
| NFR5.12 | u6-apply/nfr-requirements/tech-stack-decisions.md | 書き込みの呼び出しも、U2 の AwsAccess の時間の上限、待ち時間を延ばしながらの再試行、呼び出し回数の上限に当たったときの待ちを通す。呼び出し側は再試行しない(契約 C6… | この対象の確かめが通る | internal/awsaccess/write_test.go | build-and-test | Met |
| NFR5.13 | u6-apply/nfr-requirements/tech-stack-decisions.md | 書き込みは、上の「書き込みの進め方」のとおりに、まとまり(20以下)、読み直しの単位(100以下)、リージョン4つまでの同時、リージョンの中は順で行う(Q2) / 偽の AWS で… | この対象の確かめが通る | internal/apply/apply_test.go | build-and-test | Met |
| NFR5.14 | u6-apply/nfr-requirements/tech-stack-decisions.md | apply の呼び出しの回数は、次の式の上限以下である。記号: N(r) = リージョン r の変更のあるリソースの数、R(r) = リージョン r の読み直しの単位の数(R(r)… | この対象の確かめが通る | internal/apply/large_test.go | build-and-test | Met |
| NFR5.15 | u6-apply/nfr-requirements/tech-stack-decisions.md | 測る条件の「よくある変更」(下の表)で、1アカウント・全リージョン・1万リソースの apply の見積もりが30分以内。「値がばらばらなキー名の変更」(下の表。まとまりがほぼ1リソ… | この対象の確かめが通る | scripts/verify.sh | build-and-test | Met |
| NFR5.16 | u6-apply/nfr-requirements/tech-stack-decisions.md | 偽の AWS(遅れなし)で、測る条件の「よくある変更」と「値がばらばらなキー名の変更」の1万リソースの apply を動かしたとき、ツール自身の処理の時間(まとまりの計算、記録の保… | この対象の確かめが通る | internal/apply/large_test.go | build-and-test | Met |
| NFR5.17 | u6-apply/nfr-requirements/tech-stack-decisions.md | 書き込みの途中で認証情報の誤り(期限切れのトークンを含む。契約 C12)を受けたとき、新しい書き込みを止める。また、書き込みの呼び出し(`TagResources`・`UntagR… | この対象の確かめが通る | internal/apply/apply_test.go | build-and-test | Met |
| NFR5.18 | u7-rollback/nfr-requirements/tech-stack-decisions.md | rollback の読み取りと書き込みも、U2 の AwsAccess の時間の上限、再試行、回数の上限に当たったときの待ちを通し、`TagResources`・`UntagRes… | この対象の確かめが通る | internal/apply/execution.go | build-and-test | Met |
| NFR5.19 | u7-rollback/nfr-requirements/tech-stack-decisions.md | 書き戻しは、上の「書き戻しの進め方」のとおりに、読み直しの単位(100以下、ARN の順に先に切る)、単位の中のまとまり(20以下)、リージョン4つまでの同時、リージョンの中は順で… | この対象の確かめが通る | internal/apply/rollback_worker.go | build-and-test | Met |
| NFR5.20 | u7-rollback/nfr-requirements/tech-stack-decisions.md | rollback の呼び出しの回数は、次の式の上限以下である。記号: N(r) = リージョン r の判定する項目の数、R(r) = ⌈N(r) ÷ 100⌉(読み直しの単位の数)… | この対象の確かめが通る | internal/apply/large_rollback_test.go | build-and-test | Met |
| NFR5.21 | u7-rollback/nfr-requirements/tech-stack-decisions.md | 測る条件の「よくある変更」の apply を戻す場合に、1アカウント・全リージョン・1万リソースの rollback の見積もりが30分以内。「値がばらばらなキー名の変更」の ap… | この対象の確かめが通る | scripts/verify.sh | build-and-test | Met |
| NFR5.22 | u7-rollback/nfr-requirements/tech-stack-decisions.md | 偽の AWS(遅れなし)で、測る条件の2つの分布の1万リソースの apply の記録を戻したとき、ツール自身の処理の時間(判定、単位とまとまりの計算、記録の保存を含む)が、それぞれ… | この対象の確かめが通る | internal/apply/large_rollback_test.go | build-and-test | Met |
| NFR5.23 | u7-rollback/nfr-requirements/tech-stack-decisions.md | 書き戻しの途中で認証情報の誤り(期限切れを含む)を受けたとき、新しい書き込みを止める。書き込みの呼び出しが、再試行を使い切っても Transient で終わることが全リージョンを通… | この対象の確かめが通る | internal/apply/execution.go | build-and-test | Met |
| NFR5.24 | u7-rollback/nfr-requirements/tech-stack-decisions.md | 書いた直後の読み戻しで、`touchedKeys` の値が `beforeTags` と違った項目は、その項目だけを1秒、2秒、4秒おいて最大3回読み直す(1つの単位で最大7秒)。… | この対象の確かめが通る | internal/apply/rollback_worker.go | build-and-test | Met |
| NFR5.25 | u8-iac/nfr-requirements/tech-stack-decisions.md | 10,000 リソースの変更案(変更 約 30,000。CloudFormation と Terraform に半分ずつ)と、`--dir` に 2,000 個のテンプレートと `… | この対象の確かめが通る | internal/iac/large_test.go | build-and-test | Met |
| NFR5.26 | u8-iac/nfr-requirements/tech-stack-decisions.md | ファイルを読む回数と解析する回数を抑える。候補のファイルはそれぞれ1回だけ読んで解析し、論理 ID とアドレスの索引を作ってから場所を決める(リソースごとにファイルを読み直さない)… | この対象の確かめが通る | internal/iac/large_test.go | build-and-test | Met |
| NFR6.1 | u6-apply/nfr-requirements/security-requirements.md | 記録は、書き換えより先に、途中で壊れない方法で保存する。手元のファイルは同じフォルダーの一時ファイルに書き、ディスクへの書き出しを終えてから名前を変えて置き換える。S3 は1回の書… | この対象の確かめが通る | internal/journal/journal_test.go | build-and-test | Met |
| NFR6.2 | u6-apply/nfr-requirements/security-requirements.md | 書き換えの途中は、記録を5秒ごとと、止めたとき・終わったときに保存する(手元のファイルでも S3 でも同じ。Q4)。5秒ごとの保存は書き込みと並行して行い、書き込みを待たせない。前… | この対象の確かめが通る | internal/journal/saver.go | build-and-test | Met |
| NFR6.3 | u7-rollback/nfr-requirements/security-requirements.md | rollback の結果の保存は、U6 と同じく途中で壊れない方法で行う(U6 の NFR6.1)。印を取ったあとに読んだ記録を、このツールが知らない項目を生の JSON のまま含… | この対象の確かめが通る | internal/journal/rollback.go | build-and-test | Met |
| NFR6.4 | u7-rollback/nfr-requirements/security-requirements.md | 書き戻しの途中は、結果を5秒ごとと、止めたとき・終わったときに保存する。前の保存が終わっていなければ、その回は飛ばす。3回続けて失敗したら新しい書き込みを止める。止めたとき・終わっ… | この対象の確かめが通る | internal/journal/saver.go | build-and-test | Met |
| NFR6.5 | u8-iac/nfr-requirements/security-requirements.md | `--write` の書き出しは壊れない方法で行う(U6 の NFR6.1 と同じ考え)。同じフォルダーに推測されにくい名前の一時ファイルを、利用者だけが読める権限で作って書き、書… | この対象の確かめが通る | internal/iac/write_test.go | build-and-test | Met |
| NFR6.6 | u8-iac/nfr-requirements/security-requirements.md | 書き出す前に、書き換えるすべてのファイルが読んだときから変わっていないことを確かめ、1つでも違えばどれも書かない(機能設計 BR5.3)。書き出しの途中で失敗したら、書いたファイル… | この対象の確かめが通る | internal/iac/write_test.go | build-and-test | Met |
| NFR7.1 | u6-apply/nfr-requirements/security-requirements.md | 同じアカウントの apply と rollback の同時実行を、印で防ぐ。手元の印は「なければ作る」(作るときに、すでにあれば失敗する)方法で作り、S3 の印は条件付きの書き込み… | この対象の確かめが通る | internal/journal/lock.go | build-and-test | Met |
| NFR7.2 | u7-rollback/nfr-requirements/security-requirements.md | rollback は、記録を読む前に、apply と同じ印(目的 `rollback`、runId に rollbackId)を取り、どの終わり方でも消す(機能設計 BR2.1、B… | この対象の確かめが通る | internal/apply/execution.go | build-and-test | Met |
| NFR8.1 | u1-walking-skeleton/nfr-requirements/security-requirements.md | 認証情報(アクセスキー、シークレット、セッショントークン)を、標準出力、標準エラー、`--debug` の表示のどこにも出さない。AWS の SDK のエラー文をそのまま出さず、エ… | この対象の確かめが通る | internal/awsaccess/invoke.go | build-and-test | Met |
| NFR8.2 | u1-walking-skeleton/nfr-requirements/security-requirements.md | `--debug` の表示は、API の名前、リージョン、かかった時間、再試行の回数だけとし、タグの値やポリシーの中身を出さない(Q3) / `--debug` のテストで、タグの… | この対象の確かめが通る | internal/display/display.go | build-and-test | Met |
| NFR8.3 | u1-walking-skeleton/nfr-requirements/security-requirements.md | 認証情報はツールが保存せず、AWS の標準の設定(環境変数、プロファイル、SSO)から AWS の SDK が読む。ツールは認証情報を受け取るオプションを持たない / オプションの… | この対象の確かめが通る | internal/awsaccess/session.go | build-and-test | Met |
| NFR8.4 | u1-walking-skeleton/nfr-requirements/security-requirements.md | 外から受け取る入力(オプション、ルールファイル、AWS の応答のポリシー文書)は、境目で確かめる。ルールファイルは YAML の安全な読み方(別名の展開の上限、深い入れ子の拒否など… | この対象の確かめが通る | internal/rules/rules.go | build-and-test | Met |
| NFR8.5 | u1-walking-skeleton/nfr-requirements/security-requirements.md | 依存ライブラリの既知の脆弱性を、CI で `govulncheck` を使って確かめ、見つかればマージしない(チームの進め方 §Code Style) / CI に govulnc… | この対象の確かめが通る | .github/workflows/ci.yml | build-and-test | Met |
| NFR8.6 | u1-walking-skeleton/nfr-requirements/security-requirements.md | AWS から読んだ文字列(タグのキーと値、ARN、ポリシーの文字列)を端末に出す前に、制御文字(ANSI のエスケープを含む)を見える形(例 `\x1b`)に置き換える。他人がタグ… | この対象の確かめが通る | internal/display/display.go | build-and-test | Met |
| NFR8.7 | u1-walking-skeleton/nfr-requirements/security-requirements.md | CI と依存の供給元を固定する。GitHub Actions はコミットの SHA で固定し、Dependabot は Go の依存と GitHub Actions の両方を対象に… | この対象の確かめが通る | .github/workflows/ci.yml | build-and-test | Met |
| NFR8.8 | u1-walking-skeleton/nfr-requirements/security-requirements.md | ツールは、AWS の API と、AWS の SDK の標準の認証情報の解決が使う宛先(SSO、STS、EC2 のメタデータなど)以外へ通信しない(利用の統計や更新の確認をしない)… | この対象の確かめが通る | internal/awsaccess/session.go | build-and-test | Met |
| NFR8.9 | u2-scan/nfr-requirements/security-requirements.md | AWS Config と Resource Explorer の応答から作る文字列(ARN、種類の名前、リージョン、検出できなかった理由)と、進み具合の行に出す文字列は、U1 の … | この対象の確かめが通る | internal/cli/acceptance_u2_test.go | build-and-test | Met |
| NFR8.10 | u2-scan/nfr-requirements/security-requirements.md | 検出できなかった理由やリージョンの失敗の理由には、エラーの種類と操作の名前だけを出し、AWS の SDK のエラー文は出さない(U1 の NFR8.1) / 偽の認証情報と、拒否・… | この対象の確かめが通る | internal/awsaccess/invoke.go | build-and-test | Met |
| NFR8.11 | u2-scan/nfr-requirements/security-requirements.md | 収集の途中で認証情報の期限が切れたときは、それまでに読んだ結果を標準出力に出さずに終える。部分的な結果を、完全な結果と取り違えさせないため(信頼性の NFR5.8 と同じ振る舞いを… | この対象の確かめが通る | internal/awsaccess/invoke.go | build-and-test | Met |
| NFR8.12 | u3-rules/nfr-requirements/security-requirements.md | 下書きのキーと値は、印字できない文字(Unicode の制御文字 Cc、書式の文字 Cf、行と段落の区切り Zl・Zp)と不正な UTF-8 を、YAML のエスケープで書く。基本… | この対象の確かめが通る | internal/rules/suggest.go | build-and-test | Met |
| NFR8.13 | u3-rules/nfr-requirements/security-requirements.md | ルールファイルの読み込みは、どんな入力でも止まらず(パニックせず)、64 KiB の入力を1秒以内に読み終えるか、誤りを返す / Go のファジングのテスト(種の入力: 正しいファ… | この対象の確かめが通る | internal/rules/fuzz_test.go | build-and-test | Met |
| NFR8.14 | u3-rules/nfr-requirements/security-requirements.md | ルールファイルの中身から作る誤りと警告の文言(キー、値、行)は、U1 の表示の前の置き換えを通してから標準エラーに出す / 制御文字と U+202E を含むキーを書いたルールファイ… | この対象の確かめが通る | internal/cli/suggest.go | build-and-test | Met |
| NFR8.15 | u4-plan/nfr-requirements/security-requirements.md | 一覧、差分、JSON の出力で、データから来る文字列(キー、値、ARN、ポリシーの名前、ステートメントの ID、条件、失敗したリージョンの理由)は、端末を書き換える文字をそのまま出… | この対象の確かめが通る | internal/display/impact_test.go | build-and-test | Met |
| NFR8.16 | u4-plan/nfr-requirements/security-requirements.md | Markdown の出力に出る、データから来る文字列のすべて(表のセル、`## 対象` の行、消す候補などの一覧、影響の finding のポリシーの名前やステートメントの ID、… | この対象の確かめが通る | internal/display/impact_test.go | build-and-test | Met |
| NFR8.17 | u4-plan/nfr-requirements/security-requirements.md | 保存した変更案のファイルは、本人だけが読み書きできる権限(Unix の 0600)で作る。Windows では既定の権限(フォルダから受け継ぐ ACL)に任せ、確かめない。既にある… | この対象の確かめが通る | internal/fileout/fileout.go | build-and-test | Met |
| NFR8.18 | u4-plan/nfr-requirements/security-requirements.md | 保存した変更案の読み込みは、256 MiB を超えるファイルを全体を読まずに誤りにし、どんな入力でも止まらず(パニックせず)、誤りを返すか読み終える。UTF-8 の BOM は読み… | この対象の確かめが通る | internal/planner/saved.go | build-and-test | Met |
| NFR8.19 | u4-plan/nfr-requirements/security-requirements.md | 保存した変更案と、JSON・Markdown の出力には、契約 C2・C3 に書いた項目(U4 で足した項目を含む)だけを書き、認証情報、プロファイルの設定、環境変数の値を書かない… | この対象の確かめが通る | internal/cli/options_u4_test.go | build-and-test | Met |
| NFR8.20 | u5-impact/nfr-requirements/security-requirements.md | ポリシーの文書は、1 MiB(1,048,576 バイト)を超えるものを JSON として読まずに「読めなかった」(理由は大きすぎる)にし、影響分析を不完全にする。大きさは、IAM… | この対象の確かめが通る | internal/impact/policy.go | build-and-test | Met |
| NFR8.21 | u5-impact/nfr-requirements/security-requirements.md | 文書の読み取り(URL エンコードを戻すこと、JSON として読むこと、条件、`Resource`、ポリシー変数の取り出し)は、どんな入力でも止まらず(パニックせず)、誤りを返すか… | この対象の確かめが通る | internal/impact/fuzz_test.go | build-and-test | Met |
| NFR8.22 | u5-impact/nfr-requirements/security-requirements.md | ポリシーの文書の中身は、`--debug` にも、読めなかった理由にも出さない。読めなかった理由は、エラーの種類と操作の名前だけにする(機能設計 BR1.5)。指摘に出す条件の値、… | この対象の確かめが通る | internal/cli/impact_u5_test.go | build-and-test | Met |
| NFR8.23 | u6-apply/nfr-requirements/security-requirements.md | 認証情報を、記録、ログファイル、印、表示、`--debug` の行に出さない。ログファイルには、ARN、リージョン、status、理由、誤りの種類、操作の名前だけを書き、AWS の… | この対象の確かめが通る | internal/journal/run.go | build-and-test | Met |
| NFR8.24 | u6-apply/nfr-requirements/security-requirements.md | 記録(ARN とタグの値が入る)と、ログファイル、印、一時ファイルを置く手元のフォルダーは、その利用者だけが読み書きできる権限にする(Q5)。Linux と macOS は、ツール… | この対象の確かめが通る | internal/journal/store.go | build-and-test | Met |
| NFR8.25 | u6-apply/nfr-requirements/security-requirements.md | S3 に置く記録と印には、公開の ACL を付けない。暗号化はバケットの既定の設定に任せ、ツールからは指定しない。S3 のオブジェクトの名前は、`--record-to` の前置き… | この対象の確かめが通る | internal/awsaccess/objectstore.go | build-and-test | Met |
| NFR8.26 | u7-rollback/nfr-requirements/security-requirements.md | 認証情報、AWS のエラー文を、記録、ログファイル、印、表示、`--debug` の行に出さない。ログファイルにはタグの値を書かない(U6 の NFR8.23 と同じ。機能設計 B… | この対象の確かめが通る | internal/apply/rollback_test.go | build-and-test | Met |
| NFR8.27 | u8-iac/nfr-requirements/security-requirements.md | `iac` が出すのは、差分(Edit のある行と前後3行)、件数、一覧、案内だけ。出さないもの: 差分の外のファイルの中身、`--tf-state` の中身(リソースの属性の値。… | この対象の確かめが通る | internal/iac/write_test.go | build-and-test | Met |
| NFR9.1 | u1-walking-skeleton/nfr-requirements/reliability-requirements.md | Linux、macOS、Windows の CI で、同じテストがすべて通る。改行コードやパスの違いで結果が変わらない(ルールファイルの CRLF も読める。要件 AS4) / 3… | この対象の確かめが通る | internal/cli/acceptance_u2_test.go | build-and-test | Met |
| NFR9.2 | u1-walking-skeleton/nfr-requirements/reliability-requirements.md | 1つの実行ファイルにまとめ、実行に他のファイルや実行環境を要しない。C のライブラリに頼らない形(`CGO_ENABLED=0`)でビルドする / 3つの OS の CI で、`C… | この対象の確かめが通る | .github/workflows/ci.yml | build-and-test | Met |
| NFR9.3 | u2-scan/nfr-requirements/performance-requirements.md | Windows では、書き換えの表示に必要な端末の設定(仮想端末の処理)を有効にできたときだけ書き換え、できなければ追記に戻る。Git Bash などで端末と判定できないときも追記… | この対象の確かめが通る | internal/display/terminal_windows.go | build-and-test | Met |
| NFR9.4 | u3-rules/nfr-requirements/performance-requirements.md | 下書きは、すべての OS で LF の改行で出す。同じ収集の結果からは、OS によらず同じバイト列になる(日時の行を除く) / 同じ偽の AWS の結果で作った下書きが、3つの O… | この対象の確かめが通る | internal/rules/suggest.go | build-and-test | Met |
| NFR9.5 | u3-rules/nfr-requirements/performance-requirements.md | Windows PowerShell 5.1 のリダイレクト(`rules suggest > tagrules.yaml`)が書く UTF-16(BOM 付き)のファイルも、ルー… | この対象の確かめが通る | internal/rules/encoding.go | build-and-test | Met |
| NFR9.6 | u4-plan/nfr-requirements/performance-requirements.md | 同じ入力からは、3つの OS で同じバイト列(改行は LF、UTF-8、BOM なし)の一覧、差分、JSON、Markdown と、保存した変更案(`planId` と `crea… | この対象の確かめが通る | internal/display/golden_test.go | build-and-test | Met |
| NFR9.7 | u4-plan/nfr-requirements/performance-requirements.md | 言語の決め方は、3つの OS で同じ規則で動く(`--lang`、`LC_ALL`、`LC_MESSAGES`、`LANG` の順。Windows だけ、3つの環境変数が空のときに… | この対象の確かめが通る | internal/display/lang.go | build-and-test | Met |
| NFR9.8 | u6-apply/nfr-requirements/tech-stack-decisions.md | 手元の印、壊れない保存、ホームのフォルダー、Ctrl+C、確認の入力は、Linux、macOS、Windows で同じに動く / 3つの OS の CI で、印の競争、保存の途中で… | この対象の確かめが通る | internal/journal/journal_test.go | build-and-test | Met |
| NFR9.9 | u6-apply/nfr-requirements/tech-stack-decisions.md | Windows で、名前を変えて置き換える先のファイルを別のプロセス(読み取り中の rollback、ウイルス対策のソフトなど)が開いていて失敗したときは、100ミリ秒おいて最大5… | この対象の確かめが通る | internal/fileout/replace_test.go | build-and-test | Met |
| NFR9.10 | u7-rollback/nfr-requirements/tech-stack-decisions.md | rollback の印、壊れない保存、確認の入力、Ctrl+C は、Linux、macOS、Windows で同じに動く(U6 の NFR9.8、NFR9.9 の部品をそのまま使う… | この対象の確かめが通る | internal/cli/rollback.go | build-and-test | Met |
| NFR9.11 | u8-iac/nfr-requirements/security-requirements.md | `iac` は3つの OS で同じ結果になる。パスは `--dir` からの相対パスを `/` で区切って示す。改行は LF と CRLF のどちらのファイルも扱い、ファイルごとの… | この対象の確かめが通る | internal/iac/write_test.go | build-and-test | Met |
| NFR9.12 | u8-iac/nfr-requirements/tech-stack-decisions.md | 新しい依存 hcl/v2 も3つの OS で、CI の Go の2つの版(stable と oldstable)でビルドでき、CGO を使わない(単一の実行ファイルで配る。要件 N… | この対象の確かめが通る | .github/workflows/ci.yml | build-and-test | Met |
| NFR10.1 | u1-walking-skeleton/nfr-requirements/reliability-requirements.md | テストのカバレッジは、Go の文単位で80%以上。コマンドの入口などの配線部分も計測の対象に含める / CI でカバレッジを測り、80%未満なら失敗する | この対象の確かめが通る | .github/workflows/ci.yml | build-and-test | Met |
| NFR10.2 | u1-walking-skeleton/nfr-requirements/reliability-requirements.md | 受け入れ条件(Given/When/Then)を先に自動テストにし、実装の後に細かい単体テストを書く(チームの進め方 §Testing Posture) / Bolt のブランチ(… | この対象の確かめが通る | internal/cli/acceptance_u2_test.go | build-and-test | Met |
| NFR10.3 | u2-scan/nfr-requirements/performance-requirements.md | U2 で足す大きな偽の AWS のテスト(NFR1.6、NFR1.7、NFR5.7)は、CI の各 OS で合わせて2分以内に終わる。カバレッジは U1 と同じく `cmd` と … | この対象の確かめが通る | .github/workflows/ci.yml | build-and-test | Met |
| NFR10.4 | u3-rules/nfr-requirements/performance-requirements.md | 下書きを読み込むと、元のキー、別名、値の候補に戻る / ランダムに作ったキーと値(制御文字、引用符、`:`、`#`、空白、日本語、128文字のキー、256文字の値を含む)から作った… | この対象の確かめが通る | internal/rules/roundtrip_test.go | build-and-test | Met |
| NFR10.5 | u3-rules/nfr-requirements/performance-requirements.md | U3 のテストを足しても、カバレッジは `cmd` と `internal` のすべてで80%以上のまま / CI のカバレッジのジョブで確かめる | この対象の確かめが通る | .github/workflows/ci.yml | build-and-test | Met |
| NFR10.6 | u4-plan/nfr-requirements/performance-requirements.md | 4つの形式は、同じ変更案から同じ件数(変更の種類ごとの件数、総数、衝突、値が未指定、消す候補のキーの数、IaC、影響あり、判定できない)を出す / 乱数の種を固定して作った変更案(… | この対象の確かめが通る | internal/display/counts_test.go | build-and-test | Met |
| NFR10.7 | u4-plan/nfr-requirements/performance-requirements.md | 英語と日本語の文言は、すべての文言にどちらの言語もあり、差し込む値(件数、キー、ARN など)の数と順が同じ / 文言の一覧のすべての項目に英語と日本語があり、差し込みの印が一致す… | この対象の確かめが通る | internal/display/messages_test.go | build-and-test | Met |
| NFR10.8 | u4-plan/nfr-requirements/performance-requirements.md | U4 のテストを足しても、カバレッジは `cmd` と `internal` のすべてで80%以上のまま / CI のカバレッジのジョブで確かめる | この対象の確かめが通る | .github/workflows/ci.yml | build-and-test | Met |
| NFR10.9 | u5-impact/nfr-requirements/performance-requirements.md | U5 のテストを足しても、カバレッジは `cmd` と `internal` のすべてで80%以上のまま。U5 で足す大きな偽の AWS のテスト(NFR1.16、NFR1.17、… | この対象の確かめが通る | .github/workflows/ci.yml | build-and-test | Met |
| NFR10.10 | u6-apply/nfr-requirements/tech-stack-decisions.md | U6 のテストを足しても、カバレッジは `cmd` と `internal` のすべてで80%以上のまま。U6 のテストは偽の AWS で動き、AWS の認証情報なしで CI で実… | この対象の確かめが通る | internal/testutil/tagaccount.go | build-and-test | Met |
| NFR10.11 | u7-rollback/nfr-requirements/tech-stack-decisions.md | U7 のテストを足しても、カバレッジは `cmd` と `internal` のすべてで80%以上のまま。U7 のテストは偽の AWS で動き、AWS の認証情報なしで CI で実… | この対象の確かめが通る | internal/apply/rollback_test.go | build-and-test | Met |
| NFR10.12 | u8-iac/nfr-requirements/security-requirements.md | U8 のテストを足しても、カバレッジは `cmd` と `internal` のすべてで80%以上。U8 で足したパッケージ(と、U8 で足したファイル)だけでも80%以上。テスト… | この対象の確かめが通る | .github/workflows/ci.yml | build-and-test | Met |
| NFR10.13 | u8-iac/nfr-requirements/tech-stack-decisions.md | 新しい依存 hcl/v2 も、既存の CI の `govulncheck` と Dependabot の対象になり、既知の脆弱性で呼ばれるものがない。足した間接の依存の数と、実行フ… | この対象の確かめが通る | go.mod | build-and-test | Met |
| NFR10.14 | u8-iac/nfr-requirements/tech-stack-decisions.md | 機能設計から送られた確かめをテストにする: (a) CloudFormation の Type と resourceType の対応表のすべての行を数えて確かめる(apply が書… | この対象の確かめが通る | internal/restype/restype_test.go | build-and-test | Met |
| NFR11.1 | u1-walking-skeleton/nfr-requirements/observability-requirements.md | 結果の意味は、記号(`~`、`!`、`?`)と文字で表す。U1 は色を使わない(BR6.2) / 色のない端末で、すべての結果の意味が分かる | この対象の確かめが通る | internal/display/results.go | build-and-test | Met |
| NFR11.2 | u1-walking-skeleton/nfr-requirements/observability-requirements.md | 進み具合(リージョンの読み始めと読み終わり、件数)と、アカウント、リージョン、終了コードの行を標準エラーに出す(BR6.1) / 標準出力をファイルに保存したとき、ファイルにこれら… | この対象の確かめが通る | internal/display/display.go | build-and-test | Met |
| NFR11.3 | u1-walking-skeleton/nfr-requirements/observability-requirements.md | `--debug` を付けると、呼んだ AWS の API の名前、リージョン、かかった時間、再試行の回数を、1回の呼び出しにつき1行で標準エラーに出す(Q3)。認証情報、タグの値… | この対象の確かめが通る | internal/display/display.go | build-and-test | Met |
| NFR11.4 | u1-walking-skeleton/nfr-requirements/observability-requirements.md | エラーは2行(何が起きたか、どう直すか)で標準エラーに出す(機能設計 W3) / 各エラーの種類のテストで、2行の案内が出る | この対象の確かめが通る | internal/cli/messages.go | build-and-test | Met |
| NFR11.5 | u2-scan/nfr-requirements/performance-requirements.md | リージョンごとの状態(済み、実行中、待ち、失敗)と経過時間、検出の段階を、標準エラーに出す。標準エラーが端末で、かつ書き換えの表示が使えるときだけ同じ場所を書き換え、それ以外は1行… | この対象の確かめが通る | internal/display/progress.go | build-and-test | Met |
| NFR11.6 | u2-scan/nfr-requirements/performance-requirements.md | `--debug` の1行に、AWS Config と Resource Explorer の呼び出しも含める(U1 の NFR11.3 の形のまま) / Config と Res… | この対象の確かめが通る | internal/display/display.go | build-and-test | Met |
| NFR11.7 | u4-plan/nfr-requirements/performance-requirements.md | 色を付けた出力から色の制御の文字を取り除くと、色なしの出力と同じになる(意味を色だけで表さない)。色を出すのは、標準出力と標準エラーのそれぞれについて、(1) そのストリームが端末… | この対象の確かめが通る | internal/display/color.go | build-and-test | Met |
| NFR11.8 | u6-apply/nfr-requirements/security-requirements.md | apply の確認画面と結果は、色がなくても意味が分かる。影響のある変更は `!`(影響あり)と `?`(判定できない)の記号と文字で、飛ばしと失敗は理由の文字で示す(機能設計 B… | この対象の確かめが通る | internal/display/apply_test.go | build-and-test | Met |
| NFR11.9 | u7-rollback/nfr-requirements/security-requirements.md | rollback の確認画面と結果は、色がなくても意味が分かる。途中で止まった apply の記録、取り消し済みの記録の警告は `!` と文字で、飛ばしと失敗は理由の文字で示す(機… | この対象の確かめが通る | internal/display/rollback.go | build-and-test | Met |
| NFR11.10 | u8-iac/nfr-requirements/security-requirements.md | `iac` の結果は、色がなくても意味が分かる。手で直す一覧と反映済みとみなした削除は、色ではなく見出しと文字で区別する。差分の行の `+` と `-` も文字で示す / `NO_… | この対象の確かめが通る | internal/display/iac_test.go | build-and-test | Met |
| NFR12.1 | u8-iac/nfr-requirements/tech-stack-decisions.md | U8 が足した MPL-2.0 の依存の義務を、配布の作業に引き継ぐ。U8 では、README にライセンスの節を作り(なければ)、hcl/v2 が MPL-2.0 であること、こ… | この対象の確かめが通る | README.md | build-and-test | Met |
