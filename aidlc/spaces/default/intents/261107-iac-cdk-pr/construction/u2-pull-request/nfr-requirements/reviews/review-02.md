## Review

**Verdict:** READY
**Reviewer:** aidlc-architecture-reviewer-agent
**Date:** 2026-11-09T09:34:56Z
**Iteration:** 2

### Prior findings

| ID | Severity | Location | Finding | Required action | Status |
|---|---|---|---|---|---|
| R-01 | Major | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/nfr-requirements/traceability.json > upstream_ids と reverse | upstream_ids が NFR1 と NFR6 だけで、NFR5、NFR7、NFR8、NFR9 が一覧にも N/A にも無く、N/A が独自の reverse 配列にあった | NFR1 から NFR10 を upstream_ids と coverage に並べ、reverse をやめ、NFR9 で U2 が足すものを明記する。確認: 10件すべてが coverage にあり、reverse は無く、NFR9 は U1 の NFR9.1 と U2 の NFR9.2 を示す。NFR9.2 は security-requirements.md に実在する | Resolved (reviewer) |
| R-02 | Major | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/nfr-requirements/tech-stack-decisions.md > NFR6.15(および security-requirements.md > NFR1.9) | PowerShell の引用が ' の二重化だけで、U+2018 から U+201B と " を含む名前で命令が壊れる | 4つの引用符も二重にし、" や改行は手順の文にし、合否の基準に足す。確認: 「続きの命令の引用」の表でこの3点が決まり、NFR6.15 とNFR1.9 の合否の基準に値ごとのテストがある。残る細部は R-10 に分けた | Resolved (reviewer) |
| R-03 | Major | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/nfr-requirements/security-requirements.md > NFR1.6 と NFR1.7 | 契約 X1 の input_errors に本文の形の誤りが無く、X2 の never_contains が利用者の本文の ARN と食い違っていた | 契約 X1 と X2 を直す。確認: contract-summary.md の X1 に 60,000 文字と形の確かめが入り、X2 の never_contains は tag-refactor が組み立てる部分に限られ、「Construction での直し」の節に記録がある | Resolved (reviewer) |
| R-04 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/nfr-requirements/security-requirements.md > NFR1.6、NFR1.7、NFR1.8 | 止める形と知らせる形が伏せ字の一覧と対応せず、トークンの最小の長さと複数箇所の表示も未定だった | redact との対応表、最小の長さ、複数箇所の表示を決める。確認: 「本文の形の一覧」の表、20文字の境界のテスト、10か所と「ほか N か所」が入った | Resolved (reviewer) |
| R-05 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/nfr-requirements/security-requirements.md > NFR1.8 | タイトルに伏せ字を通すかが読み取れず、タイトルの ARN のテストも無かった | 部品ごとの表にし、タイトルの ARN のテストを足す。確認: 「部品ごとの伏せ字」の表と NFR1.8 の合否の基準で解消 | Resolved (reviewer) |
| R-06 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/nfr-requirements/security-requirements.md > NFR1.9 | 続きの命令に伏せ字を通すか未定で、URL を出す場所も不明だった | 通す、通さないを決め、URL の文を特定するか削る。確認: 通さない理由と、タイトルは通したあとのものを使うこと、URL は命令に入れないことが表に書かれた | Resolved (reviewer) |
| R-07 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/nfr-requirements/tech-stack-decisions.md > NFR6.12 | git check-ref-format が @{-1} を展開して成功し、先頭が - の値の扱いも無かった | 出力が入力と同じことを条件にし、先頭が - の名前を呼ぶ前に断り、テストに入れる。確認: NFR6.12 に3点とも入った | Resolved (reviewer) |
| R-08 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/nfr-requirements/security-requirements.md > NFR1.10 | 本文のファイルの名前の作り方、Windows の権限、中断時の扱いが未定だった | 名前の作り方、Windows の扱い、中断時の扱いを決める。確認: 乱数つきの名前、中断時は残す、Windows は権限を変えず残るリスクに記載、で解消。名前の長さは R-13 に分けた | Resolved (reviewer) |
| R-09 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/nfr-requirements/tech-stack-decisions.md > NFR6.11 から NFR6.15 | 合否の基準の欄が無く、GitHub の数え方の確認も無かった | 基準の欄を足し、数え方は出典か仮の前提にする。確認: 全行に基準の欄があり、NFR6.14 は仮の前提と書かれ、失敗時の逃げ道もある | Resolved (reviewer) |

