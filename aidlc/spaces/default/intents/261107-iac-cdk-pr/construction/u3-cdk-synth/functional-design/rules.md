# ビジネスルール: U3 CDK の synth と対応づけ(残り)

U3 の決まりです。U1 の決まり(`construction/u1-walking-skeleton/functional-design/rules.md`)のうち、U3 で広げるものは、ここで置き換えます(どれを置き換えるかは各ルールの `replaces` に書きます)。出典は、要件(`inception/requirements-analysis/requirements.md` の FR と NFR)、契約(`inception/contract-design/contract-summary.md`)、設計の決定(`decisions.md` の ADR-004)、このステージの質問の回答(FD3-Q…、`functional-design-questions.md`)です。

```yaml
rules:
  # 1. アプリの発見と選び方
  - id: BR1.1
    statement: --dir の下の CDK のアプリをすべて見つけ、フォルダーの名前の順に並べる
    category: policy
    applies_to: CdkApp
    trigger: iac の実行で、変更案に aws:cloudformation:stack-name のタグのリソースがあるとき
    logic: |
      IF 変更案に stack-name のタグのリソースが1つも無い THEN synth しない(U1 の BR1.1 と同じ、NFR5)。このとき探すのは、--cdk-app が付いていて BR1.2 の確かめに要るときだけで、探しても読むだけで結果には何も足さない。--cdk-app が無ければ探さない。
      ELSE --dir の下で cdk.json のあるフォルダーをすべて探す。
      どちらの場合も、.git、node_modules、cdk.out、.venv、venv、シンボリックリンク、ジャンクションには入らない(.venv と venv は U3 で足す。Python の仮想環境の中のパッケージを辿らず、その中の cdk.json を別のアプリと誤らないため。契約 C2 の Discover の一覧に足す)。深さは今の iac と同じ 32 まで。
      見つかったアプリは --dir からの相対のフォルダー名(/ 区切り)の順に並べる
    violation: なし(見つからなければ今までどおり CloudFormation のテンプレートを探す)
    source: FR2.1、FR2.2、NFR5、契約 C2、契約 C7 synth_rule
    replaces: U1 BR1.1(1つ目のアプリだけを synth し、ほかを外していたところ)

  - id: BR1.2
    statement: --cdk-app の値は、書き換えの前に確かめ、選んだアプリだけを対象にする
    category: validation
    applies_to: CdkAppSelection
    trigger: 変更案を読み、BR1.1 の発見を終えたあと(どのファイルも書き換える前)
    logic: |
      各値を --dir からの相対として読み、区切りを / にそろえ、. と .. を整理する。Windows では \ も区切りとして読み、大文字と小文字を区別せずに比べる。
      IF 値が --dir の外を指す、フォルダーでない、cdk.json が無い、または BR1.1 の発見で見つからない(node_modules や .venv の下など) THEN 入力の誤り。
      確かめは、変更案に stack-name のタグのリソースが無くて synth しないときも行う(誤った値を黙って通さない。そのための発見は BR1.1 のとおり)。
      発見したアプリのうち、選んだもの以外は synth せず、結果のアプリの一覧(CdkAppEntry)にも入れない
    violation: どのファイルも変えずに、誤った値と理由を示して終了コード 1
    source: FR2.1、AC2.1.1、契約 X1 input_errors

  - id: BR1.3
    statement: アプリの言語と CDK の版を決める。Python のアプリの版は Python の依存の宣言から決める
    category: calculation
    applies_to: CdkApp
    trigger: アプリを見つけたとき
    logic: |
      言語は cdk.json の app の命令から決める(U1 の BR1.2 と同じ: node、npx、ts-node、tsx、.ts や .js は typescript、python、python3、py、.py は python、それ以外は other)。
      版は、アプリのフォルダーから --dir までさかのぼり、依存の宣言のファイルが最初に見つかったフォルダーで決める。
      - typescript: package.json の dependencies と devDependencies(U1 と同じ)。aws-cdk-lib があれば 2、@aws-cdk/core があれば 1
      - python: requirements.txt、setup.py、pyproject.toml のどれかがあるフォルダーで止まり、そのフォルダーにある3つのファイルをすべて読む(契約 C2 と同じ)。どれかに aws-cdk-lib があれば 2。無く、どれかに aws-cdk-core があれば 1。依存の名前は、小文字にし、-、_、. の並びを - 1つにそろえてから比べる(aws-cdk.core と aws_cdk.core は aws-cdk-core)。名前の取り出し方は下の表のとおり
      - どれも見つからなければ 0
      Python の依存の名前の取り出し方(ファイルは実行しない。各ファイルは 1 MiB まで読む):
      | ファイル | 名前として読むもの | 読まないもの |
      | requirements.txt | 行ごとに、# から後を除き、前後の空白を除いた行の先頭の名前(英数字と . _ - の並び。[extras]、版の指定、; の環境の指定、@ の URL の前まで) | 空の行、- で始まる行(-r、-c、-e、--index-url など。取り込み先のファイルは読まない)、:// を含む行、. か / で始まる行(パスの指定) |
      | setup.py | 引用符(' か ")で囲まれた文字列のそれぞれの先頭の名前(requirements.txt の行と同じ切り出し) | 引用符の外のもの。式で組み立てた文字列 |
      | pyproject.toml | 引用符で囲まれた文字列のそれぞれの先頭の名前(project.dependencies と optional-dependencies の形)と、見出しに dependencies を含む表([tool.poetry.dependencies] など)の中の、= の左の鍵の名前(poetry の形) | コメント(# から後) |
      setup.py と pyproject.toml は、ほかの文字列(説明の文など)の先頭の語も名前として拾うことがあるが、aws-cdk-lib と aws-cdk-core に一致するものだけを使うので、誤って版を決めることは実際には無い
    violation: other は unsupportedLanguage、版が 1 は cdkV1、0 は unknownVersion で外す(synth しない)。外したアプリのリソースの扱いは BR3.2
    source: FR2.3、FR2.10、AC4.1.4、契約 C2 detection
    replaces: U1 BR1.2(typescript だけを synth していたところ)

  - id: BR1.4
    statement: 選んだアプリを、1つずつ、名前の順に synth する
    category: policy
    applies_to: SynthRun
    trigger: BR1.1 と BR1.2 で対象のアプリが決まったとき
    logic: |
      外すアプリ(BR1.3)には synth の前にまとめて「外した」を知らせる(AppSkipped)。synth するアプリの一覧で「待ち」を1回知らせる(AppsWaiting)。
      synth するアプリを名前の順に1つずつ synth する。同時に2つ以上は動かさない(FD3-Q1)。
      1つのアプリが失敗しても、次のアプリに進む(BR2.7)
    violation: なし
    source: FR2.7、FR2.9、AC2.1.1、AC2.1.2、FD3-Q1

  # 2. synth の実行
  - id: BR2.1
    statement: synth の前に、そのアプリに要る道具があるかを確かめる
    category: validation
    applies_to: CdkApp
    trigger: アプリを synth する直前
    logic: |
      typescript と python の両方で、node と cdk(アプリの node_modules/.bin の cdk、無ければ PATH の cdk。Windows では cdk.cmd)が要る。
      python では、さらに BR2.2 で選ぶ Python が要る(.venv の Python があれば、それで見つかったとする。無ければ PATH に app の命令の最初の語があること)。
      確かめる順は node、cdk、python とし、最初に見つからないものを Tool にする。
      確かめは「実行中」を知らせた(AppStarted)あとに行う(契約 C2 のとおり、synth を始めたアプリは AppStarted のあとに AppDone か AppFailed を送る。道具が無いときの状態は running から failed)
    violation: synth を動かさず、Failure{toolMissing, Tool} にする。表示では、無い道具の名前と入れ方を1行で示す(node は Node.js を入れる、cdk はアプリで npm install するか npm install -g aws-cdk、python は Python 3 を入れる)
    source: FR2.3、FR2.8、AC4.1.3
    replaces: U1 BR1.4 の道具の確かめ(node と cdk だけ)

  - id: BR2.2
    statement: Python のアプリは、アプリのフォルダーの .venv の Python を先に使う
    category: policy
    applies_to: CdkApp
    trigger: python のアプリを synth するとき
    logic: |
      .venv の Python は、Windows では <アプリ>\.venv\Scripts\python.exe、ほかでは <アプリ>/.venv/bin/python(どちらも通常のファイルか、それを指すリンク)。
      IF .venv の Python がある THEN
        - synth に渡す PATH の先頭に、その .venv の実行ファイルのフォルダー(Scripts か bin)を置き、VIRTUAL_ENV にその .venv の絶対パスを入れる。
        - さらに、cdk.json の app の命令の最初の語が python、python3、py(.exe を除いて、大文字と小文字を区別せずに比べる)のどれかなら、その語を .venv の Python の絶対パスに置き換えた命令を、cdk synth の --app で渡す(cdk.json は変えない)。Windows の .venv には python3.exe も py.exe も無いので、置き換えないと cdk init の雛形の python3 app.py が .venv の Python に解決されないため。絶対パスは二重引用符で囲む(cdk は app の命令をシェルで動かし、cmd と sh のどちらでも空白を含むパスが1語になる)。命令の残りはそのまま後ろにつなぐ。
        - 最初の語がそれ以外(.venv/bin/python app.py のように自分でパスを書いたものなど)なら、命令は置き換えない。
      ELSE PATH から app の命令の最初の語を探す。見つからなければ python が無いとする(BR2.1)。命令は置き換えない
    violation: なし(見つからなければ BR2.1 の toolMissing)
    source: FR2.3、FR2.8
    note: 使う Python を選ぶだけで、パッケージを入れたり .venv を作ったりはしない。--debug の表示には、置き換えた命令の形(.venv の Python を使ったこと)を出し、パスの中のホームのフォルダーの名前は出さない
    tests: 3つの OS で、app が python3 app.py の py-minimal に .venv を作ったものが synth できる(Windows で python3 が .venv の Python に置き換わることを含む)。.venv が無く PATH に命令の語も無いときは toolMissing(python)

  - id: BR2.3
    statement: synth には AWS と GitHub の認証情報を渡さず、認証情報を読む設定の経路も止める
    category: policy
    applies_to: SynthRun
    trigger: cdk synth(normal と trace の両方)を動かすとき
    logic: |
      U1 の BR1.3 の環境(AWS_ で始まる変数と GH_TOKEN などを外し、AWS_SHARED_CREDENTIALS_FILE と AWS_CONFIG_FILE を空の一時ファイルに向け、AWS_EC2_METADATA_DISABLED=true、CDK_DISABLE_VERSION_CHECK=1、CDK_DISABLE_CLI_TELEMETRY=true を足す)をそのまま使う。
      これで止まる経路(ドメインの設計のレビューの指摘 R-02 への答え):
      - 環境変数の認証情報、Web ID のトークン、コンテナーの認証情報: AWS_ で始まる変数を外すので止まる
      - 共有の認証情報ファイルと設定ファイル(プロファイル、SSO、credential_process を含む): 空のファイルに向けるので、どのプロファイルも見つからない
      - cdk.json と ~/.cdk.json の profile の指定: 空の設定ファイルでは見つからないので、認証情報は読まれない。Code Generation で確かめたところ、profile の指定だけでは synth は失敗しない(アプリが AWS の値を引くとき、つまり lookup などのときに失敗し、BR2.6 で needsAWS に分ける)
      - EC2 のインスタンスのメタデータ: 止める変数を足す
      tag-refactor は synth に --profile を渡さない
    violation: 決まりに合わない環境では起動しない
    residual_risk: U1 の BR1.3 と同じ(利用者のアプリのコードがホームの下のファイルを自分で読むことは止められない)
    source: FR2.4、NFR2、AC2.1.3、契約 C5、単位の定義の実装の注意(R-02)

  - id: BR2.4
    statement: synth ごとに --synth-timeout の上限を置き、超えたら子のプロセスとその子を止める
    category: constraint
    applies_to: SynthRun
    trigger: cdk synth を動かすとき
    logic: |
      上限は --synth-timeout(既定 10 分、1 秒以上)。normal と trace のそれぞれに、同じ上限を別に置く。
      上限を超えたら、cdk とそれが動かした node や python まで止め、出力先を消す
    violation: "Failure{timeout, Tool: cdk}。trace のときの扱いは BR4.3"
    source: FR2.5、NFR3、AC2.1.4、契約 X1、C5
    replaces: U1 BR1.4 の上限(10 分の決め打ち)

  - id: BR2.5
    statement: synth の出力は一時的なフォルダーに置き、成功、失敗、時間切れ、中断のどれでも消す
    category: constraint
    applies_to: SynthRun
    trigger: cdk synth を動かすとき
    logic: |
      出力先は synth ごとに新しく作る一時的なフォルダー(--output で渡す)で、利用者の cdk.out は使わない。
      synth が終わったら、結果を読んでから消す。時間切れと失敗でも消す。
      Ctrl+C などで中断されたときは、動いている外のプログラム(normal か trace の synth、または Analyze と Recheck の補助のプログラム)を止め(BR2.4 と同じく子まで)、synth の出力先と、環境と補助のプログラムのための一時ファイル(U1 のとおり)を消し、どのファイルも書き換えずに、今の iac の中断と同じ表示と終了コード 1 で終える
    violation: なし
    source: FR2.6、AC2.1.5、NFR3
    replaces: U1 BR1.4 の出力先の決まり(中断のときの扱いを足す)

  - id: BR2.6
    statement: synth の失敗を、決まった文で needsAWS と synthError に分ける
    category: calculation
    applies_to: SynthFailure
    trigger: synth が 0 以外で終わったとき
    logic: |
      標準エラーに次のどれかがあれば needsAWS。無ければ synthError。比べるのは大文字と小文字を区別した部分一致。
      - "Unable to resolve AWS account to use"
      - "Need to perform AWS calls"
      - "Could not load credentials"
      - "Unable to locate credentials"
      - "Unable to determine the default AWS account"
      - "no credentials have been configured"
      - "could not be found or parsed in shared credentials file"
      - "Profile" と "not found" が同じ行にある
      一覧は、テストで1つずつ固定する。Code Generation で、テスト用のアプリ(AWS から値を引くもの、プロファイルを指定したもの)の実際の出力と照らし、合わなければこの一覧を直す
    violation: |
      needsAWS のときは、対処として「一度 AWS の認証情報のある端末で cdk synth を動かし、cdk.context.json に値を保存してから、もう一度実行する」を示す。
      外のプログラムの出力は --debug のときだけ端末に出す
    source: FR2.4、AC2.1.3、契約 C2 needs_aws
    replaces: U1 BR1.5 の既知の文(4つ)

  - id: BR2.7
    statement: 一部のアプリが失敗しても、ほかのアプリと Terraform、CloudFormation の処理を続ける
    category: policy
    applies_to: CdkAppEntry
    trigger: アプリの synth か補助のプログラムが失敗したとき
    logic: |
      失敗したアプリは CdkAppEntry を failed(外したものは skipped)にし、理由と道具を記録する。次のアプリの synth に進む。
      CDK の処理が終わったら、Terraform と CloudFormation の書き換えを今までどおり行う。--pr のときは、書き換えたファイルがあればプルリクエストを作る(U2 の流れ)。
      終了コードは、ほかに 1 にあたるものが無ければ 2(契約 C7 exit_code_rule)
    violation: なし
    source: FR2.9、AC4.1.3、契約 C7 exit_code_rule

  # 3. 対象の決定
  - id: BR3.1
    statement: 変更案のリソースを、成功したすべてのアプリのスタックと照合する
    category: calculation
    applies_to: CdkTarget
    trigger: すべての対象のアプリの synth が終わったとき
    logic: |
      U1 の BR2.1 の照合(スタック名、論理 ID、アカウント、リージョン。空の軸は使わない)を、成功したすべてのアプリのスタックに対して行う。
      IF 候補がちょうど1つ THEN そのアプリの CDK のリソースとする。
      IF 候補が2つ以上(違うアプリ、または同じアプリの違うコンストラクトのパス) THEN 書き換えず cdkAmbiguous で手で直す一覧に出す。
      IF 候補が0 THEN BR3.2
    violation: なし(上の各場合の扱いのとおり)
    source: FR2.11、AC2.1.6、契約 C7 match_rule
    replaces: U1 BR2.1(1つのアプリとだけ照合していたところ)

  - id: BR3.2
    statement: どのアプリのものか分からないリソースは、理由を1つに決めて手で直す一覧に出す
    category: policy
    applies_to: CdkTarget
    trigger: BR3.1 で候補が0のリソースが、CloudFormation のテンプレートの探索でも見つからなかったとき
    logic: |
      ここで数えるのは、synth できなかったアプリ(synth に失敗したものと、外したもの)だけ。synth に成功したあと補助のプログラムの失敗で failed にしたアプリ(BR3.3)は、そのスタックが BR3.1 の照合に加わっているので、候補が0のリソースのアプリではないことが分かっている。数えない。
      IF synth できなかったアプリが1つも無い THEN 今までどおり templateNotFound。
      ELSE 理由を次の優先で1つ決める(FD3-Q3)。
      1. synth に失敗したアプリ(道具が無い、道具が古い、AWS が要る、時間切れ、synth のエラー)がある: cdkAppFailed。synth に失敗したアプリがちょうど1つなら、その道具の名前(node、cdk、python、または python のアプリの app の命令の最初の語の最後の要素(toolName。例 pipenv))を添える。2つ以上なら添えない
      2. 1 に当たらず、cdkV1 か unknownVersion で外したアプリがある: cdkV1
      3. 1 と 2 に当たらず、unsupportedLanguage で外したアプリがある: cdkUnsupportedLanguage
      どのアプリかは項目に書かない
    violation: なし
    source: FR2.8、FR2.9、FR2.10、AC4.1.3、AC4.1.4、契約 C7 failure_to_manual、FD3-Q3
    replaces: U1 BR2.3(失敗したアプリが1つだけの前提だったところ)

  - id: BR3.3
    statement: 補助のプログラムの失敗は、そのアプリだけを failed にして、ほかのアプリを続ける
    category: calculation
    applies_to: CdkAppEntry
    trigger: アプリの補助のプログラム(Analyze と Recheck)が失敗したとき
    logic: |
      U1 の BR1.6 の対応(node が無い、typescript が見つからない、時間切れ、内部の失敗は、そのアプリを failed にし、Tool を node、typescript、helper のどれかにする。読めないファイルはそのファイルだけを外す。結果の版が違えば書き換えの前に終了コード 1)を、アプリごとに当てはめる。
      failed にしたアプリに結び付いたリソースは、すべて cdkAppFailed(道具の名前つき)で手で直す一覧に出し、次のアプリに進む。
      U5 で Python の補助のプログラムができるまでは、python のアプリでは Analyze を呼ばず、結び付いたリソースの変更は、書き換えずに cdkUnsupportedShape で手で直す一覧に出す(アプリの状態は done)。python のアプリでは、作成の記録での補い(BR4.1〜BR4.4)も行わない(Python のソースの読み取りと作成の記録の使い方は U5 で決める)
    violation: なし
    source: 契約 C7 helper_failure_to_manual、FR2.9

  # 4. 作成の記録での補い(ADR-004)
  - id: BR4.1
    statement: 作成の箇所がソースで見つからないリソースがあるアプリだけ、作成の記録を出す設定で1回 synth し直す
    category: policy
    applies_to: SynthRun
    trigger: typescript のアプリの1回目の Analyze が成功したとき(python のアプリは U3 では Analyze をしないので対象にしない。BR3.3)
    logic: |
      そのアプリに結び付いたリソースのうち、コンストラクトのパスに合う作成の箇所(ConstructSite)がソースで見つからないもの(U1 では cdkNoSource になったもの)を集める。
      IF 1つ以上ある THEN そのアプリを cdk synth --debug(Options.WithTrace。CDK の CLI が CDK_DEBUG=true にして、作成の記録をクラウドアセンブリの metadata に残す)で、もう1回だけ synth する。環境、時間の上限、一時的なフォルダーと後片付けは1回目と同じ(BR2.3〜BR2.5)。
      ELSE synth し直さない(ふだんは追加の時間がかからない)
    violation: なし
    source: FR2.11、AC4.1.4、ADR-004、契約 C2 Options.WithTrace

  - id: BR4.2
    statement: 作成の記録から、利用者のソースの最初の箇所だけを TraceSite として読む
    category: calculation
    applies_to: TraceSite
    trigger: trace の synth が成功したとき
    logic: |
      BR4.1 で集めたコンストラクトのパスについて、クラウドアセンブリの metadata の記録(trace)の行を上から見て、最初に次をすべて満たす行のファイルと行番号を使う。
      - ファイルがアプリのフォルダーの下にある
      - node_modules、.venv、venv、site-packages の下でない
      ファイルはアプリのフォルダーからの相対(/ 区切り)にする。満たす行が無いパスは TraceSite を作らない。
      記録の正確な形(metadata のどの欄に、どの書き方で入るか)は、Code Generation でテスト用のアプリの実際の出力と照らして決め、テストで固定する
    violation: なし(TraceSite が無いパスは BR4.4 で cdkNoSource のまま)
    source: ADR-004、契約 C2 TraceSites
    note: U3 では typescript のアプリだけが対象。Python のアプリで作成の記録を使うか(記録に利用者の .py の行が入るか)は U5 で決める

  - id: BR4.3
    statement: trace の synth が失敗しても、アプリの1回目の結果はそのまま使う
    category: policy
    applies_to: SynthRun
    trigger: trace の synth が失敗した(時間切れを含む)とき
    logic: |
      アプリの状態は done のままにし、1回目の synth と Analyze の結果を使う。
      BR4.1 で集めたリソースは cdkNoSource で手で直す一覧に出す。失敗の理由と残した行は --debug に出す。
      中断(Ctrl+C)だけは BR2.5 のとおり全体を止める
    violation: なし
    source: ADR-004、FR2.9

  - id: BR4.4
    statement: trace の synth のスタックが1回目と同じときだけ TraceSite を使い、補助のプログラムで作成の箇所を補う
    category: validation
    applies_to: TraceSite
    trigger: TraceSite を読んだあと
    logic: |
      IF trace の synth のスタック(名前、アカウント、リージョン)と、各リソースの論理 ID とコンストラクトのパスが、1回目とすべて同じ THEN TraceSite を AnalyzeRequest.TraceSites に入れて、そのアプリの Analyze をもう1回行う。補助のプログラムは、ソースをたどって見つけた作成の箇所(From: source)を先に使い、見つからないパスだけ TraceSite の行にある new の式を作成の箇所(From: trace)とする(契約 C3 construct_rules)。
      ELSE(アプリの出力が毎回変わるなど) TraceSite を使わない。
      2回目の Analyze のあとも作成の箇所が見つからないリソースは cdkNoSource で手で直す一覧に出す
    violation: 使わないときは cdkNoSource。違いの内容は --debug に出す
    source: ADR-004、契約 C3 construct_rules、FR2.11、AC4.1.4

  # 5. 進み具合の表示
  - id: BR5.1
    statement: 端末のときは、アプリごとの行を書き直す表で状態と経過時間を示す
    category: policy
    applies_to: AppProgress
    trigger: AppsWaiting から CDK の処理が終わるまで、標準エラーが端末で、カーソルを動かす書き方が使えるとき
    logic: |
      scan の進み具合と同じく、アプリごとに1行(フォルダー、状態、経過時間、失敗と外したときは理由の種類)の表を、その場で書き直す。書き直しは 0.2 秒に1回まで。
      実行中のアプリの経過時間は、知らせが無くても毎秒更新する。
      状態は 待ち、実行中、済み、失敗、外した(英語は waiting、running、done、failed、skipped)。--lang に従う。
      trace の synth のあいだは、そのアプリを再び「実行中」にし、経過時間を新しく数える(BR5.4)。
      Analyze と Recheck のあいだは、そのアプリは「済み」のまま見せる(補助のプログラムは上限 2 分で、状態を増やさない)
    violation: なし
    source: FR2.7、AC2.1.2、契約 C6、FD3-Q2
    replaces: U1 BR1.7(1行ずつ足す表示だけだったところ)

  - id: BR5.2
    statement: 端末でないときは、状態が変わるたびに1行を足し、実行中のアプリがあれば1分ごとにも1行を足す
    category: policy
    applies_to: AppProgress
    trigger: 標準エラーが端末でない、またはカーソルを動かす書き方が使えないとき
    logic: |
      状態が変わるたびに「フォルダー、状態、経過時間(済みと失敗のとき)、理由の種類(失敗と外したとき)」の1行を足す。
      実行中のアプリがあるあいだ、そのアプリの最後の行から1分たつごとに「フォルダー、実行中、経過時間」の1行を足す(FD3-Q2)。
      実行中のアプリが1つも無くなったら、1分ごとの行も止まる(BR5.4)
    violation: なし
    source: FR2.7、AC2.1.2、FD3-Q2
    replaces: U1 BR1.7(端末でないときの工夫を U3 に残していたところ)

  - id: BR5.4
    statement: 時間で動く更新は display.Printer が持ち、契約 C6 の通知口は変えない
    category: policy
    applies_to: AppProgress
    trigger: CDK の進み具合を表示するあいだ
    logic: |
      cdk.ProgressSink(契約 C6 の5つのメソッド)にはメソッドを足さない(足すと実装とテストの差し替えがすべて壊れる、契約の決まり R-04)。
      時間で動く更新(書き直しの表の毎秒の更新、端末でないときの1分ごとの行)は、ProgressSink を実装する display.Printer が持つ。
      - Printer は、実行中のアプリができたとき(AppStarted)に時計を動かし、実行中のアプリが無くなったとき(AppDone、AppFailed で最後の running が終わったとき)に止める。時計は、書き直しの表では1秒ごと、端末でないときは1秒ごとに確かめて、各実行中のアプリの最後の行から1分たったら1行を足す。
      - 時計が描くときも、知らせのときと同じ Printer のロックを取る(今の progress の状態はロックの中でだけ使う決まりのまま)。ほかの出力(結果の表、メッセージ)を書く前に、今と同じく進み具合の表示を閉じ(endProgress)、時計も止める。-race のテストで、時計と知らせとほかの出力が同時に動いても競合しないことを確かめる。
      - CDK の処理の終わりの知らせは足さない。表示は、実行中のアプリが無くなると時計が止まり、次にほかの出力を書くとき(W1 の終わりのあとの結果の表示)に閉じる。
      trace の synth(BR4.1)は同じ Synthesize を使うので、AppStarted を再び送り、終わりに AppDone を送る。trace の synth が失敗しても、アプリの状態は done のまま(BR4.3)なので、IacWriter は trace の synth に、AppFailed を AppDone(同じ経過時間)に読み替えて渡す小さな包みの通知口を渡す。Printer は、済みのアプリに AppStarted が来たら実行中に戻す
    violation: なし
    source: FR2.7、契約 C6、契約の決まり(R-04)、FD3-Q2

  - id: BR5.3
    statement: 進み具合の表示と失敗の表示には、外のプログラムの出力を入れない
    category: constraint
    applies_to: AppProgress
    trigger: 進み具合と失敗を表示するとき
    logic: |
      表示に出すのは、フォルダー、状態、経過時間、理由の種類、無い道具の名前と入れ方、needsAWS の対処だけ。
      SynthFailure.detail(外のプログラムの標準エラー)は --debug のときだけ、伏せ字を通して出す
    violation: なし
    source: NFR1、契約 C2 Failure.Detail、契約 C6

  # 6. テスト
  - id: BR6.1
    statement: Python のテスト用のアプリと、失敗の形のテスト用のアプリを置き、synth のテストを3つの OS で動かす
    category: policy
    applies_to: CdkApp
    trigger: テストを実行するとき
    logic: |
      internal/cdk/testdata/apps/ に、次を足す。
      - py-minimal: Python の最小の CDK v2 のアプリ。requirements.txt で aws-cdk-lib などの版を固定し、Dependabot(pip)で更新する。CDK の CLI は ts-minimal と同じく npm で版を固定したものを使う
      - 失敗の形のアプリ: CDK v1(@aws-cdk/core と aws-cdk.core)、版が分からないもの、other の言語(app が mvn など)、AWS から値を引くもの(環境を決めたスタックで lookup を使う)。synth しない形のものには依存を入れない
      CI は、テストの前に py-minimal の .venv を作って requirements.txt を入れる(BR2.2 の .venv を使う形を、そのまま確かめる)。
      Python、Node.js、CDK の CLI、テスト用のアプリの依存のどれかが無いとき、CI ではテストを失敗にし、手元では理由を出して飛ばす(U1 の BR5.1 と同じ)。
      時間切れと中断は、応答しない差し替えの cdk と、実際のアプリに短い --synth-timeout を付けたもので確かめる。後片付けは、一時的なフォルダーの置き場所をテストのフォルダーに向けて、実行のあとに何も残らないことと、アプリの cdk.out が変わらないことで確かめる
    violation: CI で道具が無ければテストの失敗
    source: NFR8、AC2.1.4、AC2.1.5、進め方 §Testing Posture
```

