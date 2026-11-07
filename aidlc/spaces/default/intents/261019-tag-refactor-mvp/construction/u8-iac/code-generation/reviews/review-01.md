## Review

**Verdict:** READY
**Reviewer:** aidlc-architecture-reviewer-agent
**Iteration:** 1

### Findings

| ID | Severity | Location | Finding | Required action | Status |
|---|---|---|---|---|---|
| R-01 | Major | internal/iac/depth.go > hclTooDeep、hclScan.codeStep、hclScan.template | Terraform のテンプレート指示(`%{if true}` の入れ子)が深さの事前検査をすり抜ける。検査は `%{` で深さを足し、すぐ次の `}` で戻すので、指示を何重に入れても深さが 0 のままになる。実際に一時フォルダーで `locals { a = "%{if true}%{if true}...x" }` を含む `.tf` を `iac` に渡すと、約 0.5 MB で 2.3 秒、1 MB で 9.4 秒、2 MB で 40.5 秒と、ファイルの大きさの 2 乗で時間が延びた(4 MB は 2 分を超えても終わらず、手で止めた)。1 ファイルの上限 10 MB(maxFileBytes)では 10 分を超える見込みで、NFR3.19(f) の深さの守りと NFR5.25 の時間の見込み(`.tf` 80 MB で約 14 秒)に反する。ファイルは信用しない前提の入力なので、手元のリポジトリに置かれた 1 ファイルで `iac` が実質止まる。 | 検査に、`%{if}`、`%{for}` の開きと `%{endif}`、`%{endfor}` の閉じの入れ子の深さ(または指示の総数)を数えて上限を設ける。上の入力を受け入れ条件に近い形のテストにして、`SkipTooDeep` になることを確かめる。10 MB 近い入力の時間もテストか計測で示す。 | New |
| R-02 | Minor | internal/fileout/replace.go > writeTemp、code-summary.md の計画と設計からのずれ | 一時ファイルの読み直しを同じハンドルで行う変更は、読み直しの目的を弱める。同期のあとに同じハンドルで読むと、開き直しと同じくキャッシュ上の内容を読むだけで、短い書き込みの検出(Write のエラーと長さの比較)以上のことは確かめない。変更の理由の「ウイルス対策が原因と見ている」は計測で裏づけられていない。実行時間と開く回数は減るが、設計 §7 の「読み直し」が守ろうとした確かめの強さが下がることが、ずれの表に書かれていない。 | ずれの表に「確かめられる範囲が狭まる」ことと、原因が推測であることを書く。可能なら、開き直しが遅い原因を計測で確かめる。 | New |
| R-03 | Minor | internal/iac/run.go > checkBeforeWriting、internal/fileout/replace.go > Replace | 書く直前の確かめは Lstat の権限と内容のハッシュだけで、通常のファイルのままかを見ない。確かめと名前の付け替えの間にシンボリックリンクに差し替えられると、リンクそのものが新しいファイルに置き換わる。また一時ファイル経由の置き換えで、所有者、ACL、ハードリンクは引き継がれない。 | 直前の確かめで通常のファイルであることも見る。所有者などが引き継がれないことを README か設計に書く。 | New |

### 確かめたこと(壊せなかったもの)

- YAML、JSON、HCL の書き換えで、CRLF、BOM、日本語のコメントと値、行末コメント、最後の改行なし、フロー形、要素の順序違い、JSON の続けて消す要素、すべて消した場合(`Tags: []`)を一時フォルダーで試し、範囲の外のバイトは変わらなかった。ブロックスカラー、1 行にまとめた配列の削除は手で直す一覧に回り、ファイルは変わらなかった。
- 値の引用符では、YAML の型が変わる値 40 種ほど(true、0x1F、null、~、2020-01-01、1e3、0777、1:30、yes、NO、空文字など)がすべて二重引用符で書かれた。JSON は U+2028、`<&>`、引用符、絵文字が正しく出た。HCL は `${` と `%{` と `$${` と `\` を、構文の上で同じ文字列に戻るように書いた。
- git の確かめ: 未コミットの変更があるファイルは、`--write` で書かずに止まった。リポジトリの設定に `core.fsmonitor` のコマンドを置いても、`iac` の git 呼び出しでは実行されなかった。`GIT_` の環境変数は取り除かれ、`-c core.fsmonitor=false` と `GIT_LITERAL_PATHSPECS=1` が付く。使うのは rev-parse、ls-files、ls-tree だけで、status は使わない。
- 対象の決め方: Terraform は state の ARN の完全一致と 1 つのアドレス、モジュールと添字つきは手で直す。CloudFormation は論理 ID と Type の一致、複数なら stack 名とファイル名で絞り、決まらなければ手で直す。

### 確かめられなかったこと

- 時間と深さの限界のうち、YAML と JSON の事前検査の抜け穴は探していない。上の HCL の指摘と同じ形の入れ子(YAML のフロー、JSON)も確かめる価値がある。
- `ci.yml` と `verify.sh` が `cicd-pipeline.md` のとおりかは、ci.yml の `iac` の手順(終了コード 0 または 2 を許し、差分が 4 行であることを見る)を読んだだけで、全体は照らしていない。`traceability.json` の各対象が主張を確かめているか、NFR8.27(秘密が差分以外に出ない)も、全体は確かめていない。

### Validation Tool Results

| Tool | Result | Interpretation |
|---|---|---|
| go test(iac、restype、fileout、journal、display、cli、rules) | すべて ok(多くはキャッシュ) | 既存のテストは通る。ただし深さの検査の抜け穴は、テストが見ていない |
| 一時フォルダーでの `iac` の実行(YAML、JSON、HCL、git の確かめ) | 上のとおり、書き換えは正しい。HCL の入れ子の指示だけが時間の問題 | Major 1 件 |

### Summary

書き換え、引用符、git の確かめは、試した入力では壊せなかった。一方、Terraform のテンプレート指示の入れ子が深さの検査を通り抜け、ファイルの大きさの 2 乗の時間がかかる(Major)。Critical はなく Major は 1 件なので、判定は READY とするが、この検査の穴は直してから先へ進めることを勧める。
