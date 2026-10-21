# 操作の仕様: tag-refactor

コマンドラインのツールなので、「コンポーネント」はサブコマンドと、共通の出力部品です。`.claude/knowledge/aidlc-design-agent/component-spec-template.md` の形を、端末向けに読み替えて使っています(画面の幅による変化は「端末の幅」として扱います)。

画面の番号は `mockups.md` を指します。

---

## scan

| Field | Value |
|---|---|
| Component | `scan` サブコマンド |
| Description | 1アカウントの全リージョンのリソースとタグを集め、表記ゆれと必須タグの欠落を件数つきで示す |
| Category | display |
| Stories | US1.1、US1.2、US6.1 |

### States

| State | Description | Trigger |
|---|---|---|
| running | リージョンごとの進み具合を表示(画面1) | 実行開始 |
| success | 結果を表示(画面2)、終了コード 0 または 2 | すべてのリージョンを読めた |
| partial | 「結果が不完全」を先頭に表示、終了コード 1 | 一部のリージョンを読めなかった |
| untagged-unavailable | 「探せなかった」と表示、終了コードは他の結果で決まる | AWS Config も Resource Explorer も使えない |
| empty | 「問題なし」と件数を表示、終了コード 0 | 問題が見つからない |
| error | 何が起きたかと直し方を表示、終了コード 1 | 認証情報なし、ルールファイルの誤り |

### Inputs

| Input | Type | Required | Default | Description |
|---|---|---|---|---|
| `--rules` | パス | いいえ | なし | ルールファイル。ないと必須タグの欠落は調べない |
| `--profile` | 文字列 | いいえ | AWS の標準の設定 | 使う AWS のプロファイル |
| `--format` | `table` / `json` / `markdown` | いいえ | `table` | 結果の形式 |
| `--lang` | `en` / `ja` | いいえ | OS の言語設定 | 表示の言語 |

### 端末の幅

| 幅 | 振る舞い |
|---|---|
| 狭い(80桁未満) | 列を減らさず、長い名前は折り返す(切り詰めない) |
| 通常 | 画面2のとおり |

### Accessibility

| Requirement | Implementation |
|---|---|
| 色に頼らない | 状態は `[done]` などの文字、種類は記号で示す |
| 読み上げ | 罫線を使わず、1行に1つの事実を書く |
| キーボード | すべての操作はコマンドとオプションで行う |
| 出力の分離 | 結果は標準出力、進み具合と案内は標準エラー |

---

## rules suggest

| Field | Value |
|---|---|
| Component | `rules suggest` サブコマンド |
| Description | 既存のタグから正しいキー名・値の候補をまとめ、ルールファイル(YAML)の下書きを標準出力に出す |
| Category | input |
| Stories | US2.1 |

### States

| State | Description | Trigger |
|---|---|---|
| running | scan と同じ進み具合 | 実行開始 |
| success | 下書きを標準出力に、案内を標準エラーに出す(画面3) | 候補が見つかった |
| empty | 「まとめる候補はない」と案内し、空の雛形を出す | 表記ゆれがない |
| error | 終了コード 1 | 認証情報なしなど |

### まとめ方の規則(RM-Q2)

1. 大文字・小文字の違いをまとめる(`env` と `Env`)
2. 区切り文字(`-`、`_`、スペース)の違いをまとめる(`cost-center` と `cost_center` と `CostCenter`)
3. よく使われる略語の辞書でまとめる(`env` → `environment`、`prod` → `production` など)
4. まとめた結果はあくまで候補で、正しい名前は「最も多く使われている書き方」を初期値にする(案)。最終的には利用者がルールファイルで決める

---

## plan

| Field | Value |
|---|---|
| Component | `plan` サブコマンド |
| Description | ルールファイルに沿った変更案と、その影響を示す。変更案をファイルに保存できる |
| Category | display |
| Stories | US3.1、US3.2 |

### States

| State | Description | Trigger |
|---|---|---|
| success | 変更案と影響を表示(画面4〜6)、終了コード 0 または 2 | 変更案を作れた |
| no-changes | 「変更はない」と表示、終了コード 0 | ルールどおりになっている |
| partial | 「結果が不完全」または「影響分析が不完全」を先頭に表示、終了コード 1 | リージョンやポリシーを読めなかった |
| error | 終了コード 1 | ルールファイルの誤りなど |

### Inputs

| Input | Type | Required | Default | Description |
|---|---|---|---|---|
| `--rules` | パス | はい | — | ルールファイル |
| `--format` | `table` / `diff` / `json` / `markdown` | いいえ | `table` | 表示の形式 |
| `--out` | パス | いいえ | なし | apply 用に変更案を保存する |

### 表示の順番