## まとめ

| ID | 種類 | 一言で | 出典 |
|---|---|---|---|
| BR1.1 | policy | `--dir` の下のアプリをすべて見つけ、名前の順に並べる | FR2.1、FR2.2 |
| BR1.2 | validation | `--cdk-app` を書き換えの前に確かめ、選んだアプリだけを対象にする | FR2.1、X1 |
| BR1.3 | calculation | 言語と版を決める。Python の版は requirements.txt などから | FR2.3、FR2.10 |
| BR1.4 | policy | 1つずつ、名前の順に synth する | FR2.7、FD3-Q1 |
| BR2.1 | validation | 要る道具(node、cdk、python)を確かめ、無ければ名前と入れ方を示す | FR2.8 |
| BR2.2 | policy | Python はアプリの `.venv` を先に使い、app の命令の python を `--app` で置き換える | FR2.3 |
| BR2.3 | policy | 認証情報と、それを読む設定の経路を止める | FR2.4、NFR2 |
| BR2.4 | constraint | synth ごとに `--synth-timeout`、超えたら子まで止める | FR2.5、NFR3 |
| BR2.5 | constraint | 出力は一時的なフォルダー。中断でも消し、書き換えずに終える | FR2.6 |
| BR2.6 | calculation | 決まった文で needsAWS を見分ける。一覧を増やす | FR2.4 |
| BR2.7 | policy | 一部が失敗してもほかを続け、終了コード 2 | FR2.9 |
| BR3.1 | calculation | 成功したすべてのアプリと照合する。2つ以上は cdkAmbiguous | FR2.11 |
| BR3.2 | policy | どのアプリか分からないリソースの理由を優先で1つに決める | FR2.8〜FR2.10、FD3-Q3 |
| BR3.3 | calculation | 補助のプログラムの失敗はそのアプリだけ failed。Python のアプリは U5 まで手で直す一覧へ | 契約 C7 |
| BR4.1 | policy | 作成の箇所が見つからないときだけ、`--debug` で1回 synth し直す | ADR-004 |
| BR4.2 | calculation | 作成の記録から利用者のソースの最初の行を読む | ADR-004 |
| BR4.3 | policy | 2回目が失敗しても1回目の結果を使う | ADR-004 |
| BR4.4 | validation | スタックが1回目と同じときだけ記録を使い、作成の箇所を補う | ADR-004、契約 C3 |
| BR5.1 | policy | 端末では書き直しの表、経過時間を毎秒 | FR2.7 |
| BR5.2 | policy | 端末でないときは変化ごとと1分ごとに1行 | FR2.7、FD3-Q2 |
| BR5.3 | constraint | 表示に外のプログラムの出力を入れない | NFR1 |
| BR5.4 | policy | 時計は display.Printer が持ち、通知口にメソッドを足さない。trace の synth は包みの通知口で失敗を済みに読み替える | FR2.7、契約 C6 |
| BR6.1 | policy | Python と失敗の形のテスト用のアプリ、3つの OS のテスト | NFR8 |

## U3 が答える契約の未決事項

| 未決事項(契約) | U3 の答え |
|---|---|
| C2: 作成の記録を出す設定(`WithTrace`)の名前と出力の形 | `cdk synth --debug`。CDK の CLI の説明に「作成の記録を残し、CDK_DEBUG を設定する」とある(テスト用のアプリの node_modules の aws-cdk で確かめた)。記録はクラウドアセンブリの metadata に入る。どの欄にどの形で入るかは、Code Generation でテスト用のアプリの実際の出力と照らして決める(BR4.2) |
| C2: needsAWS の既知の文の一覧 | BR2.6 の8つ。Code Generation でテスト用のアプリの実際の出力と照らす |
| 単位の定義の実装の注意: 認証情報の遮断の、設定の読み込み経路(R-02) | BR2.3。U1 の環境のままで、共有の認証情報ファイルと設定ファイルを空のファイルに向けることで、プロファイル、SSO、credential_process、cdk.json の profile の経路も止まる |
