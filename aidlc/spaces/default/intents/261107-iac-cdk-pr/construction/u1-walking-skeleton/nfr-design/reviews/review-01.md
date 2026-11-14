## Review

**Verdict:** NOT-READY
**Reviewer:** aidlc-architecture-reviewer-agent
**Date:** 2026-11-08T12:43:05Z
**Iteration:** 1

### Findings

| ID | Severity | Location | Finding | Required action | Status |
|---|---|---|---|---|---|
| R-01 | Major | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u1-walking-skeleton/nfr-design/reliability-design.md > 1 外のプログラムの起動と停止 | 外のプログラムはすべて procexec の1つの関数で起動し、Unix では新しいプロセスのグループ(Setpgid)に入れる。一方、契約 C5 と security-design.md の3節は、git commit と git push の標準入力を端末につなぎ、パスフレーズの入力やフックの対話を使えると決めている。新しいグループの子は端末から見ると背景のグループなので、ssh のパスフレーズや gpg の入力で端末を読もうとすると SIGTTIN で止まり、上限の10分まで固まる。端末をつなぐ呼び出しと矛盾している | commit と push など端末につなぐ呼び出しでは、Setpgid を使わない(または端末の前景のグループを渡す)方法を設計し、その場合の停止の方法(孫のプロセスの扱い)も書く。端末を使う呼び出しと使わない呼び出しで procexec の起動の形を分け、テストで端末の対話が止まらないことを確かめる方法も書く | New |
| R-02 | Major | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u1-walking-skeleton/nfr-design/logical-components.md > CI 行 | CI の設計が1行だけで、実装できる形になっていない。NFR8.1 により CI では Node.js や cdk が無いとテストが失敗するのに、今の ci.yml で go test を動かす test、race、coverage のうち、どの仕事に setup-node と npm ci を足すかが書かれていない。race と coverage に足さないと、CI の環境で必ず失敗する。結合テストの仕事の構成(Go の準備、Node.js と npm ci の有無、ci-gate の needs への追加、secret の無い実行の扱い、go test の対象を絞るかどうか)も無い。Prettier の確かめを動かす仕事も決まっていない | ci.yml の仕事ごとの表を足す。go test を動かすすべての仕事に Node.js と npm ci を入れること、結合テスト用の仕事の手順と env の置き場所、ci-gate の needs、Prettier を動かす仕事、各仕事の timeout を明記する | New |
| R-03 | Major | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u1-walking-skeleton/nfr-design/security-design.md > 6 CI のトークン | トークンを gh と git の子のプロセスの GH_TOKEN にだけ入れるとあるが、検証の対象は tag-refactor の iac --pr であり、gh と git は tag-refactor の子として利用者の環境をそのまま受け取る(C5)。つまりトークンは tag-refactor 自身の環境に入る必要があり、書かれた範囲では成り立たない。また git push は GH_TOKEN を読まないので、認証の仕組み(gh の credential helper を git の設定として渡す方法など)が無いと、CI でテスト専用のリポジトリへの push が失敗する。トークンを URL に埋め込めば禁止事項に触れる。step の env に置くと、その step で動くテスト全体が子のプロセスにトークンを受け継ぐ | 結合テストが起動するプロセスごとに、トークンを渡す先(tag-refactor 本体、gh、git)を整理して書く。git の認証方法を、トークンをファイルや URL に残さない形で決める。secret を渡す step は結合テストだけを go test の -run で絞って動かすこと、ほかのテストにトークンが受け継がれないことを確かめる方法を書く | New |
| R-04 | Major | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u1-walking-skeleton/nfr-design/traceability.json > NFR6.6、NFR6.8、NFR6.10、NFR8.2 | 状態 OK とした対応先に、該当する設計が書かれていない。NFR6.6(cdk を node_modules/.bin/cdk、次に PATH の cdk の順で探し、引数を決める)は performance-design.md の NFR3.1 の行に無い。NFR6.8(Prettier)は logical-components.md の CI 行に無い。NFR8.2 の Dependabot の更新(npm の package-lock.json の対象フォルダーを今の dependabot.yml に足す)はどの文書にも無い。NFR6.10(アクションのハッシュ固定と ci-gate への追加)も CI 行に無い | 4つの要件の設計を該当する文書に書き、traceability.json の対応先を実在する節に直す。Dependabot の設定(npm のエコシステムとテスト用のアプリのフォルダー、commit-message の接頭辞)を設計に入れる | New |
| R-05 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u1-walking-skeleton/nfr-design/reliability-design.md > 1 外のプログラムの起動と停止 | Windows のジョブオブジェクトは、子を起動したあとで入れる設計に見える。入れるまでの間に子が孫を作ると、その孫は止められない。また、テストは「上限のあと5秒の中で戻る」と WaitDelay の5秒が同じ値で、境目でゆれる | 起動から割り当てまでの隙を認めるか、一時停止で起動して割り当てる方法を書く。テストの判定には WaitDelay より余裕のある値を使う | New |
| R-06 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u1-walking-skeleton/nfr-design/security-design.md > 4 伏せ字の規則 | ソースの中身が出る行の除き方が、行番号と縦線で始まる行と、記号だけの行を落とす消去の方式になっている。Node.js が未捕捉の例外で出す、行番号なしのソース1行と記号の行は残るので、NFR1.4 のソースの中身を出さない要件を満たさない。ARN の終わりの境目(空白、引用符、カンマなど)も定義が無い | 標準エラーは、ファイル名と行と桁とメッセージの1行目だけを残す許可の方式に統一する。ARN の終わりの文字を決め、テストの例に入れる | New |
| R-07 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u1-walking-skeleton/nfr-design/security-design.md > 5 補助のプログラムの書き出し | 比べる SHA-256 を「ビルドのときに計算した定数」とするが、go:embed のビルドにはその計算の手順が無い。生成して置く定数なら、生成と更新の漏れを確かめる手順が要る。実行時に埋め込んだバイトから計算するなら、確かめるのは書き出しのあとの改ざんだけであることを書くべきである | 定数の作り方を決める(実行時に計算するのが簡単)。何を守る確かめなのかを一文で書く | New |
| R-08 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u1-walking-skeleton/nfr-design/performance-design.md > NFR3.4 の行 | 要件の NFR3.4 は、ベンチマークを CI では記録だけにすると書くが、設計(ND-Q4)は60秒で失敗にする。人が承認した変更だが、上流の要件の文と食い違ったままである。ジャンクションの作り方(Go の標準には無い)と、外のプログラムの標準エラーを取り込む量の上限も書かれていない | 要件の文との差を設計に明記する。ジャンクションの作成方法(cmd の mklink /J か x/sys)と取り込みの上限を決める | New |

### Validation Tool Results

| Tool | Result | Interpretation |
|---|---|---|
| none declared | not run | 検証ツールは宣言されていない。代わりに、既存の .github/workflows/ci.yml、.github/dependabot.yml、契約 C5 と照らして確かめた |

### Summary

セキュリティの全体の筋(認証情報の遮断、伏せ字の閉じた一覧、書き出しの守り)は具体的で実装できるが、端末につなぐ git の呼び出しとプロセスグループの停止が矛盾していること、CI の設計(どの仕事に Node.js を入れるか、結合テストのトークンの渡し方)が実装できる形でないこと、traceability.json の OK が設計の実体と合っていないことの4つの Major があり、NOT-READY とする。
