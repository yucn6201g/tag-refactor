# コードの生成の結果: U6 書き換えと記録

計画(`code-generation-plan.md`)の Step 1〜13 をすべて終えました。ブランチは `bolt/u6-apply`(`main` の `1bc15f9` から)。実際の AWS は呼んでおらず、検証用アカウントのスタックも変えていません(U6 の確認用のスタックの作成は、チェックポイントの前に変更セットを人に示して許可を得てから行います)。push とプルリクエストはまだです。

## コミット

| コミット | 内容 | 対応 |
|---|---|---|
| `e64e8a1` | test: 偽の AWS にタグの書き込み、ARN を指定した読み取り、S3(412・409・501)を足す | Step 1 |
| `60a0792`、`02cf18c` | test: U6 の受け入れ条件のテスト15件(この時点で失敗することを確かめた)と gofmt の直し | Step 2、US4.1、US4.2、AC6.1.1 |
| `8cdb70e` | feat: 書き込みの口(`TagWriter`)、記録の置き場(`ObjectStore`)、ARN を指定した読み取りと、書き換えの対象の種類の表 | Step 3 |
| `bd7dcd9` | feat: 記録と印(`internal/journal`) | Step 4 |
| `28c13e9` | feat: まとまりごとに書き換える流れ(`internal/apply`) | Step 5 |
| `8b3a26d`、`06dcbf1`、`7156ee7` | feat: 確認画面と結果の表示(英語と日本語)、`apply` コマンド | Step 6、Step 7 |
| `4af74ad` | feat: 書き換え用の権限の例と U6 の確認用のテンプレート、ルールファイル3つ | Step 8 |
| `2b728b8` | test: 受け入れ条件のテストがすべて通る。`apply` で変わる既存のテストの期待を直す | Step 9 |
| `7a8efd1` | fix: 呼び出しを取り消した実行の記録を完了にしない | Step 10 で見つけた不具合 |
| `b58629e` | test: apply、journal、書き込みの口の単体テスト | Step 10 |
| `fbe6838` | test: 測る条件の2つの分布の大きなテスト | Step 11 |
| `a03e5ab` | test: 検証のスクリプトに U6 の確かめを足す | Step 12 |
| `ccc323a` | docs: README に apply、記録、印、権限の節 | Step 13 |
| `dfd90ef` | fix: 2回目の Ctrl+C で取り消した書き込みを resultUnknown のままにする | Step 13 の確認で見つけた不具合 |
| `b31dc92` | fix: S3 の 404 と固定の待ちに頼らない | レビュー1回目の指摘 |
| `f1fccf4` | test: U6 のチェックポイントで U1 の P2 の「決められない」だけを許す | チェックポイントの検証 |
| `6f9be03` | test: IAM のロールを U6 のチェックポイントの対象から外す | チェックポイントの検証 |
| `20c8b5a` | test: 一度もタグのないキューは U6 のチェックポイントで表示だけにする | チェックポイントの検証 |

## 作ったファイルと変えたファイル(50ファイル、+9,146 / −58。`aidlc/` を除く)

| 区分 | ファイル |
|---|---|
| 新規(実装) | `internal/apply/`(`apply.go`、`batch.go`、`stop.go`、`target.go`、`worker.go`)、`internal/journal/`(`journal.go`、`location.go`、`lock.go`、`run.go`、`store.go`)、`internal/awsaccess/`(`write.go`、`objectstore.go`、`tagtypes.go`)、`internal/cli/apply.go`、`internal/display/apply.go`、`internal/testutil/`(`tagaccount.go`、`s3bucket.go`) |
| 変更(実装) | `internal/awsaccess/awsaccess.go`・`invoke.go`、`internal/cli/cli.go`・`commands.go`・`messages.go`、`internal/display/display.go`・`messages.go`、`internal/testutil/fakeaws.go` |
| 新規(テスト) | `internal/apply/`(`apply_test.go`、`large_test.go`、`race_test.go`、`norace_test.go`)、`internal/journal/journal_test.go`、`internal/awsaccess/`(`write_test.go`、`docs_apply_test.go`)、`internal/cli/`(`acceptance_u6_test.go`、`apply_cli_test.go`、`apply_internal_test.go`)、`internal/display/apply_test.go`、`internal/testutil/tagaccount_test.go` |
| 変更(テスト) | 下の「期待を変えた既存のテスト」 |
| 例のポリシー・検証 | `docs/iam/apply-policy.json`(新規)、`test/sandbox/u6-fixtures.yaml`・`u6-add-rules.yaml`・`u6-remove-rules.yaml`(新規。`u6-untagged-rules.yaml` は作ったが、チェックポイントの検証のあとに消した)、`scripts/verify.sh`、`scripts/u6check.go`(新規。`go run` で動かす確かめの補助で、ビルドには含めない) |
| ドキュメント | `README.md` |

