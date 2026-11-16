# セキュリティの要件: U3 CDK の synth と対応づけ(残り)

U1 のセキュリティの要件(`construction/u1-walking-skeleton/nfr-requirements/security-requirements.md` の NFR1.1〜NFR1.5、NFR2.1〜NFR2.4)と、U2 で足した NFR1.6〜NFR1.10 は、U3 にもそのまま当てます。この文書には、U3 で新しく決めた要件と、U1 の要件の当て方を書きます。元になる要件は `inception/requirements-analysis/requirements.md` の NFR1〜NFR10、契約は `inception/contract-design/contract-summary.md` の X1、C2、C5、C6、C7 です。決まりは `construction/u3-cdk-synth/functional-design/rules.md` の BR1.1〜BR6.1 です。出典の NQ3-Q… は、このステージの `nfr-requirements-questions.md` の回答です。

U3 で新しく守るものは3つです。Python のアプリの synth に渡す環境、作成の記録(`cdk synth --debug`)に入る利用者の端末のパス、利用者が `--cdk-app` で渡すパスです。

このステージで作る文書は、この文書と `tech-stack-decisions.md` だけです(U3 の種類では、性能、拡張性、信頼性、観測のしやすさの文書を作りません)。そのため、セキュリティ以外の区分で U3 が足す要件も、下の「ほかの区分の要件」の表にまとめて書きます。

## 要件

| ID | 要件 | 合否の基準 | 出典 |
|---|---|---|---|
| NFR1.11 | 作成の記録(`cdk synth --debug` の metadata)と、そこから読んだ利用者の端末の絶対パスは、プルリクエストのタイトル、本文、コミットのメッセージ、ふつうの表示に出さない。TraceSite のファイルはアプリのフォルダーからの相対のパスだけを持ち、アプリのフォルダーの外、node_modules、.venv、venv、site-packages の下のパスは捨てる | テストで、作成の記録にホームのフォルダーの絶対パスを入れた差し替えの出力を読ませると、本文と表示にそのパスが出ず、TraceSite が相対のパスになる。Unix の `/home/<名前>/...` と Windows の `C:\Users\<名前>\...` の両方の形で確かめる。アプリの外、node_modules、.venv、venv、site-packages の下のフレームが使われない | NFR1、BR4.2、ADR-004 |
| NFR1.12 | `--debug` に出す synth の命令は、BR2.2 で `.venv` の Python に置き換えたときも、`.venv` の Python を使ったことが分かる形にし、パスの中のホームのフォルダーの部分は `~` に置き換える。synth と補助のプログラムと版の確かめの標準エラーは、U1 の NFR1.4 のとおり認証情報の形を伏せて `--debug` のときだけ出す | `--debug` のテストで、置き換えた命令の行にホームのフォルダーの名前が無く、`~` と `.venv` がある。Python の差し替えの標準エラーに仕込んだトークンの形が伏せ字になる | NFR1、BR2.2、BR2.6 |
| NFR1.13 | プルリクエストの本文と結果の表示に出すアプリの状態は、アプリのフォルダー(`--dir` からの相対)、言語、状態、失敗の理由の種類と道具の名前だけにする。synth の標準エラー(SynthFailure.detail)は出さない | テストで、synth の差し替えの標準エラーに ARN と 12 桁の数字とトークンの形を入れても、本文と結果の表示に出ない | NFR1、契約 C7、BR2.6 |
| NFR2.5 | Python のアプリの synth(normal と trace)と、Python と Node.js の版の確かめにも、U1 の NFR2.1 の環境(AWS_ で始まる変数と GitHub のトークンの変数を外し、空の設定ファイルに向け、メタデータを止める)をそのまま渡す。U3 が足すのは、`.venv` があるときの PATH の先頭と VIRTUAL_ENV だけにする。PATH は、親の環境の PATH の変数(Windows では名前の大文字と小文字を区別せずに探した1つ。`Path` など)の値の先頭に `.venv` の実行ファイルのフォルダーを足し、同じ名前の変数を1つだけ渡す。VIRTUAL_ENV は、親の環境にあっても外して、`.venv` の絶対パスに置き換える(U1 の環境の作り方の、足す変数を親から外してから足す形と同じ) | 差し替えた実行の口のテストで、Python のアプリの synth と版の確かめの環境に外した変数が無く、足した値と PATH の先頭と VIRTUAL_ENV がある。Windows で親の環境が `Path` のとき、渡す環境に PATH の名前の変数が1つだけで、その先頭が `.venv\Scripts`。親に別の VIRTUAL_ENV があっても、`.venv` のあるアプリでは `.venv` の絶対パスになり、`.venv` の無いアプリでは親の値のまま。trace の synth の環境も normal と同じ | NFR2、契約 C5、BR2.2、BR2.3 |
| NFR2.6 | `.venv` の Python は、アプリのフォルダーの直下の `.venv` のものだけを使う(Windows は `.venv\Scripts\python.exe`、ほかは `.venv/bin/python`)。親のフォルダーや PATH の `.venv` は探さない。`.venv` を作ったりパッケージを入れたりしない | テストで、親のフォルダーにだけ `.venv` があるときは PATH の Python を使う。実行のあとに `.venv` の中身が変わらない | NFR2、BR2.2 |
| NFR2.7 | `--cdk-app` の値は、`.` と `..` を整理したあとで `--dir` の外を指すもの、発見で見つからないもの(.git、node_modules、cdk.out、.venv、venv、リンクの下)を入力の誤りにし、どのファイルも書き換えず、synth も起動しない | 受け入れテストで、`../other`、`a/../../other`、Windows の `..\other`、`node_modules/x`、`.venv/x`、リンクを経由するパス、絶対パス(`/etc`、`C:\x`、ドライブだけの `C:`)が終了コード 1 になり、synth の差し替えが呼ばれない。絶対パスは、BR1.2 のとおり --dir からの相対として読むので、発見したアプリの一覧に一致せず入力の誤りになる。Windows では `Infra/TS` が `infra/ts` のアプリに一致する | NFR2、契約 X1、BR1.2 |

