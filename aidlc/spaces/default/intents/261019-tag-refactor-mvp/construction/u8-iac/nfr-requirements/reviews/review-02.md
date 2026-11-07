## Review

**Verdict:** READY
**Reviewer:** aidlc-architecture-reviewer-agent
**Date:** 2026-10-31T03:43:40Z
**Iteration:** 2

### Findings

| ID | Severity | Location | Finding | Required action | Status |
|---|---|---|---|---|---|
| R-01 | Major | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u8-iac/nfr-requirements/security-requirements.md > NFR3.20 | 「この2つのコマンドは利用者の git のフックを動かさない」は事実と合わない。git status は core.fsmonitor に設定されたプログラム(リポジトリの .git/config に書かれていることもある)を実行しうるため、他人から受け取ったリポジトリを --dir に渡すと任意のコマンドが走る。環境変数 GIT_DIR や GIT_WORK_TREE が残っていると -C <dir> が効かず別のリポジトリを見る。git status は index も更新しうる(読み取りだけとは限らない)。時間の上限も、Windows では git の子プロセスが残ると待ち続けることがある | 呼び出しを固定する: core.fsmonitor を無効にする設定の上書き、--no-optional-locks(または GIT_OPTIONAL_LOCKS=0)、GIT_DIR・GIT_WORK_TREE・GIT_INDEX_FILE などを環境から除く、を要件に書く。時間の上限は子プロセスごと止める(cmd.WaitDelay など)ことも書く。「フックを動かさない」という断定は、確かめるテスト(fsmonitor を設定したリポジトリで実行されないこと)つきの要件にするか、削る | Resolved (reviewer) |
| R-02 | Major | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u8-iac/nfr-requirements/security-requirements.md > NFR3.20(status --porcelain=v1 -z -- <ファイル>) | 「--」はパス指定の特殊な書き方(:(top) などのマジック、* ? [ のワイルドカード)を止めない。a[1].yaml のような名前は別のファイルに当たるか、どれにも当たらず「変更なし」と判定され、未コミットの変更を上書きしうる。--ignored を付けないので .gitignore 対象のファイルは常に「変更なし」になり、git で戻せない(BR5.2 の前提、NFR4 を当てはまらないとした理由)のに書き換わる。rev-parse は bare リポジトリや .git の中でも終了コード 0 で false を出すが、出力の true を読む要件がない。書き換えるファイルが多いと、Windows の引数長の上限(約32,000文字)で git が起動できない。テストの例にワイルドカードや : で始まる名前、ignore 済みのファイルがない | リテラルのパス指定(--literal-pathspecs または GIT_LITERAL_PATHSPECS=1)を要件にする。書き換えるファイルが git に追跡されているか(ls-files)または ignore されていないことも確かめ、追跡も管理もされていないファイルは止めるかどうかを決めて合否の基準にする。rev-parse は出力が true であることを条件にする。ファイルが多い場合の分割か --pathspec-from-file(--pathspec-file-nul)を要件にする。テストに [ * : で始まる名前、ignore 済み、約1,000ファイルの同時指定を足す | Resolved (reviewer) |
| R-03 | Major | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u8-iac/nfr-requirements/security-requirements.md > NFR3.19 と Q2 | 上限が全体を守れない。(1) 「YAML の別名の展開は解析器の上限を使う」は根拠がない。AST(トークン位置を使う設計)を解析するだけでは別名は展開されず、展開の上限は効かない。goccy/go-yaml にそのような上限があることは確かめられていない。(2) 10 MB かつ 20,000 個なら最大 200 GB を読み、合計の大きさとメモリの上限がない。(3) 深い入れ子(`[[[[…`)は 10 MB に収まり、再帰でスタックを使い切る(Go のスタックの使い切りは回復できない)。YAML・JSON・HCL の入れ子の深さの上限がない。(4) 「見るファイル 20,000 個」が、歩いたすべてのファイルか候補の拡張子のファイルだけかが書かれていない。.json はすべて候補なので、モノレポでは正常な使い方で InvalidInput になりうる。(5) 変更案と --tf-state の上限が「NFR の設計で決める」のままで、NFR5.25 の 10,000 リソースの規模を通すかどうかも決まっておらず、基準を試せない。(6) 合否の「メモリを使い切らず」は数値がなく試せない | 数値と方法を要件で決める: 別名・アンカーの数または展開後の大きさの上限(または別名を含むファイルを事前に外す)と、その確かめ方、YAML・JSON・HCL の入れ子の深さの上限、1回の実行で読む合計の大きさとメモリ(ピーク)の上限、数える対象のファイル(候補の拡張子だけ)、変更案と --tf-state の大きさの上限(NFR5.25 の規模より大きい値)。YAML の爆弾のテストは、使う解析器の実際の振る舞い(展開されるか、外されるか)を確かめて期待を書く。メモリはテストで MemStats などの数値で比べる | Resolved (reviewer) |
| R-04 | Major | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u8-iac/nfr-requirements/security-requirements.md > NFR8.27 | 「--debug でも差分の外の中身を出さない」の試験が、パスワードを持つ行がある成功した場合だけで、解析に失敗した場合を見ていない。goccy/go-yaml の構文エラーは該当行を含むソースの抜粋を文字列にする。候補は --dir の下のすべての .yaml と .json(秘密を含む設定ファイルなど、差分と無関係なものを含む)なので、BR3.1 と BR4.6 が --debug で出す「詳細」に、差分に出ない行が混ざりうる。--tf-state の内容(terraform show -json にはパスワードなどの値が入る)も、出さないものの一覧にない | 解析エラーを --debug で出すときは、ファイル名・行・桁・種類だけにして、ソースの抜粋を含めないと要件に書く。出さないものの一覧に --tf-state の values を加える。テストに、秘密の値を含み構文の誤りがある YAML・JSON・.tf と、秘密の値を含む --tf-state を足し、--debug の標準出力と標準エラーに値が出ないことを確かめる | Resolved (reviewer) |
| R-05 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u8-iac/nfr-requirements/tech-stack-decisions.md > NFR5.25 と測る条件 | 60 秒の目標に根拠となる測定がない。goccy/go-yaml は大きなファイルの解析が遅いことが知られ、YAML 約 50 MB と AST の保持で、遅い macOS・Windows のランナーでは余裕が小さい恐れがある。計測の範囲(ファイルの作成を含むか)が書かれておらず、共有ランナーでの60秒の固定の判定はぶれる。git は差し替えなので、実際の git status の時間は測られない。メモリの目標もない | Code Generation の最初に、YAML・HCL の解析の速さを小さく測る手順(または余裕を持たせた基準)を足す。計測は解析から書き出しまでと定め、ファイルの作成を除く。ぶれを避けるため、基準を余裕つき(例: 実測の2倍以内)で決めるか、CI では失敗の判定を緩め記録する。実 git を使う手元の計測も1回行う | Resolved (reviewer) |
| R-06 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u8-iac/nfr-requirements/security-requirements.md > NFR6.5 | 書いたあとの読み直しが名前の変更のあとなので、違った場合には壊れた内容がすでに元のファイルの場所にあり、戻す手順がない(「止める」だけ)。また、U6 の writeAtomic は .json 固定の名前、0600 の権限、記録用のエラー文を持ち、1時間より古い「.」で始まる .tmp- ファイルを消す removeOldTemps もあるため、そのままでは利用者のフォルダーに使えない。名前の変更の再試行は5回×100ms(約0.5秒)で、エディターやウイルス対策が開いている利用者のファイルでは短い恐れがある。iac から journal への依存の向きも決まっていない | 読み直しは、一時ファイルの内容を名前の変更の前に確かめる形にするか、名前の変更の後に違いがあれば元の内容を書き戻すと決める。共通の部品は journal の外(中立のパッケージ)に切り出し、利用者のフォルダーでは古い一時ファイルを消さないと要件に書く。U6・U7 のテストが変わらず通ることを合否に加える。再試行の回数・待ちを U8 で足りるか確かめる | Resolved (reviewer) |
| R-07 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u8-iac/nfr-requirements/security-requirements.md > NFR3.17 と NFR3.18 | NFR3.17 のテストの組み合わせに、日本語など複数バイトの文字(コメント・値・キー)、改行の種類が混ざったファイル、UTF-16 の BOM がない。goccy/go-yaml のトークンの位置が、バイトではなく文字の位置の場合は、日本語の利用者でずれる(BR4.4 の範囲の確かめで書き換えは止まるが、反映できないファイルが増える)。NFR3.18 は Windows の CI でシンボリックリンクのテストを飛ばし、ジャンクション(mklink /J、権限不要)など Windows 特有のリンクを確かめない。--dir を指す部分の扱いも決まっていない | NFR3.17 の組み合わせに複数バイトの文字と混在した改行を足し、位置の単位(バイト)をテストで確かめることを書く。NFR3.18 は Windows でジャンクションのテストを足し、--dir 自体がリンクの場合の扱いを決める | Resolved (reviewer) |
| R-08 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u8-iac/nfr-requirements/tech-stack-decisions.md > 新しい依存の扱い(Q1)と当てはまらない要件 NFR12 | 新しい依存に関する約束が、合否の基準を持つ要件になっていない。README の MPL-2.0 の節、Dependabot と govulncheck の対象の確認、間接の依存の数と実行ファイルの大きさの記録、README の「差分にファイルの中身が出る」(NFR8.27)は本文だけにある。MPL-2.0 のバイナリでは、受け取る人にソースの入手先を知らせる義務があり、NFR12 を「当てはまらない」として配布に送ると、持ち主がいない。hcl/v2 が要求する go のバージョンが CI の oldstable と合うかも確かめていない | 本文の約束を、合否の基準つきの要件にする(README の節の有無を確かめる手順、go.mod と dependabot の設定の確認、間接依存の記録)。NFR12 を、U8 が足した義務の持ち主を明記した「配布へ引き継ぐ」項目に変える。go.mod の go の行と oldstable で hcl/v2 がビルドできることを CI のジョブで確かめる | Resolved (reviewer) |
| R-09 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u8-iac/nfr-requirements/tech-stack-decisions.md > 技術の選択(JSON の解析)と security-requirements.md > NFR3.21 | 機能設計で NFR に送られた確かめが、要件になっていない: (1) CloudFormation の Type と resourceType の対応表、terraform show -json の values.arn がある種類の確かめ。(2) Terraform は偽の state で確かめるとあるが、偽の state は実物の出力とずれうる。実際の terraform show -json から採った例を使う要件がない。(3) JSON の解析器が「NFR の設計で決める」のまま(エスケープの正しさに関わる)。(4) 深さの上限で外したフォルダーに警告がなく、そこにあるテンプレートが templateNotFound になる理由が分からない。(5) NFR10.12 の合否は全体の80%なので、U8 のコードが低くても通る | 対応表のすべての種類を数えて確かめるテストを要件にする。実物の terraform show -json から採った例(版を記録)をテストに使う要件を足す。JSON の解析器を選ぶか、選ぶ基準を決める。深さの上限や数の上限で外したものの数も警告に出す。U8 のパッケージ単体の下限(または U8 のコードを含めて下がらないこと)を基準にする | Resolved (reviewer) |
| R-10 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u8-iac/nfr-requirements/security-requirements.md > NFR3.21 | 検証では test/sandbox/ を一時フォルダーの git のリポジトリに写して iac --write を実行するが、写しをコミットする手順がない。NFR3.20 は追跡されていないファイルがあれば止めるので、そのままでは検証コマンドが必ず止まる | 検証の手順に、写したあとで git init、追加、コミットを行うことを書く。コミットの者の名前などは一時のリポジトリの設定で与え、利用者のグローバルの設定に頼らない | New |
| R-11 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u8-iac/nfr-requirements/security-requirements.md > NFR3.19 (b) と合否の基準のヒープ 2 GB | 合計 500 MB まで読むと定めているが、ヒープ 2 GB の確かめは約 100 MB の大きな試験でしか行わない。上限いっぱい(500 MB)で 2 GB に収まるかは確かめられず、事前の確かめ (f) の数え方(ブロックスカラー、ヒアドキュメント、引用符の中の括弧)が正しいファイルを誤って外さないかも試験にない | 合計が上限に近い入力(解析木を持ち続けないことで収まること)の試験を 1 つ足すか、ヒープの目標を 100 MB の規模に限ると書く。事前の確かめで、ブロックスカラーとヒアドキュメントの中の括弧が数えられても正しいファイルが外れないことの試験を足す | New |
| R-12 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u8-iac/nfr-requirements/tech-stack-decisions.md > NFR12.1 | 合否の基準が、unit-of-work.md の配布の項目に MPL-2.0 の義務が書き足されていることだが、現在の unit-of-work.md の該当項目(配布の仕組みの行)にはまだ書かれていない。このステージの成果物ではない文書への書き足しに依存している | 書き足しを、このステージの承認の前か、Code Generation の完了の条件として誰が行うかを明記する。書き足しをしない場合は、持ち主の記録を traceability.json か構築の次の文書のどちらかに置くと決める | New |

> R-01 Reviewer note: NFR3.20 が GIT_ の環境変数の除去、GIT_OPTIONAL_LOCKS=0、-c core.fsmonitor=false、cmd.WaitDelay による子プロセスの打ち切りを定め、fsmonitor の印と GIT_DIR のテストが付いた。断定は削られた

> R-02 Reviewer note: GIT_LITERAL_PATHSPECS=1、rev-parse の出力が true、ls-files での追跡の確認、--ignored=matching と --untracked-files=all、30,000 文字での分割が入り、特殊な名前・ignore・約1,000ファイルのテストも付いた

> R-03 Reviewer note: 候補の拡張子だけを数える、合計 500 MB、別名を展開しない、深さ 256 の事前の確かめ、変更案 200 MB と tf-state 500 MB、ヒープ 2 GB を MemStats で確かめる、と数値と試験が揃った。ただし下の Minor の残りがある

> R-04 Reviewer note: 解析の誤りは名前・行・桁・種類だけ、tf-state の値も出さない、と定まり、構文の誤りと秘密の値のテストが付いた

> R-05 Reviewer note: 計測の範囲が決まり、最初の小さな計測と 30 秒の見込みの基準、実 git での手元の計測が加わった

> R-06 Reviewer note: 一時ファイルを名前の変更の前に確かめる形になり、部品を internal/fileout(実在を確認)に寄せ、利用者のフォルダーで古い一時ファイルを消さず、Windows のやり直しを約5秒にした。U4・U6・U7 のテストが通ることも基準にある

> R-07 Reviewer note: 複数バイトの文字、混在した改行、UTF-16 の BOM、Windows のジャンクション、--dir 自体がリンクの場合がテストと要件に入った

> R-08 Reviewer note: NFR10.13、NFR9.12、NFR12.1 が合否の基準つきの要件になり、traceability の NFR12 も持ち主つきで直った。引き継ぎの記録の残りは下の Minor

> R-09 Reviewer note: 対応表と実物の terraform show -json の例が NFR10.14、JSON は encoding/json に決まり、深さで外したフォルダーは警告に数え、U8 のパッケージ単体の80%も基準に入った

### Validation Tool Results

| Tool | Result | Interpretation |
|---|---|---|
| (なし) | ステージの定義に検証の道具がない | 道具は走らせていない。ID は U8 の中で重ならず、traceability.json の NFR10.14 と NFR12.1 は本文と一致した。internal/fileout の実在を確かめた |

### Summary

前回の Major 4件と Minor 5件はすべて直り、git の呼び出し、ファイルの上限、--debug の漏れ、書き出しの安全は実装できる水準の数値と試験になった。残りは検証手順のコミットの抜け、上限いっぱいでのメモリの確かめ、配布への引き継ぎの記録の3つの Minor だけで、READY とする。
