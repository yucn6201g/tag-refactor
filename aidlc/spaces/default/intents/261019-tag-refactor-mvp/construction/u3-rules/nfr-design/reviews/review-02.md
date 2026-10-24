## Review

**Verdict:** READY
**Reviewer:** aidlc-architecture-reviewer-agent
**Date:** 2026-10-24T00:19:22Z
**Iteration:** 2

### Findings

| ID | Severity | Location | Finding | Required action | Status |
|---|---|---|---|---|---|
| R-01 | medium | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u3-rules/nfr-design/logical-components.md > 依存の向き(`rules` は `errkind` だけ、種類の照会は関数の値として受け取る)と `internal/inventory` の行 | 機能設計 BR3.4 は、`when[].match.resourceType` を読み込みのときに検査し、U2 の種類の対応表を使って直し方(例 `write "ec2:instance"`)を出すと定めている。しかし受け取る口として示されているのは `SuggestInput` の2つの関数だけで、種類の照会を `rules` の読み込みに渡す手段がない。現在の `rules.Load(path)` と `rules.Parse(path, data)` に引数はなく、`rules` は `inventory` を import できない。そのため BR3.4 を実装する人は、グローバル変数か `rules` 側への表の複製のどちらかを推測で選ぶことになる。ファジングの対象 `rules.Parse` の呼び出しの形にも影響する | 種類の照会を `rules` の読み込みに渡す方法を決めて書く(例: `Load` と `Parse` に省略できるオプションを足し、nil なら「条件に書けない」の案内にする)。`cli` が scan と plan の両方で `inventory` の関数を渡すこと、ファジングは nil で動かすことも書く | Resolved (reviewer) |
| R-02 | medium | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u3-rules/nfr-design/security-design.md > §4 文字コードの見分け・大きさの上限、ファジングの対象 | 文字コードの見分けと、変換後 64 KiB の判定を、現在の `Load`(`readLimited`)と `Parse` のどちらが受け持つかが書かれていない。現在の `Parse` は大きさを一切判定せず、64 KiB の判定は `readLimited` にある。一方ファジングの対象は「`rules.Parse`(バイト列からの読み込みのすべて)」で、種の入力に UTF-16 も含めている。見分けを `Load` に置くと `Parse` のファジングは UTF-16 の経路に届かない。`Parse` に置くと、その中で大きさを判定しない限り、ファジングが渡す 64 KiB を超える入力で「1秒以内」を保証できない。また、1回の呼び出しが1秒以内という検査を、3つの OS の CI の `-race` つきの普通のテストとして動かすと、遅い実行環境でたまたま失敗する(U2 では `race` のビルドタグで件数を減らしている) | 見分け、変換、変換後の大きさの判定を置く関数を1つに決め、ファジングの対象と一致させる。ファジングが渡す入力の大きさの上限(128 KiB 超は `t.Skip` する、または `Parse` の中で判定する)を書く。1秒の検査は、`-race` のときに余裕を持たせるか、`-race` では回数だけを見る形にする | Resolved (reviewer) |
| R-03 | medium | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u3-rules/nfr-design/security-design.md > §3 下書きの書き出しの守り(日本語などをそのまま出す行)、§4 文字コード(PowerShell の保存の案内) | 下書きは標準出力に UTF-8 で出し、日本語などの印字できる文字は書き換えずにそのまま出す。ところが案内している `rules suggest > tagrules.yaml` と `rules suggest \| Out-File -Encoding utf8 tagrules.yaml` では、PowerShell(5.1 も 7 も)が子プロセスの出力を `[Console]::OutputEncoding`(Windows の既定は OEM コードページで、日本語環境では 932)で読み取ってから書き出す。ツールはコードページを変えない(`internal/display` に設定はない)。このため、日本語のタグの値は保存の時点で文字化けし、読み込みは通るが元の値と違う下書きになる。NFR9.5 は UTF-16 の BOM を読めることだけを確かめており、この文字化けを防げない。C5 には下書きの出力先を指定するオプションがなく、リダイレクトが唯一の保存の方法になっている | 日本語などの値が保存で壊れない方法を決めて書く。例: 非 ASCII をすべて `uXXXX` で書く、または README の保存の例に UTF-8 の出力設定(`[Console]::OutputEncoding`)を含める。その方法を確かめるテストか手元の確認(日本語の値を含む下書きを PowerShell で保存して読み戻す)を足す | Resolved (reviewer) |
| R-04 | low | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u3-rules/nfr-design/security-design.md > §1 rules suggest は読み取りの口だけで動く | 「ConfigReader、ExplorerReader、PolicyReader は渡さない。渡す口がないので呼ぶ経路がない」とあるが、現在の `connect` と `collect` は5つの読み取りの口をすべて持つ `readers` を作って返す。`rules suggest` が `collect` を再利用すると、`readers` の中に config、explorer、policies が存在する。実際の防ぎとなるのは、記録された操作を確かめるテストだけである | 組み立ての方法を書く。例: account と tags だけを持つ型を新しく作る、または `connect` を二段階にする。それをしない場合は、「口は存在するが使わず、テストで防ぐ」と正直に書く | Resolved (reviewer) |
| R-05 | low | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u3-rules/nfr-design/logical-components.md > 性能の設計(64 KiB への調整) | 値の候補を外したときの引き算は「外したキーの値の部分のバイト数」だけである。しかし BR6.7 は、外したキーに「values omitted: the draft would exceed 64 KiB」のコメントを足すと定めており、不完全なときの先頭のコメントも加わる。引き算だけでは合計が実際より小さく見積もられ、64 KiB を超えた下書きが「収まった」と判断される | 引き算に足されるコメントのバイト数を含める。最後に組み立てた下書きの実測が上限以下であることを確かめる(超えていれば続ける)ことを、設計とテストに書く | Resolved (reviewer) |
| R-06 | low | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u3-rules/nfr-design/security-design.md > §4 文字コード(BOM のない UTF-16 の見分け) | 見分けは「最初の2バイトのどちらかが 0x00」だけで、最初の文字が非 ASCII(例: 日本語のコメント)の BOM のない UTF-16 は見分けられず、「UTF-8 として読み、不正な UTF-8 はその行の誤り」に落ちて、NFR9.5 が求める行番号1と直し方の案内にならない。奇数バイトの UTF-16 の扱いも書かれていない | 見分けの限界(ASCII で始まる場合だけ見分けられる)を README と設計に書くか、UTF-8 として不正なときに偶数長で 0x00 を多く含む場合は同じ案内にする。奇数バイトの扱い(末尾の1バイトは U+FFFD にする、など)を決める | Resolved (reviewer) |
| R-07 | low | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u3-rules/nfr-design/security-design.md > §4 文字コード(BOM のない UTF-16 の行) | BOM のない UTF-16 の見分けに「UTF-8 として不正で」が条件として入っている。ASCII だけの UTF-16(`a0b0` の並び)は UTF-8 として正しく、NUL 文字になるだけである。そのため、この行は ASCII で始まる UTF-16 では働かず、「UTF-8 で保存し直す」の案内が出ない。 | 見分けの条件から「UTF-8 として不正」を外し、「偶数長で 0x00 が 4 分の 1 以上」だけにする。ファジングの種に ASCII の BOM なし UTF-16 を足す。 | New |