1. 対象(アカウント、リージョン数、作成日時)
2. 変更(種類ごとの件数。重なりは総数で1件)
3. 値が未指定の必須タグ
4. 消す候補(`aws:` で始まるキーは除く)
5. IaC で管理されているリソースの件数
6. 影響(`!`)と判定できない参照(`?`)。ポリシー名、ステートメント、条件、原因の変更
7. 要約と終了コード、次の手順

---

## apply

| Field | Value |
|---|---|
| Component | `apply` サブコマンド |
| Description | 変更案に沿ってタグを書き換える。書き換える前の状態を記録する |
| Category | input |
| Stories | US4.1、US4.2 |

### States

| State | Description | Trigger |
|---|---|---|
| confirming | 確認画面を表示し、`yes` の入力を待つ(画面7) | 実行開始 |
| cancelled | 何も書き換えずに終了、終了コード 0 | `yes` 以外を入力 |
| blocked-account | 何も書き換えずに終了、終了コード 1 | 保存した変更案とアカウントが違う(RM-Q7) |
| blocked-impact | 何も書き換えずに終了、終了コード 2 | `--yes` だけで、影響のある変更がある(FR5.5) |
| recording | 書き換える前の状態を記録する | `yes` の入力後 |
| record-failed | 何も書き換えずに終了、終了コード 1 | 記録を保存できない |
| applying | 進み具合を表示(画面8) | 記録の保存後 |
| done | 結果と取り消しのコマンドを表示。失敗ありは 1、飛ばしのみは 2、なしは 0 | 終了 |

### Inputs

| Input | Type | Required | Default | Description |
|---|---|---|---|---|
| `--plan` | パス | いいえ | なし | 保存した変更案。ない場合は `--rules` から作る |
| `--rules` | パス | `--plan` がないときは必須 | — | ルールファイル |
| `--yes` | フラグ | いいえ | なし | 確認を省く |
| `--allow-impact` | フラグ | いいえ | なし | `--yes` のときに、影響のある変更も進める |
| `--record-to` | パスまたは `s3://` | いいえ | `.tag-refactor/runs/`(仮) | 書き換える前の状態の保存先 |

### 確認画面の内容(RM-Q4)

- 対象のアカウントID(と、今の認証情報のアカウントID)、リージョンの数、変更案の作成日時
- 変更の種類ごとの件数
- 影響のある変更(`!`)と判定できない参照(`?`)の一覧
- IaC で管理されているリソースの件数
- 書き換える前の状態の保存先

---

## rollback

| Field | Value |
|---|---|
| Component | `rollback` サブコマンド |
| Description | apply の実行IDを指定して、記録した状態に戻す |
| Category | input |
| Stories | US5.1 |

### States

| State | Description | Trigger |
|---|---|---|
| confirming | 戻す件数と、後から変えられたリソースの件数を表示し、`yes` を待つ(画面9) | 実行開始 |
| cancelled | 何も戻さずに終了、終了コード 0 | `yes` 以外を入力 |
| not-found | 記録が見つからないと表示、終了コード 1 | 実行IDの記録がない |
| blocked-account | 何も戻さずに終了、終了コード 1 | 記録のアカウントと今のアカウントが違う |
| done | 戻した・飛ばした・失敗した件数を表示。失敗ありは 1、飛ばしのみは 2、なしは 0 | 終了 |

### Inputs

| Input | Type | Required | Default | Description |
|---|---|---|---|---|
| `--run` | 実行ID | はい | — | 戻す apply の実行ID |
| `--force` | フラグ | いいえ | なし | 後から変えられたリソースも戻す |
| `--record-from` | パスまたは `s3://` | いいえ | `.tag-refactor/runs/`(仮) | 記録の読み込み元 |
| `--yes` | フラグ | いいえ | なし | 確認を省く |

---

## 共通の出力部品

| 部品 | 内容 | 使う画面 |
|---|---|---|
| 対象の見出し | アカウントID、リージョン数(、作成日時) | 2、4、7、9 |
| 進み具合 | 端末では書き換え表示、端末でないときは1行ずつ追記 | 1、8 |
| 不完全の警告 | `!` と「Results are incomplete」を結果の先頭に出す | 2、4 |
| 要約 | 件数と終了コードの意味を1行で | すべて |
| 次の手順 | 次に実行するとよいコマンドを1行で | 2、3、4、8 |
| エラー | 1行目に何が起きたか、2行目に直し方 | すべて |

## Assumptions & Open Questions

- オプション名と、記録の既定の保存先は仮のものである。[assumption]
- 表記ゆれのグループの「正しい名前」の初期値を、最も多く使われている書き方にするのは案である。[assumption]
- 端末の幅が狭いときに切り詰めず折り返すのは案である。[assumption]