削除したファイルはありません。新しいパッケージは計画どおり `internal/apply` と `internal/journal` の2つだけです。

## 主な実装の決定

- **書き込みの口:** 書き込みは読み取りの `Session` とは別の `Writer` 型にし、`apply` だけが作る(NFR3.10。読み取りのコマンドには書き込みの口を渡さない)。`TagResources`・`UntagResources` は `(WriteResult, error)` を返し、リソースごとの失敗は `WriteResult.Failed` に誤りの種類で入れる。一時的な失敗のリソースだけを、SDK の再試行の待ち時間のあとに書き直す(全部で最大 `MaxAttempts` 回。NFR5.12)。`aws:` で始まるキーは AWS を呼ぶ前に拒む。
- **失敗の分け方:** `FailedResourcesMap` の `StatusCode` を主にする(429 と 5xx は Transient、403 は AccessDenied、404 は NotFound、400 は InvalidInput)。状態コードがないときだけ `ErrorCode` で決める。AWS のエラー文は記録にもログにも書かない。
- **記録と印:** `Begin` が返す `Run` に `RecordBefore`・`RecordAfter`・`Complete`・`Close`・`WriteLog` を持たせた。保存は一時ファイル `.<runId>.tmp-<乱数>` に書いてから名前を変える(Windows は最大5回試す)。5秒ごとの保存は変わった分があるときだけで、重ならない。3回続けて保存に失敗したら新しい書き込みを止める。印は手元を `O_EXCL` で、S3 を条件付きの書き込み(`PutIfAbsent`)で取り、印を消す前に runId を確かめて他の実行の印は消さない。
- **書き換えの流れ:** リージョンごとに、変更の形(付けるタグと消すキー)が同じリソースを20件ずつのまとまりにし、読み直しは100件までの単位で行う(BR4.7)。書き込みのあとに読み戻して結果を決める(BR4.3)。同時に動くリージョンは4まで、リージョンの中では呼び出しを重ねない。
- **止め方(2段):** 合図の役は `internal/cli` に置き、`apply` には「止める合図」の channel と取り消せる `context` を渡す。1回目の Ctrl+C は動いているまとまりを終えてから止め、2回目は実行の `context` を取り消す。最後の保存、印を消すこと、止めたあとの読み戻しは、取り消されない別の `context`(30秒の上限)で行う。
- **取り消した書き込みの扱い(`dfd90ef`):** 2回目の Ctrl+C で取り消した書き込みは、読み戻しで変更前のタグが見えても `pending`(`resultUnknown`)にする。取り消した要求が AWS 側で読み戻しのあとに反映されることがあり、「何も書いていない」とは言い切れないため(BR4.5、セキュリティ設計 §5)。直す前は、読み戻しが記録を閉じるより先に終わると `failed`(`writeFailed`)になり、`internal/cli` の `TestTwoCtrlCCancelTheRunningCall` が30回中6回失敗していた。直したあとは40回続けて通る。`internal/apply` に `TestACancelledWriteStaysUnknown` を足した。

## テストの結果(手元、Windows)