## ほかの区分の要件

| ID | 区分 | 要件 | 合否の基準 | 出典 |
|---|---|---|---|---|
| NFR3.11 | 性能 | synth の上限は `--synth-timeout`(既定 10 分、最小 1 秒)で、1回の synth ごと(normal と trace のそれぞれ)に置く。U1 の NFR3.1 の決め打ちの 10 分を置き換える | 終わらない差し替えの `cdk` に `--synth-timeout 1s` を付けると、約1秒で止まり、子のプロセスが残らない。`0s`、`500ms`、`abc` は入力の誤り | NFR3、契約 X1、BR2.4 |
| NFR3.12 | 性能 | 全体の時間の上限は置かない。README の `iac` の節に、最も長い時間の見積もり方を書く。アプリ1つあたり、Python の版の確かめ 30 秒(NFR3.14)+ synth の上限 × 2(normal と trace)+(Node.js の版の確かめ 30 秒 + 補助のプログラムの上限 2 分)× 3(analyze 2回と recheck 1回、NFR3.17。今の Analyzer は補助のプログラムの呼び出しごとに `node --version` を動かす)で、これに synth するアプリの数を掛ける。既定の上限では、アプリ1つあたり 28 分 | README の確かめ(合否は文書の確かめ)。式に、版の確かめ、synth の2回、補助のプログラムの3回が入っている | NQ3-Q2、BR1.4、BR4.1、BR4.4 |
| NFR3.17 | 性能 | 補助のプログラムは、アプリごとに analyze を1回、作成の箇所が見つからないリソースがあり trace の synth のスタックが1回目と同じときだけもう1回(合計2回まで)、書き換えがあれば recheck を1回だけ起動する(ファイルごとに起動しない)。Python のアプリでは U5 まで起動しない。U1 の NFR3.5 を置き換える | テストで起動の回数を数える。trace を使わないアプリで analyze 1回、使うアプリで analyze 2回、trace の synth が失敗したかスタックが違うアプリで analyze 1回、Python のアプリで 0回 | NFR3、契約 C3、BR3.3、BR4.1、BR4.4 |
| NFR3.13 | 性能 | trace の synth は、作成の箇所が見つからないリソースがある TypeScript のアプリだけで、アプリごとに1回だけ動かす | テストで、作成の箇所がすべて見つかるアプリでは trace の synth を呼ばず、見つからないアプリで1回だけ呼ぶ。Python のアプリでは呼ばない | BR4.1、BR3.3 |
| NFR3.14 | 性能 | Python と Node.js の版の確かめは、1回の呼び出しごとに 30 秒の上限を置く(U1 の読むだけの `git` と同じ) | 応答しない差し替えの `python` で、上限のあとに止まり、そのアプリが失敗になる | NFR3、NQ3-Q1 |
| NFR3.15 | 性能 | 発見は .git、node_modules、cdk.out、.venv、venv、リンクの下に入らず、深さ 32 までにする。変更案に stack-name のタグのリソースが無く `--cdk-app` も無いときは発見もしない | テストで、フォルダーとファイルを入れた `.venv` と `venv` のあるアプリで、`.venv` の下を読まない(読んだフォルダーを数える)。stack-name のリソースが無いときに発見の関数が呼ばれない | BR1.1、NFR5 |
| NFR3.16 | 信頼性 | U1 の NFR3.10(子のプロセスごと止める)を、Python のアプリの synth(cdk が動かす python とその子)、trace の synth、版の確かめにも当てる | 子を作って待ち続ける差し替えの `python` で、時間切れと中断のあとに子が残らない(3つの OS) | NFR3、BR2.4 |
| NFR4.5 | 信頼性 | synth の出力先(normal と trace のそれぞれ)、認証情報の遮断の一時ファイル、補助のプログラムの一時ファイルは、成功、失敗、時間切れ、中断のどれでも消す。中断が Analyze と Recheck の途中に来たときも同じ。アプリの `cdk.out` は変えない | 中断を Analyze の途中に入れるテストを含め、実行のあとに一時的なフォルダーが残らず、アプリの `cdk.out` が変わらない | NFR4、BR2.5 |
| NFR5.3 | 互換性 | 変更案に stack-name のタグのリソースが無いときは、`--cdk-app` があっても synth せず、出力と終了コードを今の `iac` と同じにする(`--cdk-app` の確かめで誤りがあれば終了コード 1)。`--cdk-app` も `--synth-timeout` も付けないときの、アプリが1つの TypeScript のリポジトリの結果は、U1 と同じ | 前の intent の `iac` の受け入れ条件のテストと、U1 の受け入れ条件のテストがそのまま通る。ただし U1 の NFR3.8(アプリが2つ以上あるとき最初の1つだけを synth し、残りを外す)のテストと、U1 の NFR3.5(analyze は1回)の回数のテストは、そのまま通す対象から除き、U3 の複数のアプリのテストと NFR3.17 のテストで置き換える | NFR5、BR1.1 |
| NFR8.3 | テスト | Python のテスト用のアプリ `py-minimal` の synth を、CI の3つの OS で動かす。CI はテストの前に `py-minimal` に `.venv` を作り、`requirements.txt` を入れる(BR2.2 の `.venv` の形と、Windows での `python3` の置き換えをそのまま確かめる)。Python、`.venv`、Node.js、CDK の CLI、依存のどれかが無いとき、CI では失敗、手元では理由を出して飛ばす | 3つの OS の CI で、`py-minimal` の synth のテストが通る。CI の定義で、道具が無いと失敗になることを確かめる | NFR8、BR6.1 |
| NFR8.4 | テスト | 失敗の形のテスト用のアプリ(CDK v1 の TypeScript と Python、版が分からないもの、other の言語、AWS から値を引くもの)を置く。AWS から値を引くものは、認証情報を遮断した synth で needsAWS になることを確かめ、実際の AWS には届かない(CI から AWS を呼ばない) | CI で needsAWS のテストが通り、テストに AWS の認証情報を渡していない(CI の定義の確かめ) | NFR8、BR2.6、BR6.1 |
| NFR10.4 | 観測 | 進み具合の表示は、端末でカーソルを動かす書き方が使えれば書き直しの表(経過時間を毎秒更新)、そうでなければ変化ごとと実行中のアプリの1分ごとに1行を足す。状態は文字で示し、色だけに頼らない。U1 の NFR10.1 を置き換える | テストで、端末の差し替えでは表が書き直され、端末でないときは1分ごとの行が出る(時計を差し替える)。-race のテストで、時計の更新とほかの出力が競合しない | NFR10、FR2.7、BR5.1、BR5.2、BR5.4 |
| NFR10.5 | 言語 | U3 で足す表示の文(アプリの状態、Python が無い、Python の版が古い、`--cdk-app` と `--synth-timeout` の誤り、needsAWS の対処、どのアプリか分からないリソースの理由)は、英語と日本語の両方を用意し、`--lang` で選ぶ | `--lang ja` と `--lang en` のテストで、それぞれの文が出る | NFR10 |

