# ビジネスルール: U2 プルリクエストの作成(残り)

U2 の決まりです。出典は、要件(`inception/requirements-analysis/requirements.md` の FR1、NFR1、NFR3、NFR10)、契約(`inception/contract-design/contract-summary.md` の X1、X2、C1)、このステージの回答(FD2-Q…、`functional-design-questions.md`)です。U1 で作った決まり(U1 の rules.md の BR4.1〜BR4.5)は変えずに使い、ここでは足すものと仕上げるものを書きます。

```yaml
rules:
  # 1. オプションと入力の誤り
  - id: BR1.1
    statement: --pr-branch、--pr-title、--pr-body-file は --pr と一緒にだけ使え、書き換えの前に確かめる
    category: validation
    applies_to: PullRequestOptions
    trigger: iac を実行したとき
    logic: |
      IF --pr が無いのにどれかがある THEN 入力の誤り。
      --pr-branch は git check-ref-format --branch で正しいこと。--pr-title は空でなく、改行を含まず、256 文字まで。
      --pr-body-file は、コマンドを動かしたフォルダーからの相対か絶対で、読めて、UTF-8 で、1 MiB まで(FD2-Q3)。
      加えて、--pr-body-file の中身が 60,000 文字を超えるときも入力の誤りにする(本文の上限 65,536 文字から、変更したファイルの一覧と見出しのための 5,536 文字を残す)。
      --pr-title は、伏せ字(BR3.2 と同じ関数)を通したあとの文で、空でないこと、256 文字までであることを確かめる。伏せ字で文が変わったときは、そのことを標準エラーに1行で示す
    violation: どのファイルも変えずに理由と対処を示し、終了コード 1
    source: FR1.4、契約 X1 input_errors、FD2-Q2
  - id: BR1.2
    statement: 既定のブランチ名は、UTC の日時を入れた tag-refactor/iac-<YYYYMMDD-HHMMSS>Z にする
    category: calculation
    applies_to: DefaultBranchName
    trigger: --pr-branch が無いとき
    logic: 実行を始めたときの時刻を UTC に直して名前を作る
    violation: なし
    source: FR1.4、FD2-Q1
  - id: BR1.3
    statement: タイトルとコミットのメッセージは、--pr-title があればその文、無ければ既定の文(--lang に従う)
    category: policy
    applies_to: PullRequestOptions
    trigger: 作成のとき
    logic: IF --pr-title がある THEN その文を伏せ字を通して使う。ELSE 既定の文(U1 の BR4.3)。どちらもコミットのメッセージの1行目に同じ文を使う
    violation: なし
    source: FR1.4、U1 の BR4.3

  # 2. 書き換えの前の確かめ(U1 の BR4.1 に足すもの)
  - id: BR2.1
    statement: 作るブランチと同じ名前のブランチが、手元にもリモートにも無いことを確かめる
    category: validation
    applies_to: PullRequestPlan
    trigger: --pr を付けて、書き換えの前
    logic: 手元は refs/heads/<ブランチ> の有無、リモートは git ls-remote --heads <リモート> refs/heads/<ブランチ> で確かめる
    violation: どのファイルも変えずに理由と対処(--pr-branch で別の名前を付ける)を示し、終了コード 1
    source: FR1.5、AC1.1.4
  - id: BR2.2
    statement: リモートは今のブランチの追跡先、無ければ origin。どちらも無ければ止める
    category: policy
    applies_to: PullRequestPlan
    trigger: 書き換えの前
    logic: branch.<今のブランチ>.remote があればそれ、無ければ origin。origin も無ければ止める
    violation: どのファイルも変えずに理由と対処を示し、終了コード 1
    source: FR1.5、AC1.1.3
  - id: BR2.3
    statement: --write と同じ確かめ(--dir が git の作業ツリーで、書き換えるファイルがコミット済みで変更が無い)も行う
    category: validation
    applies_to: PullRequestPlan
    trigger: 書き換えの前
    logic: 今の iac の gitCheck をそのまま使う。--pr の確かめ(BR2.1、BR2.2、U1 の BR4.1)が通ったあとで行う
    violation: どのファイルも変えずに止まり、終了コード 1
    source: FR1.6、AC1.1.4
  - id: BR2.4
    statement: コミットには iac が書き出したファイルだけを入れ、フックと署名は利用者の git の設定どおりに動かす
    category: constraint
    applies_to: PublishInput
    trigger: コミットの段階
    logic: |
      書き出したファイル(リポジトリの根からの相対)を git add -- <files> でステージし、git --literal-pathspecs commit --only -m <タイトル> -- <files> でコミットする。
      利用者がすでにステージしていたほかのファイルは、コミットに入らず、ステージされたまま残る。
      commit は端末につないで動かし(U1 の BR4.5)、利用者のフック(pre-commit、commit-msg など)と署名(commit.gpgsign)をそのまま動かす。tag-refactor は --no-verify も --no-gpg-sign も付けない
    violation: フックや署名が失敗したら、コミットの段階の失敗として BR4.3 で止まる
    source: FR1.7、FR1.8、AC1.1.5、U1 の BR4.2

  # 3. 本文
  - id: BR3.1
    statement: 本文は、変更したファイル(IaC の種類つき)と手で直す一覧を、--lang の言語で載せる
    category: calculation
    applies_to: PullRequestBody
    trigger: 書き出したファイルがあり、作成に進むとき
    logic: |
      --pr-body-file が無ければ、紹介の 1〜3 行、## Changed files (N)、## To fix by hand (N) の順。
      あれば、ファイルの中身、## Changed files (N)、## To fix by hand (N) の順。
      手で直す項目が 0 件なら、その節は省く。見出しと文は --lang に従う
    violation: なし
    source: FR1.10、契約 X2、AC1.1.6
  - id: BR3.2
    statement: 手で直す一覧の理由は、表示の文の一覧の固定の文だけを使い、tag-refactor が組み立てる部分は伏せ字を通す
    category: constraint
    applies_to: ManualLine
    trigger: 本文を組み立てるとき
    logic: |
      理由は ManualReason ごとのカタログの文(cdkAppFailed は道具の名前を足す)。
      伏せ字は、本文の組み立て(IacDisplay)が、tag-refactor の作る部分(紹介、変更したファイル、手で直す一覧)にだけ、つなぎ合わせる前に通す。
      --pr-body-file の中身は通さない(契約 X2)。利用者が書いた文の中身は利用者の責任とし、README の iac の節にそう書く。
      そのため、Publish は受け取った本文をそのまま使い、本文の全体に伏せ字を通さない(U1 の Publish の redact.String(in.Body) をやめる)。タイトルは BR1.1 で伏せ字を通したものを使う
    violation: なし
    source: 契約 X2、NFR1
  - id: BR3.3
    statement: 本文が 65,536 文字を超えるときは、手で直す一覧の後ろを省き、それでも超えるときは作る前に止める
    category: constraint
    applies_to: PullRequestBody
    trigger: 本文を組み立てたあと
    logic: |
      IF 本文が 65,536 文字を超える THEN 手で直す一覧の後ろから1行ずつ省き、最後に「ほか N 件(全部はこのコマンドの表示を見てください)」の1行を置く。
      --pr-body-file の中身は BR1.1 で書き換えの前に 60,000 文字までと確かめているので、省いても超えるのは、変更したファイルの一覧だけで残りの 5,536 文字を超える(およそ 80 ファイル以上)ときに限る。そのときは作成に進まない
    violation: 作成に進まないときは、書き出したファイルはそのままに、理由と、プルリクエストを手で作る方法(書き出したファイルをコミットして push し、gh pr create で作る)を示して終了コード 1(ブランチもコミットも作らない)
    source: FD2-Q2

  # 4. 結果と終了コード
  - id: BR4.1
    statement: 書き換えるものが無ければ、ブランチもプルリクエストも作らない
    category: policy
    applies_to: PublishInput
    trigger: 書き換えのあと
    logic: |
      IF 書き出したファイルが無く、手で直す項目も、synth できなかった(failed か skipped の)CDK のアプリも無い THEN「反映するものはありません」と示して終了コード 0。
      IF 書き出したファイルが無く、手で直す項目か synth できなかったアプリがある THEN 手で直す一覧とアプリの状態を示して終了コード 2(契約 C7 exit_code_rule)
    violation: なし
    source: FR1.11、AC1.1.7、AC1.1.8
  - id: BR4.2
    statement: 作成が済んで、手で直す項目があれば終了コード 2、無ければ 0
    category: calculation
    applies_to: PublishOutcome
    trigger: 作成のあと
    logic: 終了コードは 1 > 2 > 0 の順で決める(契約 X1)。作成が済み、手で直す項目か synth できなかったアプリがあれば 2
    violation: なし
    source: FR1.13、AC1.1.6
  - id: BR4.3
    statement: 途中で失敗したら、済んだ段階、失敗した段階と原因、続きの命令、本文のファイルの場所を標準エラーに示す
    category: policy
    applies_to: ResumeCommand
    trigger: ブランチ、コミット、push、作成のどれかが失敗したとき
    logic: |
      本文のファイル(<Git の管理フォルダー>/tag-refactor/pr-body-<名前>.md)は、Publish の始め(ブランチの段階の前)に書く。こうして、どの段階で失敗しても、続きの命令が指すファイルがある。成功したら消し、失敗したら残す。
      続きの命令は、失敗した段階から作成までの分を1行ずつ作る。どの命令も、リポジトリの根を git -C で名指し、パスはリポジトリの根からの相対にする。
        branch: git -C '<根>' switch -c '<ブランチ>'
        commit: git -C '<根>' add -- '<ファイル1>' '<ファイル2>' のあと git -C '<根>' --literal-pathspecs commit --only -m '<タイトル>' -- '<ファイル1>' '<ファイル2>'
        push:   git -C '<根>' push '<リモート>' 'refs/heads/<ブランチ>:refs/heads/<ブランチ>'
        create: gh pr create --repo '<owner/name>' --base '<行き先>' --head '<ブランチ>' --title '<タイトル>' --body-file '<本文のファイル>'
      tag-refactor が動いている OS で、Windows は「PowerShell 用」、それ以外は「sh 用」と示し、単一引用符で囲む(中の ' は PowerShell では ''、sh では '\'' にする)。本文は命令に入れない。
      gh pr create が成功で終わったのに URL を読み取れないときは、成功として扱い、「gh pr list --head <ブランチ> で確かめてください」と示す(終了コードは BR4.2 のとおり)。
      --debug が無くても示す(FD2-Q4)。済んだことは元に戻さない
    violation: 終了コード 1
    source: FR1.12、AC1.1.9、契約 C1 resume、FD2-Q4
  - id: BR4.4
    statement: git と gh の呼び出しの上限で止まっても、出力とログにトークンを出さない
    category: constraint
    applies_to: PullRequestPlan
    trigger: 応答しない git や gh
    logic: U1 の上限(読むだけの git と switch -c と add は 30 秒、commit と push は 10 分、gh は 2 分)で止め、その段階の失敗にする。表示と --debug は伏せ字を通す
    violation: 終了コード 1
    source: NFR1、NFR3、AC1.1.10、U1 の BR4.5

  # 5. 範囲
  - id: BR5.1
    statement: --pr は CDK の有無によらず、CloudFormation と Terraform の書き換えでも使える
    category: policy
    applies_to: PublishInput
    trigger: --pr を付けたとき
    logic: 書き出したファイルの種類によらず、同じ流れでブランチ、コミット、push、作成を行う
    violation: なし
    source: FR1.2、AC1.1.1
```