### New findings

| ID | Severity | Location | Finding | Required action | Status |
|---|---|---|---|---|---|
| R-10 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/nfr-requirements/tech-stack-decisions.md > NFR6.15(および security-requirements.md > 続きの命令の引用) | PowerShell の合否の基準が「構文の解析器に通す」だけで、解析器は文字列の値を返すが、外のプログラムが実際に受け取る引数は確かめない。Windows PowerShell 5.1 は空白を含む引数を二重引用符で包んで渡すので、タイトルの末尾が円記号(バックスラッシュ)で空白も含むと、閉じの引用符が打ち消されて引数が変わりうる(既知の挙動。実機での確認が要る)。表の「安全に囲めない値」にも入っていない | PowerShell のテストを、引数をそのまま出す小さなプログラムを実際に呼んで受け取った引数を比べる形にする(Windows の CI)。末尾が円記号の値と空白を含む値を、テストの値と「安全に囲めない値」の候補に足し、実機で壊れるなら手順の文にする | New |
| R-11 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/nfr-requirements/security-requirements.md > 本文の形の一覧(URL の userinfo の行) | 人が承認した NQ2-Q1 = A は、止める形を GitHub のトークンと AWS のアクセスキーの ID に限っていた。パスワードつきの URL を止める決めは、この文書で足され、承認された要約に無い(契約 X1 にも足された)。上書きのオプションが無いので、説明用の例(user:pass@host)を書く利用者も止まるが、残るリスクの回避方法はトークンの例しか触れていない | パスワードつきの URL を止める決めを、人に確認する決めとして記録するか、知らせるだけに改める。止めるなら、README に書く回避方法の例に URL の形も含める | New |
| R-12 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/nfr-requirements/security-requirements.md > U1 の要件の当て方(NFR1 の行) | NFR1 の行は U1 の NFR1.1 から NFR1.5 を「そのまま」当てるとするが、U1 の NFR1.2(本文と表示に ARN とアカウント ID を出さない)は、NFR1.7(利用者の本文の ARN をそのまま載せる)と NFR1.9(続きの命令の値は伏せない)で狭まる。例外は別の箇所に書かれ、この行は矛盾して読める | NFR1 の行に、U1 の NFR1.2 は NFR1.7 と NFR1.9 の範囲で例外があると書く | New |
| R-13 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/nfr-requirements/security-requirements.md > NFR1.10 | 本文のファイル名はブランチ名(記号を - に置き換えたもの)を含む。ブランチ名は区切りごとにフォルダー名の上限に収まっても、 / を - にするとファイル名が 255 文字を超えうる。作成が Publish の始めに失敗するときの扱い(どの終了コードで、何を示すか)も書かれていない | ブランチ名の部分に上限(例: 80 文字で切る)を決め、本文のファイルが作れないときは、どのファイルも変えずに入力の誤りに近い形で止めることと、その確かめの受け入れテストを書く | New |

### Summary

前回の R-01 から R-09 は、すべて成果物と契約の直しで解消したことを確かめました。Critical と Major は残っておらず、新しい指摘は Minor の4件(PowerShell の引数の確かめ、パスワードつき URL の承認の範囲、U1 の NFR1.2 との書き方、本文のファイル名の長さ)です。次のステージに進めますが、R-10 と R-11 は NFR Design かコード生成の前に直すことを勧めます。

