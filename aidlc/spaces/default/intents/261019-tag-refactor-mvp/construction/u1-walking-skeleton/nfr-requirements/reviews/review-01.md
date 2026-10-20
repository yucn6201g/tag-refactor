**Verdict:** READY
**Reviewer:** aidlc-architecture-reviewer-agent
**Iteration:** 1

### Prior findings

なし(iteration 1 のため、前回の指摘はありません)。

### New findings

| ID | Severity (high/medium/low) | Location | Finding | Suggested fix |
|---|---|---|---|---|
| R-01 | medium | construction/u1-walking-skeleton/nfr-requirements/security-requirements.md > 脅威と対策 / NFR8.4 | scan と plan は、AWS のアカウント内の他の人が付けられるタグのキー・値、ARN、ポリシー文書の文字列をそのまま端末に表示する。これらに制御文字や ANSI エスケープ列が含まれていると、利用者の端末の表示を書き換えられる(端末への注入)。入力の検証は NFR8.4 でルールファイル・オプション・ポリシー文書を対象にしているが、表示側の無害化の要件がなく、脅威表にもない | 「端末に出す外部由来の文字列は、制御文字をエスケープして表示する」という要件(例 NFR8.6)を追加し、制御文字を含むタグの値でエスケープされることを合否の基準にする。脅威表にも行を足す |
| R-02 | medium | construction/u1-walking-skeleton/nfr-requirements/reliability-requirements.md > NFR10.2 | 合否の基準が「受け入れ条件のテストが、実装より前のコミットに入っている」だが、チームの進め方では Bolt は `main` に squash マージされ、1つのコミットになる。マージ後の `main` ではこの順序を確かめられない。CI でも自動判定できず、検証できない基準になっている | 確かめる場所と方法を決める(例: プルリクエストのブランチ上で受け入れ条件のテストのコミットが実装より前にあることを、Code Generation の記録かレビューで確かめる)。または、基準を「各 AC に対応するテストが存在し、トレーサビリティの表で対応が取れている」に置き換える |
| R-03 | medium | construction/u1-walking-skeleton/nfr-requirements/scalability-requirements.md > NFR1.4、NFR1.5 | 合否の基準が「テストの実行時間の上限内に終わる」となっているが、その上限の値がどこにも定義されていない。測れない基準のため、通ったかどうか判定できない。また NFR1.6(observability-requirements.md)は NFR1.1 と同じ表示の重複で、別 ID にする意味がない | 上限を数値で決める(例: 1万件で 5 秒以内、ポリシー500件×変更1,000件で 5 秒以内。CI の3 OS で余裕を見る)。または時間ではなく計算量・メモリの観点(例: 件数の2倍で処理時間がおよそ2倍以内)で書く。NFR1.6 は NFR1.1 に統合し、traceability.json の対応先も直す |
| R-04 | medium | construction/u1-walking-skeleton/nfr-requirements/tech-stack-decisions.md > U1 で置くもの / security-requirements.md > NFR3.2 | 読み取り用 IAM ポリシーの例の API 一覧に、機能設計 BR1.1 が必要とするリージョンの有効性の確認(契約 C6 の `TagReader.ListRegions`。実体は ec2:DescribeRegions か account:ListRegions のいずれか)が挙がっていない。一覧が不完全なまま「この例だけで動く」と定めており、検証用アカウントで初めて失敗する。また、例のポリシーが読み取り以外の操作を含まないことを自動で確かめる基準がない(確認は手元の検証のみ) | 一覧に ListRegions に使う API を明記し、NFR3.2 の基準に「例のポリシーの Action が、読み取りの操作(Get/List/Describe 系と tag:GetResources、sts:GetCallerIdentity)だけであることを CI のテストで確かめる」を加える |
| R-05 | medium | construction/u1-walking-skeleton/nfr-requirements/tech-stack-decisions.md > YAML の読み込み | YAML のライブラリを「コードの生成のときに決める」として先送りしており、候補も採らなかった案も挙がっていない。NFR8.4(別名の展開の上限、行番号つきエラー)の達成はライブラリの機能に依存するため、実現できるか分からないまま要件を置いている。代表的な候補(gopkg.in/yaml.v3、goccy/go-yaml など)は保守状況にも差がある | 選定の基準(別名の展開の制限が設定できる、行番号を取れる、保守されている、ライセンス)と候補2つ以上の比較を、この段階で書く。決められない場合は、選定をコードの生成の最初の作業とし、基準を満たさないときの代替(読む前にサイズと別名の数を数える)を要件に残す |
| R-06 | low | construction/u1-walking-skeleton/nfr-requirements/security-requirements.md > NFR8.5 / tech-stack-decisions.md > CI | サプライチェーンの対策が `govulncheck` と Dependabot だけ。GitHub Actions のバージョンの固定(コミット SHA)、Dependabot の対象に github-actions を含めること、`govulncheck`・`golangci-lint` のバージョンの固定、`go.sum` の検証(`-mod=readonly`)、マージ後に新しく見つかる脆弱性を拾う定期実行、がない | NFR8.5 の下に、これらを数項目の要件として足す(各項目は CI の設定ファイルを見れば確かめられる)。最低でも Actions の固定と定期実行は入れる |
| R-07 | low | construction/u1-walking-skeleton/nfr-requirements/security-requirements.md > 守るもの(ポリシーの中身) | 「ポリシーの中身はツールの外に送らない」と書いているが、これを守る要件と基準がない。AWS の API 以外へ通信しないこと(テレメトリ・更新確認なし)と、SDK のリクエスト・レスポンスのログ出力(`ClientLogMode`)を有効にしないことが明記されていない。後者は `--debug` の NFR8.2 を迂回して認証情報やポリシーを出しうる | 「AWS の API 以外へ通信しない」「SDK のログ出力は有効にしない」を要件にする。基準は、差し替えたテストで SDK のログモードが無効であること、依存に通信するパッケージ(テレメトリなど)が入っていないことの確認 |
| R-08 | low | construction/u1-walking-skeleton/nfr-requirements/security-requirements.md > NFR3.1 | 合否の基準が「書き込みの口を作るコードがないことを、テストで確かめる」と曖昧で、どう確かめるかが決まっていない | 方法を1つ決める(例: scan・plan の組み立て関数が受け取る型を読み取り用のインターフェースだけにし、書き込みの API を呼ぶとテストが失敗する差し替えを渡して全コマンドを通す。または、scan・plan のパッケージが書き込み用の SDK の操作を import していないことをテストで検査する) |
| R-09 | low | construction/u1-walking-skeleton/nfr-requirements/traceability.json > NFR5 | 要件 NFR5 の合否の基準は「呼び出し回数の上限に当たる状況を再現したテストで処理が完了する」だが、U1 は既定の再試行だけ(NFR5.2、BR1.8)で、待ち時間を延ばす再試行は U2 に送っている。それでも status が `OK` で、一部だけを満たしている事実が記録から読み取れない | status を `PARTIAL`(または OK にして target に「待ち時間を延ばす再試行は U2」と明記)にして、U2 で閉じる項目を後から追えるようにする |
| R-10 | low | construction/u1-walking-skeleton/nfr-requirements/reliability-requirements.md > NFR9.2 | 「別の環境で `--help` で動かせる」は、どの環境かが決まっていない。静的リンク(`CGO_ENABLED=0`)かどうかも書かれておらず、環境によって結果が変わりうる | 基準を具体化する(例: `CGO_ENABLED=0` でビルドし、3 つの OS の CI で、ビルドしたファイルを他のファイルなしの空のディレクトリから `--help` で実行する) |

