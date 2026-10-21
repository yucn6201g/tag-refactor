## Review

**Verdict:** READY
**Reviewer:** aidlc-architecture-reviewer-agent
**Iteration:** 1

対象: `construction/u1-walking-skeleton/infrastructure-design/` の `cicd-pipeline.md`(主)、`infrastructure-specification.md`、`monitoring-design.md`、`traceability.json`。Q1〜Q3 の回答は人の決定として扱い、指摘にしていません。

### Prior findings

| ID | Now | Severity | Note |
|---|---|---|---|
| (なし) | - | - | 1回目のレビューのため、前回の指摘はありません |

### New findings

| ID | Severity (high/medium/low) | Location | Finding | Suggested fix |
|---|---|---|---|---|
| R-01 | medium | `infrastructure-specification.md` > 確認用のリソース(P1、P2、R1、R2) | 確認用のリソースの中身が、テンプレートを書けるところまで決まっていません。(a) R1 は「誰も引き受けられない信頼の設定」としていますが、IAM のロールの信頼ポリシーは空にできず、必ず誰かを Principal に書きます。(b) R2 は「同じアカウントの人の SSO のロールだけ」としていますが、SSO のロール名は `AWSReservedSSO_<権限セット名>_<ハッシュ>` で、ハッシュは事前に分かりません。また、人の権限セット側にも `sts:AssumeRole` の許可が要ります。(c) P1・P2 の Action と Resource が決まっていません。「実害のない形」の定義がないため、実装者の判断で権限を与えてしまうおそれがあります。(d) 名前付きの IAM リソースは `CAPABILITY_NAMED_IAM`、名前なしなら `CAPABILITY_IAM` が必要で、デプロイ手順にも書かれていません。(e) R2 を使う側のプロファイルの連鎖(`role_arn` と SSO の `source_profile`、`TAG_REFACTOR_IT_PROFILE`)も未記載です | 次を仕様に書く。R1: 信頼ポリシーは、存在しない Principal を使うか、アカウントの root を信頼しつつ明示的な Deny を付けるか、固定の方式を決める。R2: 信頼先は `aws:PrincipalArn` の `ArnLike` 条件(`arn:aws:iam::<account>:role/aws-reserved/sso.amazonaws.com/*/AWSReservedSSO_<権限セット>_*`)で絞り、人の権限セットに必要な許可も書く。P1・P2: 存在しない ARN への読み取り系の Action 1つなど、権限として実害のない具体形を書く。デプロイのコマンド例(`--capabilities`)と、R2 を使うプロファイルの設定例を足す |
| R-02 | medium | `infrastructure-specification.md` > R1 の「誰も引き受けられない」/ `nfr-design/reliability-design.md` > チェックポイントでの確認 の 2 | チェックポイントの確認 2「`aws:ResourceTag/env` と `ec2:ResourceTag/env` の条件が `Env` のタグにも当てはまるか」を、現在の確認用リソースでは確かめられません。R1 を誰も引き受けられず、P1・P2 は実際の許可として働かないため、実際の評価で条件が一致するかを見る手段がありません。IAM のポリシーシミュレーターを使う手順も、そのための `iam:SimulatePrincipalPolicy` の権限(R2 の読み取り用ポリシーには含まれない)も書かれていません。機能設計の仮定が、検証されないまま残ります | 確認 2 の確かめ方を決めて仕様に書く。案 (1): 人の SSO のロールが R1 を引き受けられるようにし、キュー B(`Env=dev`)だけを対象にした読み取り系の許可を P1 の条件つきで与え、実際の呼び出しの成否で確かめる。案 (2): 人の権限でシミュレーターを使う手順書を置く。どちらでも、R2 とは別の権限であることを明記する。確かめない場合は、機能設計の仮定のまま残し、ドキュメント根拠(BR5.2 の出典)で足りると記録して、チェックポイントの項目を外す |
| R-03 | medium | `infrastructure-specification.md` > P2「`aws:ResourceTag/${aws:PrincipalTag/team}`」 | P2 は、ポリシー変数をタグのキー側(条件キーの名前)に置く前提で、BR5.4 の `?`(undecided)を確かめようとしています。IAM のポリシー変数は、条件の値と Resource では使えると文書にありますが、条件キーの名前の側で使えるかどうかは、私は確認できていません。IAM やCloudFormation の検証で拒否されると、スタック全体のデプロイがチェックポイントで失敗します(不確実) | チェックポイントの前に、`aws iam` の検証かデプロイの事前確認(cfn-lint と、変更セットまたは `aws-iac` の検証)で、このポリシー文書が受理されるかを確かめる。受理されない場合、`${...}` を含むキーは単体テストの差し替えだけで確かめることにし、P2 からは外す(変数を値側に置く形は、BR5.4 の対象と別なので、そこに置き換えないよう注意) |
| R-04 | medium | `infrastructure-specification.md` > R2 と `docs/iam/readonly-policy.json`、`cicd-pipeline.md` > `test` | R2 のポリシーは「`docs/iam/readonly-policy.json` と同じ内容」とされていますが、CloudFormation のテンプレートは外のファイルを取り込めません。内容を二重に書くことになり、U2・U5 が操作を足したとき、片方だけ更新されて NFR3.2(1)「例のポリシーだけで scan と plan が動く」の確認が、古い権限で通るおそれがあります。またテンプレート自体(`test/sandbox/u1-fixtures.yaml`)は CI で一度も検査されず、構文の誤りはチェックポイントのデプロイで初めて分かります | `test` で、テンプレートに埋め込んだ R2 のポリシーと `docs/iam/readonly-policy.json` が一致することを確かめる Go のテストを足す(受け入れ条件の延長で小さい)。あわせて `cfn-lint`(版を固定、AWS を呼ばない)を `lint` のジョブに加えるか、「デプロイの前に人が `validate-template` と `cfn-lint` を動かす」手順を書く。過剰にならないよう、cfn-guard などは加えない |
| R-05 | medium | `infrastructure-specification.md` > 確認用のリソース(IAM のリソースと Resource Groups Tagging API) | 確認用のリソースが、タグ取得の API(`tag:GetResources`)でどう見えるかが決まっていません。(a) IAM は全世界共通のサービスで、タグ取得 API が IAM のリソースを返す地域が限られるはずです(私は us-east-1 と記憶していますが、未確認)。スタックを「1つのリージョン」に置くとき、どのリージョンかが決まっていません。(b) 新しく付けたタグが API に出るまで時間がかかることがあり、デプロイの直後に scan を動かすと、期待と違う結果になります。(c) 「確かめた種類に合わせて、表のリソースを入れ替える」は、設計として曖昧です。(d) 期待する結果(scan が返すべきリソース数、タグ、変更、ぶつかり、影響)の一覧がなく、チェックポイントで合否を機械的に判断できません。なお CloudFormation は `aws:cloudformation:*` のタグを自動で付けるため、BR2.2/BR3.3 の「aws: のキーは対象外」を自然に確かめられる点は良い点です | スタックのリージョンを決めて書く(IAM のリソースが scan に出る必要があるなら、その地域にする。出る必要がなければ、IAM は scan の対象外で、ポリシーの読み取りは IAM の API だけで行うと明記する)。デプロイの後は、タグの反映を待ってから scan を動かす手順にする。表のリソースの入れ替えは、コードの生成の確認結果を記録する場所(このファイルの更新)を決める。期待結果の表(リソース、期待する変更とぶつかりと影響)を足す |
| R-06 | medium | `infrastructure-specification.md` > 確認用のリソース、Deployment > IaC | 取り違えと後始末の安全策がありません。(a) 本番に近い別のアカウントや、既存のリソースが多いアカウントに誤ってデプロイしないための確認(アカウント ID の確認など)がありません。(b) S3 のバケット名を固定すると、他人と衝突します。名前は CloudFormation の自動生成にすべきです。(c) 削除の手順(スタックの削除、S3 が空であること)、削除しないで残ったときの費用の扱い、E3(AWS Config や Resource Explorer の有効化。少額の費用がある)が確認用のスタックの外にあり、後始末の対象かどうかが書かれていません。(d) スタックの削除のタイミング(チェックポイントの後か、U2 以降も使うか)が決まっていません。共有の表では U2、U5〜U7 が自分のスタックを足すとありますが、U1 のスタックは「削除する」とも書かれており矛盾します | 削除の時期を決める(例: U1 のスタックは検証ごとに作って削除する。繰り返し使うなら残す)。バケット名は自動生成にする。デプロイ前に `aws sts get-caller-identity` で検証用アカウントであることを確かめる手順と、スタックに目印のタグを付ける。E3 の後始末(有効化したものを戻すか残すか)を、費用の見積もりとあわせて書く |
| R-07 | medium | `cicd-pipeline.md` > `ci.yml` の注記(`.gitattributes`)/ `team.md` §Code Style | `.gitattributes` が Go のファイルだけを LF に固定する書き方です。GitHub の Windows のランナーでは `core.autocrlf` の既定が有効なため、テストデータ(`testdata/` の期待される出力のファイル、ルールファイルの YAML、JSON)は CRLF で取り出されます。期待する出力との比較や、「CRLF のルールファイルも同じに読める」テスト(NFR9.1)で、Windows だけが失敗するか、逆に CRLF の入力を意図せず LF にして、CRLF のテストが空振りします | `.gitattributes` を `* text=auto eol=lf` を基本にして、CRLF として扱いたいテスト用のファイルは `-text` か専用の拡張子で明示する(または、テストの中で CRLF を組み立てる)。この方針を `cicd-pipeline.md` に書く |
| R-08 | medium | `cicd-pipeline.md` > `lint` と `vuln` の「版を固定」、`dependabot.yml` / `test` の Go の版 | (a) `golangci-lint` と `govulncheck` の版の固定が、ワークフローの `version:` 入力や `go run ...@vX` のような形になると、Dependabot の gomod も github-actions も追えず、固定した版が更新されないまま古くなります(NFR8.7 の「固定」は満たすが、更新の道が切れる)。(b) 「Go の最新版と1つ前の版」で、`go.mod` の `go` 行が1つ前の版より新しいと、Go の自動の取得(`GOTOOLCHAIN=auto`)により、実際には別の版で動いて、組み合わせの意味がなくなります。「最新版」の指定方法(`stable`/`oldstable` など)も、SHA で固定した `setup-go` とどう両立するかが書かれていません | (a) `go.mod` の `tool` ディレクティブ(Go 1.24 以降)か `tools` 用モジュールで版を管理し、Dependabot の対象にする。または、更新の確認を週1回の手作業の項目として書く。(b) `GOTOOLCHAIN=local` を指定し、`go.mod` の `go` 行を「1つ前の版」以下に保つこと、版の指定の方法を書く |
| R-09 | medium | `cicd-pipeline.md` > `ci.yml` の `test`(「`integration` のビルドタグのテストは動かない」)/ `nfr-design/reliability-design.md` | `integration` のビルドタグのテストは CI で一度も構築されません。これは実際のアカウントに対する唯一の確認(チェックポイントの検証)のコードで、型の誤りや API の変更で壊れても、チェックポイントの当日まで分かりません | `lint` か `test` に `go vet -tags integration ./...`(実行せず、型の検査だけ)を足す。AWS は呼ばないので、チームの CI の方針に反しません |
| R-10 | medium | `cicd-pipeline.md` > 関門(ブランチの保護)、流れ 4 | ブランチの保護の書き方が足りません。(a) 必須のジョブの名前(マトリクスのジョブは `test (ubuntu-latest, 1.xx)` のように組み合わせごとに名前が変わる)を、保護の設定と一致させる方法がありません。Go の版が変わるたびに、必須の名前が古くなって、マージできなくなるか、保護が外れます。集約用のジョブを1つ置いて、それだけを必須にするのが普通です。(b) 管理者への適用(「管理者も含める」)と、保護を回避できる人の扱いがありません。リポジトリの持ち主が1人なら、設定しないと関門は効きません。(c) 「人が承認したら」とありますが、1人の開発でプルリクエストの作者は自分で承認できません。必須の承認者数をどうするかが決まっていません。(d) 無料のプランの非公開リポジトリでは、ブランチの保護や必須のステータスの確認が使えないはずです(不確実。リポジトリが公開か非公開かは、このファイルから分かりません)。(e) squash マージの既定のコミットメッセージはプルリクエストのタイトルですが、Conventional Commits を確かめる仕組みがなく、Dependabot の既定のタイトル(`Bump ...`)も形式に合いません | 集約ジョブ(例 `ci-gate`、`needs` に全部のジョブを列挙)を足し、それだけを必須にする。保護の設定の一覧(管理者も含める、承認者数、ブランチを最新にする、マージの方法を squash だけにして既定のメッセージをプルリクエストのタイトルにする)を書く。リポジトリの公開の状態とプランを確認する(非公開の無料のプランなら、代わりの手段を決める)。Dependabot の `commit-message.prefix` を設定する。タイトルの検査は、必要になるまで加えなくてよい |
| R-11 | low | `cicd-pipeline.md` > `scheduled.yml`、`monitoring-design.md` > Alerts | 週1回の定期実行について、(a) 公開リポジトリでは、60日間リポジトリに動きがないと定期実行が自動で止まります(私の記憶で、未確認)。(b) 失敗の通知が行くのは「リポジトリの持ち主」ではなく、定期実行の設定を最後に変えた人です(私の記憶で、未確認)。「GitHub が持ち主に知らせる」は、そのまま信じられません | 通知の送り先は「最後に cron を編集した人」と書き換えるか、失敗時にイシューを作る手順を加える。60日の停止は、第1弾が終わるまでは問題になりにくいため、注意書きにとどめてよい |
| R-12 | low | `cicd-pipeline.md` > `test`(性能のテストを必須のジョブで動かす)、全ジョブ | 性能のテスト(NFR1.4、NFR1.5)を共有のランナー(特に macOS と Windows)で動かして必須の関門にすると、実行時間の揺れで不安定に失敗するおそれがあります。また、全ジョブに `timeout-minutes` が、ワークフローに `concurrency`(同じブランチの古い実行の取り消し)の指定がありません | 性能のテストは、実時間でなく操作の回数や割り当ての数で判定するか、余裕のあるしきい値にする(コードの生成で確認)。各ジョブにタイムアウトを入れる |
| R-13 | low | `traceability.json`、`monitoring-design.md` | 追跡の表は、設計の文書と矛盾していません(NFR8.1〜8.8、NFR9、NFR10 は参照先が実在します)。ただし、(a) NFR8.7 は「OK」ですが、上の R-08 の更新の道、(b) NFR3.2 は R-04 の一致のテスト、(c) NFR1.2 は R-05 の期待結果が決まらないと、確かめ方が弱いままです。監視の設計の「scan と plan がそれぞれ1分以内」は目安で、NFR との対応が `traceability.json` の `NFR1.2` にしかなく、数値の出所が書かれていません | R-04、R-05、R-08 を直したあとで、`traceability.json` の `target` を更新する。1分の目安は、出所(NFR1.2)を `monitoring-design.md` に書く |

