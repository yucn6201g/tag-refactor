## Review

**Verdict:** READY
**Reviewer:** aidlc-architecture-reviewer-agent
**Date:** 2026-10-31T02:00:40Z
**Iteration:** 1

### Findings

| ID | Severity | Location | Finding | Required action | Status |
|---|---|---|---|---|---|
| R-01 | Minor | internal/cli/cli.go > rootCommand の Long(「It only reads from AWS.」の行) | ルートの `--help` は今も「AWS からは読むだけ」と説明しています。しかし U6 の apply と今回の rollback はタグと記録の S3 オブジェクトを書き換えます。README は「AWS に書き込むのは `apply` と `rollback` だけ」に直してあるため、README とヘルプが食い違います。`code-summary.md` に「U7 の範囲外の古い文言」として書かれたまま、直されていません。 | ルートの `--help` の文を「`apply` と `rollback` だけが書き込む」に合わせて直し、ヘルプの文言を確かめるテストを 1 件足します。直さないなら、持ち越す先の作業単位を決めて記録に書きます。 | New |
| R-02 | Minor | internal/awsaccess/arn_test.go > TestTheOperationsOfRollbackAreInTheApplyPolicy(`traceability.json` の NFR3.13) | NFR3.13 の確かめは「rollback が呼んだ操作の集まりが例の許可の一覧に含まれること」ですが、このテストは操作の名前 7 つを手で並べて比べるだけで、rollback を動かしていません。rollback が新しい操作(たとえば S3 の別の操作)を呼ぶようになっても、このテストは通り続けます。 | 偽の AWS(HTTP レベル)で rollback を S3 の記録つきで動かし、実際に記録された呼び出しの操作名を `ApplyPermissions()` と比べるテストにします。または、手で並べた一覧であることを `traceability.json` の NFR3.13 の行に書きます。 | New |
| R-03 | Minor | internal/apply/rollback_target.go > RollbackTarget.result と internal/apply/rollback_worker.go > decideRollback の outGone | 書き込みの NotFound で飛ばした項目は、記録の reason が `resourceNotFound:NotFound` になります。最初の判定で答えに出なかった項目は `resourceNotFound` だけです。契約 C4 の `results[].reason` の値の一覧(`resourceNotFound`、`writeFailed` など)には `:<誤りの種類>` の接尾辞がなく、同じ理由が 2 つの書き方で記録されます。接尾辞は U6 から引き継いだ書き方ですが、契約の文面はそれを書いていません。 | 契約 C4 の注に「failed の reason には `:<誤りの種類>` が付く」と、skipped の `resourceNotFound` の扱い(付ける・付けない)を書きます。コードを直すなら、skipped には接尾辞を付けないようにそろえます。 | New |
| R-04 | Minor | go.mod、go.sum(コミット 3d8f8e3) | このブランチの最初のコミットは、Dependabot のマージで壊れた `go.sum` の行を消し、間接依存 3 つ(eventstream、checksum、s3shared)の版を上げるものです。変更は小さく、`go build`・`go vet`・`go test ./...` は通りますが、U7 の機能とは関係がありません。トランクの方針では 1 つの Bolt が `main` 上の 1 コミットになるため、この修正も U7 の squash コミットに入ります。 | プルリクエストの説明に、U7 とは別の前提の修正が入っていることを書きます。`main` の CI を先に直したいなら、この 1 コミットだけを別のプルリクエストで先に入れます。 | New |

### Validation Tool Results

| Tool | Result | Interpretation |
|---|---|---|
| `go build ./...`、`go vet ./...` | PASS | 問題なし。 |
| `go test ./...`(1 回) | PASS(13 パッケージすべて ok) | U6 の apply のテストは変更されておらず、共有の部品(`execution.go`)に移したあとも通る。`journal_test.go` の変更は補助関数 `idle` の 2 行だけで、期待は変わっていない。 |
| golangci-lint 2.15.0 | 0 issues | `gofmt -l` の出力もなし。 |
| 新しいテストの数 | 77 件(受け入れ 8、apply 35、journal 7、awsaccess 5、display 7、cli 12、testutil 2、rules 1) | `code-summary.md` の数と一致。 |
| `scripts/verify.sh`(読んで確かめた。実行はしていない) | 実行せず | U7 の手順は、`trap cleanup EXIT` と INT・TERM の `exit 130` の下にある。失敗時は check_keys の 4 つを R3 の apply で外し、一時キューは `u7_queue_url` が空でなければ削除する。plan の範囲の確かめ(`u6check.go plan`)が apply の前にあるので、範囲外の変更は書く前に止まる。AWS にものが残る経路は見つからなかった。 |

### Summary

機能設計の規則 BR1.1〜BR7.4、security-design の §2〜§9、契約 C10 の 3 つの呼び出しは、実装と合っています。共有に直した `execution.go` は U6 の振る舞いを変えず(U6 のテストの期待は 1 つも変わっていません)、記録の確かめ、0 個のタグ、壊れない保存、放棄の印、エラー文を出さないことは、実装とテストの両方で確かめられていました。Critical と Major はなく、見つかったのは Minor 4 件(古いヘルプ文、権限のテストの弱さ、reason の書き方の不一致、U7 と関係のない依存の修正)だけなので READY です。