| 確かめたこと | 結果 |
|---|---|
| `gofmt -l` | 出力なし |
| `go vet ./...`、`go vet -tags integration ./...` | 問題なし |
| `golangci-lint run`(`integration` のタグあり・なし) | 0 issues |
| `go test ./...`(Go 1.27.0) | 13パッケージすべて通る |
| `go test ./...`(Go 1.26.0。CI の oldstable と同じ系列) | すべて通る |
| カバレッジ(`-coverpkg` で `cmd` と `internal` のすべて) | 96.3%(下限80%) |
| 新しいテスト | 最上位で93件(受け入れ条件15、cli 12、apply 23、journal 19、awsaccess 11、display 7、testutil 5、rules 1) |
| `govulncheck` | 外部のモジュールに該当なし。手元の Go 1.27.0 の標準ライブラリの既知の脆弱性だけ(GO-2026-6809 ほか。1.27.3 で修正) |

大きな偽の AWS のテスト(1万件、17リージョン、偏ったリージョンに2,000件。NFR5.13、NFR5.14、NFR5.16):

| 分布 | GetResources(上限) | TagResources(上限) | UntagResources(上限) | 時間 |
|---|---|---|---|---|
| よくある変更 | 334(374) | 585(585) | 585(585) | 0.21 秒 |
| 値がばらばらなキー名の変更 | 300(374) | 10,000(10,000) | 10,000(10,000) | 0.45 秒 |

- 上限はどちらも NFR5.14 の式で、置いた件数から計算した。1回の書き込みは20件以下、読み取りは100件以下、同時に動くリージョンは4以下、リージョンの中で呼び出しが重ならないことも確かめた。偏ったリージョンの回数は、NFR の測る条件の見積もり(最初の読み取り20回、読み直しと読み戻し50回、付ける・消す約105回ずつ)と一致した。
- 受け入れ条件のテスト15件は Step 2 で先に書いてコミットし、失敗することを確かめてから実装した(Testing Contract の custom の順)。
- `-race` は、手元に cgo が無いため動かしていない。race 用の大きさ(10分の1)を、定数を一時的に変えて流し、上限以内(56〜77ミリ秒)だった。CI の `race` ジョブで確かめる。
- `cfn-lint` は手元に無いため動かしていない。CI の `lint` ジョブで確かめる。

## 作りながら確かめたこと

- **`Resource` の形(Service Authorization Reference):**
  - `tag:TagResources`・`tag:UntagResources`・`tag:GetResources` はリソースの種類を持たない操作なので `"*"`。理由を README に書いた。
  - SQS、SNS、Secrets Manager、KMS、Lambda、DynamoDB、CloudWatch Logs のタグの操作の ARN の形は、NFR の設計 §2 の表のとおりだった。
  - EC2、S3、IAM、RDS は、該当する表を取り出せず確かめられなかった。NFR の設計の形のまま使い、検証用アカウント(R3 で S3、IAM のロール、EC2 のセキュリティグループを書き換える)で確かめる。
- **SDK の `FailureInfo`:** `resourcegroupstaggingapi` v1.41.5 の `types.FailureInfo` に `StatusCode int32`、`ErrorCode`(`InternalServiceException`、`InvalidParameterException`)、`ErrorMessage` があることを確かめた。

## 期待を変えた既存のテスト

- **`internal/cli`(U1):** 「まだ使えないコマンド」のテストの対象を `apply` から `rollback` と `iac` に変えた。
- **`internal/cli` の誤りの文言:** Conflict の文言を、別の apply か rollback がそのアカウントで動いている(印がある)という意味の文言と `--force-unlock` の案内に変え、AccountMismatch と `--record-to` の誤りの例を足した。
- **`internal/awsaccess/docs_test.go`:** 例のポリシーの `<account-id>` の定数を、U6 のテストと共有する場所に移した。
- **`internal/display/outputguard_test.go`:** 出力を守るテストの対象から `scripts/`(`u6check.go`)を外した。`scripts/` は利用者向けのコマンドの出力ではない。
- **`internal/rules/rules_test.go`:** U6 のルールファイル3つが読み込めることを確かめるテストを足した(既存の期待は変えていない)。

## 計画と設計からのずれ