## U1 の要件の当て方

| 元の要件 | U3 での当て方 |
|---|---|
| NFR1 | U1 の NFR1.1〜NFR1.5、U2 の NFR1.6〜NFR1.10 のまま。NFR1.11〜NFR1.13 を足す |
| NFR2 | U1 の NFR2.1〜NFR2.4 を、Python のアプリと trace の synth と版の確かめに広げる(NFR2.5)。NFR2.6、NFR2.7 を足す |
| NFR3 | U1 の NFR3.1 を NFR3.11 で、NFR3.5 を NFR3.17 で置き換え、NFR3.12〜NFR3.16 を足す。U1 の NFR3.8(アプリは1つ)は、U3 で複数のアプリを扱うので役目を終え、そのテストは U3 の複数のアプリのテストで置き換える(NFR5.3)。NFR3.2〜NFR3.4、NFR3.6、NFR3.7、NFR3.9、NFR3.10 はそのまま |
| NFR4 | U1 の NFR4.1〜NFR4.4 のまま。NFR4.3 を Python と trace の synth と補助のプログラムの中断に広げる(NFR4.5) |
| NFR5 | U1 の NFR5.1、NFR5.2 のまま。NFR5.3 を足す |
| NFR6 | U1 の NFR6.1〜NFR6.10 と U2 の NFR6.11〜NFR6.15 のまま。NFR6.16〜NFR6.21(`tech-stack-decisions.md`)を足す |
| NFR7 | 足したコードもカバレッジ 80% に入れる。`python` を呼ぶ部分も差し替えてテストする。U1 の NFR7.1 のまま |
| NFR8 | U1 の NFR8.1、NFR8.2 に NFR8.3、NFR8.4 を足す |
| NFR9 | U3 はプルリクエストの作り方を変えない。U1 の NFR9.1 と U2 の NFR9.2 のまま |
| NFR10 | U1 の NFR10.1 を NFR10.4 で置き換え、NFR10.5 を足す。NFR10.2、NFR10.3 はそのまま |