### Findings

| ID | Severity | Location | Finding | Required action | Status |
|---|---|---|---|---|---|
| R-01 | Major | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/nfr-requirements/traceability.json > upstream_ids と reverse | upstream_ids が NFR1 と NFR6 の2つだけで、NFR5、NFR7、NFR8、NFR9 が一覧にも N/A にも出てこない。NFR9(実際の GitHub にプルリクエストを作る結合テスト)は U2 の中心の振る舞いに直接かかわる。NFR2、NFR3、NFR4、NFR10 の N/A は、決まった形の coverage ではなく、独自の reverse という配列に書かれており、確かめの道具が読まない。U1 の traceability.json は NFR1 から NFR10 のすべてを coverage に置いている | upstream_ids に NFR1 から NFR10 のすべてを並べ、coverage に入れる。U1 をそのまま使うものは、U1 のどの NFRx.y に従うかと理由つきの N/A(または OK)にする。reverse の配列はやめて coverage に統合する。NFR9 は U2 で何を足すか(または U1 の NFR9.1 で足りること)を明記する | New |
| R-02 | Major | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/nfr-requirements/tech-stack-decisions.md > NFR6.15(および security-requirements.md > NFR1.9) | NFR1.9 は「ブランチ名やファイル名に引用符や $ があってもシェルが別の命令として読まない」と約束するが、NFR6.15 の引用の決まりは PowerShell で ' を '' にするだけである。PowerShell は U+2018、U+2019、U+201A、U+201B も一重引用符として扱うため、ファイル名やブランチ名にこれらの文字があると文字列が途中で閉じ、続きが別の命令として読まれる。Windows PowerShell 5.1 は引数の中の二重引用符を ネイティブの命令に渡すときに落とすので、" を含むブランチ名やタイトルでも引数が変わる。git のブランチ名には ' や " や $ が使える。合否の基準のテストも、これらの文字と改行を含む名前を挙げていない | NFR6.15 に、PowerShell では ' に加えて上の4つの引用符も二重にすること、二重引用符と改行など引用で安全に渡せない文字を含む名前の扱い(たとえば、その命令を示さず手順の文で示す)を決める。NFR1.9 の合否の基準に、これらの文字を含む名前のテストを足す | New |
| R-03 | Major | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/nfr-requirements/security-requirements.md > NFR1.6 と NFR1.7 | NFR1.6 は、契約 X1 の input_errors に無い新しい入力の誤り(本文のトークンの形)を足している。契約 X2 の never_contains は ARN とアカウント ID を本文に入れないとしているが、NFR1.7 は利用者の本文にそのまま入れると決めている。契約が直されないまま、実装者が契約と NFR のどちらに従うかを推測することになる | 契約 X1 の input_errors に「--pr-body-file の中身に GitHub のトークンか AWS のアクセスキー ID の形がある」を足し、X2 の never_contains を「tag-refactor が組み立てる部分」に限ると直す(または契約の修正が要ることをこの文書に明記し、直す段階と持ち主を決める) | New |
| R-04 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/nfr-requirements/security-requirements.md > NFR1.6、NFR1.7、NFR1.8 | 形の範囲が伏せ字(internal/redact)と一致しない。伏せ字の閉じた一覧には URL の userinfo(://user:token@)があるが、NFR1.6 と NFR1.7 はこれを止めも知らせもせず、利用者の本文にそのまま載る。GitHub のトークンの形は、redact では prefix のあとに1文字以上でよいが、NFR1.6 は「始まる並び」とだけ書き、最小の長さを決めていない。止める道具には誤検出の逃げ道(上書きの方法)も無い。同じ形が本文に複数あるとき、1行の知らせにどう載せるかも未定である | 止める形と知らせる形の一覧を、redact の一覧との対応表で書き(userinfo は止める、知らせる、対象外のどれかを決める)、トークンの形の最小の長さと境界のテストを決める。複数あるときの表示(例: 種類ごとに行の番号をまとめて1行)も決める | New |
| R-05 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/nfr-requirements/security-requirements.md > NFR1.8 | 利用者のタイトル(--pr-title)に伏せ字を通すかどうかが読み取れない。「利用者の本文と利用者のタイトルを含む全体には、二重に伏せ字を通さない」と書く一方、「ただしタイトルは、伏せ字を通したあとで 256 文字」と書く。機能の設計(BR1.1、BR1.3)は利用者のタイトルに1回伏せ字を通す。本文の ARN は伏せず、タイトルの ARN は伏せる、という差も明記されていない。合否の基準のテストも本文だけである | 「利用者のタイトルは1回だけ伏せ字を通す(BR1.1)。利用者の本文は通さない」のように、部品ごとに通す、通さないを表にして書く。タイトルに ARN を入れるテストを合否の基準に足す | New |
| R-06 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/nfr-requirements/security-requirements.md > NFR1.9 | 続きの命令を標準エラーに出すとき伏せ字を通すかどうかが決まっていない。NFR1.2 は表示に伏せ字を通すとするが、ファイル名やブランチ名に12桁の数字や ARN の形があると、続きの命令が <redacted> に書き換わって動かなくなる。また「リモートの URL は userinfo を除いて出す」とあるが、続きの命令の形(push は リモート名、gh は owner/name)には URL が出てこず、どこで URL を出すのか分からない | 続きの命令は、伏せ字を通すか(動かなくなる場合の扱いも)通さないか(通さない理由と、タイトルは通した後のものを使うこと)を決める。URL を出す場所を特定するか、その文を削る | New |
| R-07 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/nfr-requirements/tech-stack-decisions.md > NFR6.12 | git check-ref-format --branch は、@{-1} のような記法を前のブランチの名前に展開して成功する。終了コードだけで確かめると、正しい名前として通り、あとの git switch -c の段階で、ファイルを書き換えたあとに失敗する。-- で始まる値をオプションとして読ませる恐れもある | 出力が入力と同じことも合格の条件にする。先頭が - の名前は呼ぶ前に断る。受け入れテストに @{-1} と - で始まる名前を入れる | New |
| R-08 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/nfr-requirements/security-requirements.md > NFR1.10 | 本文のファイルの名前の「名前」が何かを決めていない(現行のコードはブランチ名の記号を - に置き換える)。同じ名前に置き換わる別のブランチ名で上書きが起きる。Windows で自分だけが読み書きできるファイルにする方法と確かめ方が無い(Unix の 0600 だけ)。中断(Ctrl+C)のときにファイルを残すか消すかも決めていない | 名前の作り方(例: 置き換えたブランチ名に日時か乱数)、Windows の扱い(確かめられないなら、そう書いて残るリスクに入れる)、中断のときの扱いを決める | New |
| R-09 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/nfr-requirements/tech-stack-decisions.md > NFR6.11 から NFR6.15 | 表に合否の基準の欄が無い。U1 の NFR6.x も同じ形なので衝突はしないが、NFR6.11(線形時間、1 MiB の本文で何秒か)、NFR6.14(60,000 文字の数え方の境界)は測れる基準にできる。NFR6.14 の「GitHub の本文の上限に近い数え方」は、GitHub が文字数かバイト数で数えるかの確認がされていない | 各行に、テストで確かめる基準(境界の文字数、1 MiB の本文を確かめる時間の目安)を足す。GitHub の数え方は出典を示すか、仮の前提と書く | New |

> R-01 Not re-checked this round

> R-02 Not re-checked this round

> R-03 Not re-checked this round

> R-04 Not re-checked this round

> R-05 Not re-checked this round

> R-06 Not re-checked this round

> R-07 Not re-checked this round

> R-08 Not re-checked this round

> R-09 Not re-checked this round