- **ルールファイルの種類の書き方:** `cicd-pipeline.md` にある `AWS::SQS::Queue` の書き方はルールファイルの読み込みで誤りになるため、scan が示す形(`sqs`、`iam:role`、`ec2:security-group` など)で書いた。
- **権限の例の S3 の操作:** NFR の設計 §2 に合わせて `s3:ListBucket` を含めた4つにした(NFR3.11 は3つと書いている。ないキーを読んだときに AccessDenied ではなく NotFound を返させるため)。
- **契約 C10 の形:** `RecordBefore`・`RecordAfter`・`Complete` は、runId を引数で渡す形ではなく `Begin` が返す `Run` のメソッドにした。止めた実行のための `Close` と、ログを書く `WriteLog` を足した。
- **契約 C6 の形:** `TagWriter` は `(WriteResult, error)` を返す。書き込みは `Session` とは別の `Writer` 型にした。
- **検証のスクリプトの JSON の確かめ:** 手元に jq がないため、`scripts/u6check.go`(`go run` で動かす)で記録の JSON を確かめる。
- **AC4.2.6 の確かめ方:** 「保存の途中で止める」は journal の単体テストでファイルの操作を差し替えて確かめる。受け入れ条件のテストでは、強制終了で残った一時ファイルと、書き換えの途中の Ctrl+C で記録が壊れないことを確かめる。このテストでは、止める合図が次のまとまりより先に届くよう、Ctrl+C のあと偽の AWS の答えを200ミリ秒遅らせた。
- **機能設計の Minor の直し(Step 13):** `functional-spec.md` の状態の図に、まとまりごとの読み直し(BR4.7)による pending から skipped への遷移を足し、期待するタグの例の表の4行目の空行を消した。
- **作業の進め方:** 開発の役が、`u6check.go` の動作確認で一時フォルダーに `printf` で試しのファイルを2つ書いた(確認後に消した)。push 前の自分の直前のコミットを gofmt の直し忘れのため2回 amend した(`28c13e9`、`fbe6838`)。

## レビュー(1回目、READY)の指摘への対応(`b31dc92`)

- **Major: S3 の記録が 404 に頼っていた。** S3 は、ないキーを読んだとき、`s3:ListBucket` を持つ場合だけ 404 を返し、持たない場合は 403 を返す(SDK の文書)。例のポリシーの `s3:ListBucket` は `s3:prefix` の条件つきで、この条件がないキーの読み取りの判定でも成り立つかは文書に書かれていない。そのため、毎回の実行で通る `Begin` の事前の読み取りをやめた。最初の保存は条件付きの書き込みなので、同じ runId の記録があればそこで断る(`TestBeginRefusesARunIDThatHasARecord` を、最初の保存で断る形に直した)。権限の例の条件(`StringLike` を `StringLikeIfExists` にする案)は変えていない。変えると記録のバケット全体の一覧を許すことになるため、設計で決めた手順どおり、チェックポイントの手順 6(R3 でないキーを読んで NotFound になるか)で確かめてから決める。403 になった場合に残る影響は、`--force-unlock` で S3 の印がないときと、印を消すときに印が既にないときの2つ。
- **Minor: 条件付きの書き込みの再試行。** S3 に届いた条件付きの書き込みが再試行されて 412 になったとき、印は runId が自分のものなら取れたものとし、記録の最初の保存は中身が同じなら保存できたものとした(`TestARetriedConditionalWriteFindsItsOwnObject`)。
- **Minor: 固定の待ちのテスト。** 止める役が、`apply` に渡された止める合図の channel も直接見るようにし、確かめの前に出た合図は必ず見えるようにした。`TestTheFirstStopFinishesTheRunningBatch` の50ミリ秒の待ちを消した。`TestTheSecondStopCancelsTheRunningCall` の300ミリ秒は、取り消してもすぐには終わらない呼び出しを表すためのもので、合図の待ちではないので残した。`internal/cli` のテスト(AC4.2.6 の200ミリ秒、2回目の Ctrl+C)は、合図が `internal/cli` の goroutine を通るため偽の AWS の答えの遅れを残した。手元で30〜40回続けて通ることを確かめた。CI の3つの OS と `race` で確かめる。

## チェックポイントの検証(2026-10-30)