### 確認した点(問題なし)

- CI の構成(6 通りの `test`、Linux の `coverage`、`lint`、`vuln`、`modcheck`、3 つの OS の `build`)は、`security-design.md` §6、`reliability-design.md`、NFR8.5、NFR8.7、NFR9.2、NFR10.1 と整合しています。小さな U1 に対して過剰ではありません。
- AWS の認証情報を CI に置かず、CI から実際の AWS を呼ばない点は、チームの進め方 §Testing Posture と一致します。
- 確認用のリソースは、費用のかからないもの(IAM、空の S3、SQS)だけで、S3 の公開遮断と暗号化も入っています。ルールの BR2.2、BR4.1、BR4.2、BR5.1、BR5.2、BR5.3 は、キュー A〜C、バケット、P1〜P3 でおおむね確かめられます(R-02、R-03、R-05 を除く)。
- SSO のプロファイルは、Go の公式 SDK(v2)の共有設定で扱えます。長く使えるキーを作らない点は NFR8.3 と一致します。

### 判定の理由

高(high)の指摘はありません。ただし、中(medium)の指摘が多いため、コードの生成に入る前(または確認用のテンプレートを書く前)に、R-01、R-02、R-03、R-05、R-10 を仕様に反映することを推奨します。

### Findings

| ID | Severity | Location | Finding | Required action | Status |
|---|---|---|---|---|---|
| - | - | - | No findings | No action required | Resolved |