> R-01 Reviewer note: security-design.md §4 で `Load(path, opts)`、`Parse(path, data, opts)`、`Options{TypeHint}` を定義した。nil のときは「条件に書けない」の案内になる。cli が scan と plan で渡すこと、ファジングは `Options{}` で動かすことも書かれている。`rules` は `inventory` を import せずに済む。

> R-02 Reviewer note: 文字コードの見分け、変換、64 KiB の判定をすべて `Parse` の先頭に置いた。`Load` はファイルの種類の確認と 128 KiB + 1 バイトまでの読み取りだけを行う。ファジングの対象と一致している。`-race` では 1 秒の上限を `race` のビルドタグで 10 倍にする。

> R-03 Reviewer note: `rules suggest --output <ファイル>` を足した(C5 への追加)。UTF-8(BOM なし、LF)で一時ファイルに書いてから名前を変え、既にあるファイルは上書きしない。日本語と補助面の文字を読み戻す CI のテストと、手元の PowerShell での確認も書かれている。

> R-04 Reviewer note: 接続の関数を、アカウントとタグの取得だけを持つ型を返す段と、残りの口を足す段に分けた。`rules suggest` は一段目だけを使い、偽の AWS で操作が 3 種類に限られることをテストで確かめる。

> R-05 Reviewer note: 64 KiB への調整で、先頭のコメントと `values omitted` コメントのバイト数を合計に入れた。調整のあとに組み立てて実測し、超えていれば調整を続ける。

> R-06 Reviewer note: 見分けの限界を書いた。奇数バイトの最後の 1 バイトと、対になっていないサロゲートは U+FFFD にする。BOM のない UTF-16 は「UTF-8 で保存し直す」の直し方を出す。

### Validation Tool Results

| Tool | Result | Interpretation |
|---|---|---|
| sensor-traceability | pass: true(gaps、orphans、invalid はすべて 0) | 追跡の不備はない。 |

### Summary

前回の 6 件はすべて直っている。新しい指摘は軽微な 1 件だけで、ブロックしない。