## まとめ

| ID | 種類 | 一言で | 出典 |
|---|---|---|---|
| BR1.1 | validation | --pr-* のオプションは --pr と一緒に、書き換えの前に確かめる | FR1.4、X1 |
| BR1.2 | calculation | 既定のブランチ名は UTC の日時と Z | FR1.4、FD2-Q1 |
| BR1.3 | policy | タイトルとコミットのメッセージは --pr-title か既定の文 | FR1.4 |
| BR2.1 | validation | 同じ名前のブランチが手元にもリモートにも無いこと | FR1.5 |
| BR2.2 | policy | リモートは追跡先、無ければ origin | FR1.5 |
| BR2.3 | validation | --write と同じ確かめも行う | FR1.6 |
| BR2.4 | constraint | コミットは書き出したファイルだけ、フックと署名はそのまま | FR1.7、FR1.8 |
| BR3.1 | calculation | 本文は変更したファイルと手で直す一覧を --lang で | FR1.10、X2 |
| BR3.2 | constraint | 理由は固定の文、組み立てる部分は伏せ字 | X2、NFR1 |
| BR3.3 | constraint | 上限を超えたら手で直す一覧を省き、それでも超えたら止める | FD2-Q2 |
| BR4.1 | policy | 書き換えるものが無ければ作らない(0 か 2) | FR1.11 |
| BR4.2 | calculation | 作成のあとの終了コードは 2 か 0 | FR1.13 |
| BR4.3 | policy | 途中で失敗したら、済んだ段階と続きの命令を示す | FR1.12、FD2-Q4 |
| BR4.4 | constraint | 上限で止まってもトークンを出さない | NFR1、NFR3 |
| BR5.1 | policy | CloudFormation と Terraform でも使える | FR1.2 |