### 検証と所見

- IDの継承: 要件 NFR1〜NFR12 は traceability.json で全件(OK 7、N/A 5)に対応づけられている。N/A の理由(U2、U5、U6、U7 が受け持つ)は妥当で、機能設計・契約・チームの進め方(カバレッジ80%、3 OS、gofmt、golangci-lint、govulncheck、Dependabot、CI で実際の AWS を呼ばない)との矛盾は見つからなかった。
- 機能設計との整合: NFR3.1/BR1.5、NFR8.1/BR1.4、NFR5/BR1.8・BR5.5、`--debug`/BR1.7(質問 Q3 の確認文に沿っている)は整合している。「Impact analysis is incomplete」の文言も機能設計と一致する。
- 質問 Q1〜Q4 の人の回答は決定として扱い、指摘していない。
- 重大(high)な問題はない。上の medium は、実装の前に直しておくと手戻りが減る点で、U1 を安全でなくしたり、検証できなくしたりするものではないため、READY とした。

### Summary

U1 のセキュリティ要件は、認証情報の非出力、読み取り専用、入力の検証、依存の脆弱性確認を、測れる基準つきでほぼ網羅しており、機能設計・契約・チームの進め方とも整合している。残る課題は、端末への注入の対策がないこと、NFR10.2 と NFR1.4/1.5 の基準が検証できないこと、IAM ポリシーの例の API 一覧の漏れ、YAML ライブラリの選定が先送りされていることで、いずれも high ではないため READY とした。

### Findings

| ID | Severity | Location | Finding | Required action | Status |
|---|---|---|---|---|---|
| - | - | - | No findings | No action required | Resolved |