- 人の許可を得て、ValidatePolicy(R3 のインラインポリシーと BU6RPolicy。指摘0件)のあと、変更セット `u6-create`(5つの追加、置き換えなし)を作って示し、実行した。スタック `tag-refactor-u6-fixtures` は `CREATE_COMPLETE`。人がプロファイル `tag-refactor-apply` を足した。
- 4回目の `verify.sh` で `verify: OK`(終了コード0、検証済みとして記録)。それまでの3回は、次の3つで止まった。どれも書き換えの前に止まり、何も書き換えず、印も残らなかった。
  1. **U1 の P2 の影響の指摘:** U1 の確認用のポリシー P2 の文 `PolicyVariableKey`(条件のキー `aws:ResourceTag/${aws:PrincipalTag/team}`)が、どのタグのキーにも「決められない」を付けるため、手順 2 の「影響なし」が成り立たなかった。人の判断で、P2 のこの指摘だけを許し(`u6check`)、apply に `--allow-impact` を付けた(`f1fccf4`)。P2 以外の指摘があれば手順 2 で止まることを、作った変更案で確かめた。
  2. **IAM のロール RU5 が集まらない:** `tag:GetResources` を `iam:role` で絞ると us-east-1 でも0件で、scan は IAM のロールを集められず、apply の対象にならない(AWS の文書の一部にも、GetResources は IAM のユーザーとロールに対応しないとある)。人の判断で RU5 を対象から外し、5つの種類で確かめ、README に制限として書いた(`6f9be03`)。R3 の IAM のタグの操作は偽の AWS のテストだけで確かめる。
  3. **一度もタグのないキュー:** `plan` はタグ付けの API で集めたリソースだけから変更案を作るため、一度もタグのないリソースは変更案に入らず、手順 5 の apply は成り立たなかった。人の判断で、手順 5 は「ARN を指定した `tag:GetResources` の答えに出るか」の表示だけにし、`u6-untagged-rules.yaml` を消し、README に制限として書いた(`20c8b5a`)。
- 手順 3: 足して消すことを5回くり返し、10回の apply の記録がすべて完了、失敗と未処理なし、5つの対象(BU5、QU5、TU5、SU5、GU6)がすべて `applied`。範囲には U1・U5・U6 の確認用のリソース(U1 のバケットとキュー3つ、BU6R など)も入り、10件(4回目の remove と5回目の add は9件)だった。4回目の remove の変更案に、対象外の確認用のリソースの1つが入らず(収集の結果整合と見られる)、そのリソースはキーが付いたまま5回目の add で変更なしになり、5回目の remove(10件)で消えた。5つの対象は毎回すべて入っていた。
- 手順 4: どのリソースと IAM のロールにもキーが残らず、手元と S3 の印も残らなかった。
- 手順 5: ARN を指定しても、タグの API は一度もタグのないキューを返さなかった。
- 手順 6: 3回目の記録 `20261030T051817Z-df7d` は S3 にあり、暗号化(AES256)され、所有者の許可だけだった。R3 で、ないキーの読み取りは NotFound になった。条件つきの `s3:ListBucket` で 404 が返ることが分かったので、レビューの Major の、権限の例の条件を広げる案は採らない。
- 手順 7(NFR5.15):
  - L95: `tag:GetResources` 90 ms(標本30)、`tag:TagResources` 217 ms(標本5、最大)、`tag:UntagResources` 151 ms(標本5、最大)。再試行0回。
  - 見積もり: よくある変更 0.9 分(上限30分)。値がばらばらな場合 14.8 分(記録だけで、判定しない)。
  - **見積もりの限界:** 1回の書き込みは数件で、標本も5〜30と少なく、L95 は実質の最大である。20件のまとまりや100件の ARN の読み取りは、これより長いことがある。それでも、よくある変更の見積もりは上限の3%で、余裕が大きい。
- U2 の見積もり(NFR1.8)は、この日の実行で 408〜409 秒(上限600秒)だった。

## まだ確かめていないこと

- **CI:** `-race`、`cfn-lint`、3つの OS は、プルリクエストの CI で確かめる。
- **確かめていない種類:** IAM のロール・ユーザー・管理ポリシー(scan で集まらない)、EC2 のインスタンス・スナップショット・AMI、KMS、Lambda、DynamoDB、CloudWatch Logs、RDS は、検証用アカウントで書き換えていない。
- **後の作業の候補:** IAM のリソースを集めること、一度もタグのないリソースに必須タグを付けられるようにすること。どちらも U2・U4 の範囲に手が入る。
