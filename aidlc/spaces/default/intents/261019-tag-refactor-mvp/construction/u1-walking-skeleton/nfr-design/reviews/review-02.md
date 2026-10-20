## Review

**Verdict:** READY
**Reviewer:** aidlc-architecture-reviewer-agent
**Iteration:** 2

対象: `construction/u1-walking-skeleton/nfr-design/` の全成果物と、改訂された上流の `nfr-requirements/security-requirements.md`(NFR3.2、NFR8.8)、`functional-design/functional-spec.md`(Internal の種類)。第1版の指摘 R-01〜R-12 が設計の文面で解消したかを確かめ、改訂で増えた不整合を探した。

**総評:** 高い重大度の指摘(R-01)は、`reliability-design.md` に「HTTP の層」と「契約のインターフェースの層」の2層の差し込み口の表、環境の隔離、時計の差し込み口、試行ごと30秒と操作全体120秒の区別が書かれて解消した。NFR3.1・5.1・5.2・8.1・8.8・11.3・1.3 は、`awsaccess` の本物の実装を通して CI で確かめられる設計になった。YAML の展開、制御文字、エラー経路、通信の範囲も、ライブラリの設定に頼らない形で決まっている。残るのは、契約の要約への反映漏れなど低い重大度のものだけで、U1 を安全でない、確かめられない、または誤ったものにするものはない。

### Prior findings

| ID | Now | Severity | Note |
|---|---|---|---|
| R-01 | Resolved | high | 差し替え位置を2層に分け、HTTP の層で SDK の設定・上限・再試行・ページ読み・エラー変換・`--debug` を本物のまま動かす。環境の隔離(一時の設定ファイル、IMDS 無効、偽の認証情報)と時計の差し込み口、30秒の意味(試行ごと)も決まった。操作名の記録を HTTP の層で行う点も `security-design.md` §1 に反映済み |
| R-02 | Resolved | medium | アンカー・別名・`<<`・重複キーを構文木で拒む設計にした(`security-design.md` §4)。ライブラリの設定に頼らない。YAML の爆発の再現テストを受け入れ条件に含めるかは明記されていないが、NFR8.4 の合否に含まれるので許容 |
| R-03 | Resolved | low | 「上限 + 1 バイト」までの読み方、通常ファイル以外の拒否、改行の正規化を判定の後に行う順序が書かれた |
| R-04 | Resolved | medium | Cc・Cf・Zl・Zp と不正な UTF-8 を対象にし、動的に作る全文字列に適用し、バックスラッシュも `\\` にした。`go/parser` による直接書き込みの禁止テストも CI に置く設計になった |
| R-05 | Resolved | medium | `Internal` を C12 に追加し、`SilenceErrors`/`SilenceUsage`、パニックの受け止め、型名だけを出す方針が決まった。ただし契約の要約側への反映は R-13 を参照 |
| R-06 | Resolved | medium | NFR8.8 の文言を、SDK の標準の認証情報の解決の宛先を含む形に直し、IMDS は有効のままで解決に30秒の上限をかけ、テストでは IMDS を無効にする。SSO の期限切れなどのエラー型の区別は「コードの生成で確かめる」としており、設計として妥当 |
| R-07 | Resolved | low | `iam:GetPolicy` を外し、「付いているものだけ」の指定で NFR3.2 を満たす旨を要件側と揃えた。ワイルドカードの Action の拒否と、操作一覧の一致テスト、ロールでの検証をチェックポイントに入れた |
| R-08 | Resolved | low | 終了条件を「続きの印が空」とし、「空のページ + 印あり」をテストに含めた。再試行の回数が取れない場合の `retries=n/a` も決まった |
| R-09 | Resolved | low | 正規表現(線形時間)に変換する方針と、病的なパターンの性能テストが入った |
| R-10 | Resolved | low | 依存の表が書き直され、`cli → impact`、`impact → planner`(型だけ)、通知口と記録先の定義元が明記された。循環はない |
| R-11 | Resolved | low | `permissions: contents: read`、`persist-credentials: false`、`go mod verify`、`go mod tidy` の差分、ブランチ保護、`-coverpkg=./...` と測る OS が設計に入った。ブランチ保護は人が設定し、チェックポイントで確かめる運用で妥当 |
| R-12 | Resolved | low | `scalability-design.md` のポリシーの行に、保持する項目(ARN、Sid か番号、演算子、条件キー、値)が明記された |

### New findings

| ID | Severity (high/medium/low) | Location | Finding | Suggested fix |
|---|---|---|---|---|
| R-13 | low | `inception/contract-design/contract-summary.md` C12(475行付近)、`reliability-design.md` 「エラーの種類と終了コード」 | `Internal` は U1 の機能設計と NFR の設計で C12 に追加されたが、共有の契約の要約 `contract-summary.md` の C12 には載っていない。U2〜U8 が C12 を消費するため、後続の作業単位の実装者が `Internal` を知らず、種類の網羅(switch の既定)を誤る恐れがある。U1 は C12 の所有者なので、追加自体は妥当 | コード生成の前までに、契約の要約の C12 に `Internal`(終了コード 1)を反映するか、U1 の完了時に契約の更新として記録する。コード生成で定義する `errkind` の一覧と契約が一致することを、チェックポイントで確かめる |
| R-14 | low | `traceability.json` NFR8.5 の行、`security-design.md` | NFR8.5 の対応先が「security-design.md 5」となっているが、供給元の節は改訂後は §6(§5 はエラーの経路)。NFR8.1 の対応先にもエラー経路の §5(`Internal`)が含まれていない | NFR8.5 を §6 に、NFR8.1 に §5 を追記する |
| R-15 | low | `reliability-design.md` 「時間の上限と再試行」 | 「再試行を含めた1つの操作の全体に120秒」の「1つの操作」が、ページ読みの全ページを含む操作全体なのか、1ページの呼び出しなのかが書かれていない。全体と読むと、件数の多いアカウントでは正常な読み取りが `Transient` になりうる(NFR1.3 の読み方では1万件でも数十ページの想定) | 「1回の API 呼び出し(1ページ)ごとに再試行を含めて120秒」と明記する。コードの生成で確かめる項目に加える |

### 良い点(参考)

- 差し込み口を2層に分け、本物の `awsaccess` を通す受け入れ条件のテストにしたことで、セキュリティ要件の大半が CI で確かめられる。
- 入力の検査をライブラリの設定に頼らず、ツール自身が行う設計にしたため、依存の挙動の変化に強い。
- 過剰設計は増えていない。

### Findings

| ID | Severity | Location | Finding | Required action | Status |
|---|---|---|---|---|---|
| - | - | - | No findings | No action required | Resolved |
