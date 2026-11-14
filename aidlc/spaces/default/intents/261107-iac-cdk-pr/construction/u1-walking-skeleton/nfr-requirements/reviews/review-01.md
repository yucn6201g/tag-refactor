## Review

**Verdict:** READY
**Reviewer:** aidlc-architecture-reviewer-agent
**Date:** 2026-11-08T08:47:08Z
**Iteration:** 1

### Findings

| ID | Severity | Location | Finding | Required action | Status |
|---|---|---|---|---|---|
| R-01 | Major | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u1-walking-skeleton/nfr-requirements/security-requirements.md > NFR1.2 と NFR1.4 | NFR1.2 は、表示、--debug、プルリクエストに ARN とアカウント ID を出さないと言い切っているが、伏せるのは tag-refactor が組み立てる文だけである。NFR1.4 が --debug に出す synth と補助のプログラムの標準エラーで伏せるのは認証情報の形だけで、ARN や 12 桁の数字は伏せない。git と gh の標準エラーは端末に直結される(BR4.5)ので、NFR1.2 と NFR1.3 の対象にもならない。どこまでが保証か決まっておらず、受け入れテストも組み立てる文の分しかない。加えて、公開の場に出るプルリクエストの本文の決まり(変更したファイルの一覧だけ。タグの値と外のプログラムの出力は載せない。BR4.4)が、セキュリティの要件と合否の基準に入っていない | NFR1.2 に、保証の範囲を出力の経路ごとに表で書く(プルリクエストのタイトル、本文、コミットのメッセージは載せてよいものを一覧に限る。端末の表示と --debug は、伏せるものを ARN と 12 桁の数字まで広げるか、広げないなら外のプログラムの標準エラーは対象外だと明記する)。BR4.4 の「タグの値と外のプログラムの出力を本文に載せない」を要件にし、仕込んだ値が本文に出ないことを合否の基準にする。--pr-title と --pr-body-file で利用者が渡した文は対象外だと書く | New |
| R-02 | Major | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u1-walking-skeleton/nfr-requirements/security-requirements.md > NFR2.4 | 実際の GitHub の結合テストのトークンについて、置き場所(CI の secret、子のプロセスの GH_TOKEN)は決めているが、次が要件に無い。(1) トークンの権限の範囲(テスト専用のリポジトリだけ、必要最小の権限、有効期限)。(2) トークンを受け取る仕事と手順の範囲(今の ci.yml は contents: read だけで AWS の認証情報も持たない前提なので、新しい仕事だけが secret を受け取ること)。(3) テストが clone したテスト専用のリポジトリで動かす npm ci と synth の子のプロセスに、トークンを渡さないこと(npm のインストール時のスクリプトは利用者のコードと同じに動く。synth は BR1.3 で外れるが、npm ci には決まりが無い)。合否の基準も「コードの確かめ」だけで、機械で確かめられない | NFR2.4 に、トークンは fine-grained でテスト専用のリポジトリの必要最小の権限だけとすること、secret を渡すのは結合テストの手順だけで、ワークフローの permissions は最小に保つこと、npm ci と synth と補助のプログラムの子のプロセスの環境に GH_TOKEN を入れないことを足す。合否の基準に、環境を差し替えたテストで npm ci の子の環境にトークンが無いことと、ワークフローの定義の確かめ(secret を参照する手順が1つだけ)を入れる | New |
| R-03 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u1-walking-skeleton/nfr-requirements/security-requirements.md > 脅威の見方(STRIDE)と残るリスク | 脅威の表が情報の漏えいと改ざんだけで、tag-refactor が利用者のリポジトリの中の cdk.json を見つけただけで、そのアプリのコード(cdk.json の app の命令、node_modules/.bin/cdk、ts-node)を動かす点が脅威として挙がっていない。残るリスクは「利用者が自分で cdk synth を動かすときと同じ信頼」とするが、実際は iac を動かしただけで、取得したばかりの信頼できないリポジトリのコードが動く点が前と違う。公開の場への書き込み先の取り違え(upstream への誤ったプルリクエスト)も挙がっていない | 脅威の表に、信頼できないリポジトリでの実行(権限の昇格)と、プルリクエストの行き先の取り違えを足し、手当て(synth の実行をアプリごとに標準エラーに示す NFR10.1、行き先のリポジトリとブランチを作成後に示す FR1.9、README の注意)を対応づける | New |
| R-04 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u1-walking-skeleton/nfr-requirements/security-requirements.md > NFR1.4 | 伏せる対象が「ソースの中身」と「ghp_ などのトークンの形」で、形の一覧が閉じていない(gho_、ghs_、github_pat_、AKIA、ASIA など)。ts-node や tsc の標準エラーのコードの抜粋(code frame)を伏せる方法も決まっておらず、テストはトークンの形と環境の変数の値しか見ない | 伏せる形を一覧にし(GitHub のトークンの接頭辞、AWS のアクセスキー ID の形)、コードの抜粋は行ごと除くか、--debug の標準エラーは中身を伏せない旨を決めて、そのテストを足す | New |
| R-05 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u1-walking-skeleton/nfr-requirements/security-requirements.md > NFR2.3 | 補助のプログラムの SHA-256 の確かめは、確かめてから node が読むまでの間に書き換えられる余地が残る。Windows には 0700 と 0600 に当たる決まりが無く、一時的なフォルダーの保護は %TEMP% の既定の権限に頼る。この条件の違いが書かれていない | Windows での保護は既定の権限に頼ると明記する。または、補助のプログラムを標準入力や node の引数で渡して、一時ファイルを無くすことを比べて決める | New |
| R-06 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u1-walking-skeleton/nfr-requirements/performance-requirements.md > NFR3.4 と NFR3.7 | 「30 秒以内」「10 分以内」と書きながら、CI では記録だけで合否にしないので、要件として確かめられない(目安として質問の回答 Q1 と Q5 の A に沿ってはいる) | 目安であって合否の基準ではないと見出しの表で分けるか、ベンチマークの時間が 2 倍を超えたら警告を出すなど、確かめる方法を1つ決める | New |
| R-07 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u1-walking-skeleton/nfr-requirements/tech-stack-decisions.md > NFR6.9 と reliability-requirements.md > NFR3.10 | Windows でプロセスの木を止める方法が「ジョブオブジェクトか taskkill /T」の二択のままで、NFR3.10 の「上限のあと 5 秒以内に戻る」を満たす方を決めていない | NFR Design で方法を1つに決め、Windows の CI のテストで子の子のプロセスが残らないことを確かめる基準に直す | New |
| R-08 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u1-walking-skeleton/nfr-requirements/security-requirements.md > NFR1.1 と NFR2.4 | 合否の基準が「コードの確かめ」「ワークフローとテストのコードの確かめ」で、機械で繰り返せない | リポジトリ全体を GH_TOKEN などの読み取りと秘密の形の文字列で検索するテストか CI の手順を足す | New |

### Validation Tool Results

| Tool | Result | Interpretation |
|---|---|---|
| (none declared) | 該当なし | 手で確かめた。traceability.json は inception の NFR1 から NFR10 のすべてに、導出した NFRx.y の ID を対応づけ、各 ID は 5 つの要件の文書と技術の選択のどれかに存在する(NFR1.5 は observability、NFR3.8 と NFR3.9 は scalability、NFR3.10 と NFR4.x は reliability、NFR6.2 から NFR6.10 は tech-stack-decisions)。BR1.3、BR4.3 から BR4.5、BR5.2 の内容は NFR2.1 から NFR2.4 と NFR1.2 から NFR1.4 に反映されている |

### Summary

U1 の非機能の要件は、inception の NFR1 から NFR10 と契約 C5、機能の設計の決まりと整合し、対応づけも抜けていない。Critical は無い。ただし、出力ごとの伏せ字の保証の範囲とプルリクエストの本文の決まりが曖昧な点(NFR1.2 と NFR1.4)と、CI のトークンの権限と受け渡しの範囲が要件にない点(NFR2.4)の2つの Major がある。Major は 2 件以下のため READY とするが、次のステージに進む前に直すことを勧める。
