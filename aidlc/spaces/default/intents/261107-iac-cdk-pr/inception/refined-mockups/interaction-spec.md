# 操作の仕様: iac への CDK の反映とプルリクエストの作成

画面は `mockups.md`、要件は `requirements.md`、ストーリーは `stories.md` です。コマンドラインのツールなので、部品の仕様の型(`component-spec-template.md`)を、表示のまとまりごとに当てはめます。画面の幅や触れる操作の項目は、端末の出力には当てはまらないので「当てはまらない」と書きます。

## 進み具合の表示(synth)

| 項目 | 内容 |
|---|---|
| 部品 | synth の進み具合 |
| 説明 | CDK のアプリごとの `cdk synth` の状態と経過時間を示す(FR2.7) |
| 種類 | 状態の表示(feedback) |

### 状態

| 状態 | 説明 | きっかけ |
|---|---|---|
| waiting | まだ始めていない | 対象のアプリが決まったとき |
| running | `cdk synth` が動いている | 実行を始めたとき |
| done | 対応が取れた | synth が成功したとき |
| failed | 失敗した(道具が無い、AWS が要る、時間切れ、v1、synth の誤り) | 失敗したとき。理由を同じ行の後ろか次の行に出す |
| empty | CDK のアプリが無い | `cdk.json` が見つからないとき。表示は出さない(今の `iac` と同じ出力になる、NFR5) |

### 入力

| 入力 | 型 | 必須 | 既定 | 説明 |
|---|---|---|---|---|
| `--cdk-app` | フォルダー(繰り返し可) | いいえ | すべての見つかったアプリ | 対象の絞り込み |
| `--synth-timeout` | 時間(`10m`、`90s` など) | いいえ | `10m` | アプリごとの上限 |

### 画面の幅による変化

当てはまらない(端末の出力)。端末かどうかで、行を書き換えるか追記するかが変わる(`mockups.md` 画面1)。

### アクセシビリティ

| 要件 | 実装 |
|---|---|
| 状態の伝え方 | 文字の `[done]` などで示し、色は補助 |
| 読み上げ | 1行に1つのアプリ。端末でないときは追記だけで、行の書き換えはしない |
| 入力の受け付け | なし |

## 書き換えの前の確かめ(`--pr`)

| 項目 | 内容 |
|---|---|
| 部品 | プルリクエストの前の確かめ |
| 説明 | ファイルを書き換える前に、プルリクエストを作れるかを確かめる(FR1.5、FR1.6) |
| 種類 | 入力の確かめ |

### 確かめる順番

1. `--pr-branch`、`--pr-title`、`--pr-body-file` が `--pr` と一緒に使われているか(なければ入力の誤り)
2. `--dir` が git の作業ツリーで、書き換えるファイルがコミット済みで変更がないか(今の `--write` と同じ)
3. HEAD がブランチを指しているか
4. push するリモート: 今のブランチの追跡先、無ければ `origin`。どちらも無ければ止める(US-Q4)
5. そのリモートが GitHub か
6. `gh` があり、そのホストにログインしているか
7. 行き先(base)にする今のブランチが、そのリモートにあるか
8. 作るブランチの名前が、手元にもリモートにもないか

最初に満たされなかった項目で止め、1文目に原因、2文目に対処、最後に「No file was changed.」を出す(`mockups.md` 画面5)。

### 状態

| 状態 | 説明 |
|---|---|
| 通った | 1行の「Checked: …」を出して書き換えに進む |
| 止まった | 理由と対処を出し、終了コード 1 |

## プルリクエストの作成の進み具合

| 段階 | 成功のときの行 | 失敗のときの「Done so far」 | 続きの案内 |
|---|---|---|---|
| 書き換え | `Wrote N files (...)` | (今の `--write` の失敗の表示) | 今の `--write` と同じ |
| ブランチの作成 | `Created branch <名前>` | `N files written.` | `git switch -c <名前>` から |
| コミット | `Committed N files` | `N files written, branch created.` | `git add <ファイル>` と `git commit` から |
| push | `Pushed <リモート> <名前>` | `N files written, branch created and committed.` | `git push -u <リモート> <名前>` から |
| 作成 | `Opened pull request` と URL | `N files written, branch created and committed, branch pushed to <リモート>.` | `gh pr create --base <base> --head <名前>` |

- どの段階で失敗しても、済んだことは元に戻さず「Nothing was undone.」を出す(FR1.12)。
- コミットと push は、利用者の `git` の設定(フック、署名)のまま行う(FR1.7)。
- コミットに入れるのは、書き換えたファイルだけ(FR1.8)。
- 外のプログラムの呼び出しには、すべて時間の上限がある(NFR3)。上限で止まったときは、その段階の失敗として扱う。

## 手で直す一覧の理由(CDK に足すもの)

| 理由 | 英語の文(例) | 出典 |
|---|---|---|
| 値が式 | `the value is an expression (<式の頭の部分>)` | FR3.4 |
| スコープ全体のタグ | `the tag is set on the whole stack, and the plan does not change every resource in it the same way` | RF-Q3、FR3.5 |
| ほかのリソースと共有 | `the same value is also used by other resources (<名前>)` | FR3.5 |
| アプリの synth の失敗 | `the CDK app could not be synthesized (<理由>)` | FR2.8、FR2.9 |
| 対応が取れない | `the CDK app does not say which source made this resource` | FR2.11 |
| CDK v1 | `this is a CDK v1 app; only CDK v2 is supported` | FR2.10 |
| 書き方が対応していない | `this way of setting tags is not supported yet` | FR3.4 |

- 場所は `ファイル:行`。synth の失敗など、場所が決まらないときはアプリのフォルダー(`mockups.md` 画面8)。
- 日本語の文は、`messages` に英語と並べて置く(NFR10)。

## 判断の決まり

| 場面 | 決まり | 出典 |
|---|---|---|
| スタックやコンストラクトの全体に付けたタグ | 変更案が、そのスコープでタグを受けるすべてのリソースを同じように変えるときだけ書き換える。そうでなければ一覧に出す | RF-Q3 |
| 書き換えるものが無い | プルリクエストを作らない。手で直すものが無ければ 0、あれば 2 | FR1.11、US-Q5 |
| 一部のアプリの失敗 | ほかは続け、`--pr` でもプルリクエストを作る。終了コード 2 | FR2.9 |
