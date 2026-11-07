## Review

**Verdict:** READY
**Reviewer:** aidlc-architecture-reviewer-agent
**Date:** 2026-10-29T09:57:07Z
**Iteration:** 1

### Findings

| ID | Severity | Location | Finding | Required action | Status |
|---|---|---|---|---|---|
| R-01 | Major | docs/iam/apply-policy.json > Statement `TagRefactorRecordList`、internal/journal/run.go > `Begin`(S3 の分岐の `Store.Get`)、internal/journal/lock.go > `releaseS3` と `ForceUnlock` | S3 に記録を置く経路は、「ない記録の `GetObject` が NotFound(404)で返る」ことに頼っている。`Begin` は毎回、まだ存在しない記録を `Get` して NotFound を期待する。ところが例のポリシーの `s3:ListBucket` は `s3:prefix` の条件(`<record-prefix>/*`)つきである。`s3:prefix` は一覧の要求にしか付かないキーなので、`GetObject` が見つからないときの権限の判定では条件が成り立たず、404 ではなく 403(AccessDenied)になる可能性が高い。そうなると `Begin` は NotFound 以外の誤りとして `apply` を止め、例のポリシーのまま `--record-to s3://...` が使えない。この前提は検証用アカウントで未確認である(`verify.sh` の U6 手順 6 で確かめる予定だが、まだ動かしていない)。また `Begin` の事前の `Get` は、直後の条件付き書き込み(`PutIfAbsent`。同名があれば 412 で Conflict)と同じことを確かめていて、404 への依存だけが余計に増えている | `Begin` の S3 側の事前の `Get` をやめ、`PutIfAbsent` の Conflict だけで「同じ runId の記録がある」を判定する(ローカル側の `Lstat` と同じ強さになる)。あわせて `TagRefactorRecordList` の条件を `StringLikeIfExists` にするなどして、`s3:prefix` がない要求でも ListBucket が成り立つようにし、`security-design.md` §2、`docs_apply_test.go`、`u6-fixtures.yaml` の R3 の3か所を同じ形にそろえる。検証用アカウントでの手順 6(ないキーが NotFound になる)が通ることを、チェックポイントの前に確かめる | New |
| R-02 | Minor | internal/journal/lock.go > `Lock`(S3 の `PutIfAbsent` が Conflict のときの分岐) | S3 の印を `If-None-Match: *` で書く要求が、通信の途中の失敗で SDK に再試行されると、1回目は S3 に届いて成功し、2回目が 412 になる場合がある。このとき `Lock` は、読んだ印の runId が自分のものでも他の実行の印として Conflict を返し、手元の印だけを消す。S3 には自分の印が残り、利用者は `--force-unlock` を使うしかなくなる(安全側の失敗だが、案内が「他の実行が動いている」になり紛らわしい) | Conflict のとき、読んだ印の runId が自分の runId なら、取れたものとして進める(それ以外は今のとおり Conflict)。この分岐の単体テストを足す | New |
| R-03 | Minor | internal/apply/apply_test.go > `TestTheFirstStopFinishesTheRunningBatch`(50ms の待ち)、`TestTheSecondStopCancelsTheRunningCall`(300ms)、internal/cli/acceptance_u6_test.go(AC4.2.6 の 200ms の遅延) | 止める合図が `apply` の中に届くまでの時間を固定の待ちで合わせているため、負荷の高い CI(特に Windows と `-race` のジョブ)では合図が間に合わず、次のまとまりが書かれて件数の確認が外れるおそれがある。実際に `TestTwoCtrlCCancelTheRunningCall` が30回中6回失敗した前例がある(`dfd90ef`) | 固定の待ちをやめ、書き込みの呼び出しの中で「止める合図が内部で受理された」ことを(テスト用の確認口や `stopped()` の確認で)待ってから進める形にする。直らない場合は、待ちを十分長くし、CI で3つの OS とも繰り返して通ることを確かめる | New |

### Validation Tool Results

| Tool | Result | Interpretation |
|---|---|---|
| `go vet ./...` | 問題なし | 通常のビルド対象に問題はない |
| `go test ./...`(Go 1.27.0、Windows) | 13パッケージすべて成功 | `internal/apply`、`internal/journal`、`internal/cli` を含めて通る。`-race` は手元に cgo がなく未実行で、CI の `race` ジョブで確かめる必要がある |
| `traceability.json` の `target` の実在確認 | 欠けているファイルなし(76件が OK) | 追跡の対象のファイルはすべて実在する。粒度はファイル単位 |

### Summary

契約 C4 の記録の状態と `touchedKeys`(付けるのが先、50件を超えるときだけ消すのが先、途中で飛ばした項目の扱い、2段の Ctrl+C での `resultUnknown`)、あらゆる終わり方での印の解放、競合のないロックの順序、失敗の分類、記録の原子的な保存は、設計どおりに実装され、テストもエラー系を実際に確かめていて、READY と判断する。残る主な懸念は、S3 に記録を置く経路が、条件つきの `s3:ListBucket` のもとで 404 を返すという未検証の前提に頼っている点で、チェックポイントの前に直すか、検証用アカウントで確かめること。計画からのずれ(`s3:ListBucket` を含めた4操作、`Run` のメソッド化、`Writer` の分離など)はいずれも設計の意図に沿っていて受け入れられる。