## 残るリスク

- `cdk synth` は利用者のアプリのコードを動かします。Python のアプリでは、`.venv` の中のパッケージ(利用者が入れたもの)も動きます。これは、利用者が自分で `cdk synth` を動かすときと同じ信頼の範囲です(U1 の残るリスクと同じ)。知らないリポジトリで `iac` を動かすことは、そのリポジトリのコードを動かすことだと README に書きます。
- 利用者のアプリのコードが、ホームの下の AWS の設定を決め打ちのパスで自分で読むことは止められません(U1 と同じ)。
- needsAWS の文の一覧(BR2.6)は、CDK と AWS の SDK の版で変わることがあります。合わない文が出たときは synthError になり、`--debug` で標準エラーを見れば分かります。一覧はテストで固定し、Code Generation で実際の出力と照らします。
- Python の版の確かめ(NQ3-Q1)は、最小の版より古いことだけを見ます。Python の新しすぎる版で `aws-cdk-lib` が動かないことは見つけられず、synthError になります。

## 脅威の見方(STRIDE の考え方で、関係するものだけ)

| 脅威 | 例 | 手当て |
|---|---|---|
| 情報の漏えい | Python のアプリの synth に AWS の認証情報が渡り、利用者のアプリが boto3 で AWS を呼ぶ | NFR2.5 |
| 情報の漏えい | 作成の記録の絶対パス(利用者の名前を含む)がプルリクエストの本文に入る | NFR1.11 |
| 情報の漏えい | synth の標準エラーのアカウント ID や ARN が、本文のアプリの状態に入る | NFR1.13 |
| 改ざん | `--cdk-app ../../other` で `--dir` の外のアプリの synth を動かす | NFR2.7 |
| 改ざん | 親のフォルダーに置かれた `.venv` の Python が使われる | NFR2.6 |
| サービスの妨害 | 大きな `.venv` を辿って発見が終わらない | NFR3.15 |
