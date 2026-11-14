## Review

**Verdict:** NOT-READY
**Reviewer:** aidlc-architecture-reviewer-agent
**Date:** 2026-11-09T03:22:04Z
**Iteration:** 1

### Findings

| ID | Severity | Location | Finding | Required action | Status |
|---|---|---|---|---|---|
| R-01 | Critical | .github/workflows/release.yml > job test step "Test (no AWS calls)" (go test -mod=readonly ./...) | 公開用のワークフローの test の手順が、Node.js の導入も、テスト用アプリの `npm ci` もしないまま `go test ./...` を動かしている。GitHub Actions では `CI` が設定されているので、`testutil.RequireCDKTools` は `node_modules` が無いと `t.Fatalf` で失敗にする(BR5.1)。そのため `TestV02U1` の実 synth のテストと `internal/cdk` のテストが失敗し、`vX.Y.Z` タグの push(v0.2.0 の公開)と、`go.mod` や `go.sum` を変える Dependabot のプルリクエストで動く release の dry run が、必ず失敗する。計画の手順 13 は `ci.yml` だけを直し、`release.yml` は直していない(source-manifest にも無い) | `release.yml` の test の仕事に、`ci.yml` と同じハッシュ固定の setup-node(Node.js 22 系、npm のキャッシュ)と `npm ci --ignore-scripts --no-audit --no-fund`(テスト用アプリのフォルダー)を足す。足した変更を source-manifest.json と code-summary.md に載せ、release の dry run(workflow_dispatch か go.mod を触るプルリクエスト)で test の仕事が通ることを確かめる | New |
| R-02 | Minor | internal/pullrequest/pullrequest.go > gitEnv と talk(commit と push) | 対話の `git commit` と `git push` にも、`GIT_` で始まる変数をすべて外した環境を渡している。`GIT_SSH_COMMAND`、`GIT_SSH`、`GIT_ASKPASS`、`GIT_CONFIG_GLOBAL` で認証や署名を設定している利用者は、push や commit に失敗する。code-summary.md と README に既知の残りとして書かれているが、`--pr` の主な使い方を損なう | 外す対象を `GIT_DIR`、`GIT_WORK_TREE`、`GIT_INDEX_FILE`、`GIT_OBJECT_DIRECTORY` など、別のリポジトリを指せる変数に絞るか、認証と署名に使う変数だけを通す許可の一覧を持たせる。やらない場合は U2 で見直すことを、後の Unit の入力として明示的に記録する | New |
| R-03 | Minor | internal/pullrequest/pullrequest.go > checkTitle と internal/display/iacpr.go > PullRequestTitle | 既定のタイトルは固定の文に変更案のファイル名を足したもので、256 文字を超えると Check が `iac.prBadTitle` で止まる。ファイル名が約 210 文字を超える変更案では `--pr` が使えず、U1 にはタイトルを変える方法が無い | タイトルに入れるファイル名を一定の長さで切り詰めて 256 文字に収めるか、この制限が U2 のタイトル指定で解消されることを記録する。切り詰める場合はテストを足す | New |
| R-04 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u1-walking-skeleton/code-generation/unit-test-instructions.md > この単位のテストの動かし方 | 「この単位のテストを動かす命令」の `-run` の式(TestV02U1 と TestCDK の2つの名前の前方一致)は、この単位が足した `internal/display` の `TestPullRequest*` と `TestManualLineOfAFailedApp` など、`internal/cli` の `TestTheTokenReachesOneStepOnly` と `TestStaleBranches` などを拾わない。命令どおりに動かすと、足したテストの一部が実行されない | 名前の付け方を `TestV02U1` と `TestCDK` にそろえるか、`-run` の式にこれらのテストの名前を足して、命令がこの単位のテストをすべて動かすようにする | New |

### Validation Tool Results

| Tool | Result | Interpretation |
|---|---|---|
| go vet ./... (GOTOOLCHAIN=local, CGO_ENABLED=0) | 問題なし | 全パッケージがビルドでき、型の誤りは無い |
| go test -count=1 ./internal/redact ./internal/procexec ./internal/pullrequest ./internal/iac ./internal/display ./internal/cdk | 6 パッケージすべて成功 | 実 synth を含む単体テストが手元で通る(実 AWS と実 GitHub は呼んでいない) |
| go test -count=1 ./internal/cli | 成功(約 108 秒) | 受け入れ条件のテスト、CLI の既存テストが通る。`TestGitHubPullRequest` は secret が無く飛ぶ |
| go mod tidy -diff | 差分なし | go.mod と go.sum は整っている。modcheck の仕事は通る見込み |
| source-manifest.json と git の変更の突き合わせ | 一致(58 件) | 変更と新規のファイルがすべて載っている。README の plan の節、internal/apply、scripts、internal/iac/testdata は変わっていない |
| ci.yml の読み取り | 問題なし | setup-node と action はハッシュ固定、test と race と coverage に npm ci、ci-gate の needs に prettier と github-pr、secret は github-pr の1つの手順の env だけ(`workflow_test.go` が確かめる) |

### Summary

実装の質は高い。認証情報の分離(大文字小文字を区別しない比較、空の設定ファイル、止める変数)、補助のプログラムの書き出し(本人だけのフォルダー、SHA-256 の確かめ)、伏せ字、BR4.2 の順番、NFR5(CloudFormation のタグが無ければ synth しない)は、設計どおりでテストも実際に挙動を確かめている。ただし `release.yml` の test の仕事に Node.js と `npm ci` が無く、このままでは v0.2.0 の公開と release の dry run が必ず失敗するため、NOT-READY とする。Critical 1件を直せば、残りは Minor だけである。
