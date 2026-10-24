## Review

**Verdict:** NOT-READY
**Reviewer:** aidlc-architecture-reviewer-agent
**Date:** 2026-10-24T06:23:41Z
**Iteration:** 1

### Prior findings

| Severity | Location | Finding | Status |
|---|---|---|---|
| なし | なし | 1回目のレビューのため、前回の指摘はありません | なし |

### New findings

| Severity | Location | Finding | Required action |
|---|---|---|---|
| high | `construction/u4-plan/nfr-requirements/tech-stack-decisions.md` > 色の行(「Windows の端末は U2 で仮想端末の処理を有効にしている」)、および `performance-requirements.md` > NFR11.7 | 技術の選択の根拠が事実と違います。U2 の `enableVT`(`internal/display/terminal_windows.go`)は `canRedraw(stderr)` からしか呼ばれず(`display.go:55`)、標準エラーが端末で、かつ進み具合を書き換える場合にだけ働き、終了時に元へ戻します。U4 の結果は標準出力に出す(BR3.10)ので、標準出力の仮想端末の処理は誰も有効にしません。Windows の従来のコンソールや PowerShell 5.1 では、色の制御文字(`ESC[32m` など)がそのまま見える出力になります。NFR11.7 は `NO_COLOR` と「端末でない」だけを決めており、Windows で仮想端末の処理を有効にできたときだけ色を付けること、`TERM=dumb`(U2 の `canRedraw` は尊重している)のときに色を出さないこと、標準出力と標準エラーを別々に判定すること(BR3.7)を、要件にも合否の基準にも含めていません。 | 色を出す条件を要件にします。「そのストリームが端末で、`NO_COLOR` が空でなく、`TERM` が `dumb` でなく、Windows ではそのストリームの仮想端末の処理を有効にできたときだけ」。標準出力と標準エラーを別々に判定します。技術の選択の「U2 で有効にしている」を、「標準出力についても U4 で有効にし、終了時に元へ戻す」に直します。合否の基準に、判定関数を差し替えた Windows の分岐(有効にできない場合は色なし)と `TERM=dumb` のテストを加えます。 |
| medium | `construction/u4-plan/nfr-requirements/security-requirements.md` > NFR8.15、`tech-stack-decisions.md` > JSON の行 | NFR8.15 は JSON についても「端末を書き換える文字をそのまま出さない」と書いていますが、技術の選択は `encoding/json` の標準の動きのままとしています。標準の `encoding/json` が必ず変えるのは、0x20 未満、`"`、`\`、`<`、`>`、`&`、U+2028、U+2029、不正な UTF-8 です。DEL(0x7f)、C1 制御文字(U+0080〜U+009F。U+009B は 8 ビット端末の CSI)、U+202E などの双方向の制御文字は、そのまま出ます。合否の基準の入力に U+202E を挙げていますが、JSON で確かめるのは「正しい JSON として読めて元に戻る」ことだけなので、この矛盾は検出されません。また、エスケープの形を書いた箇所が「`<`、`>`、`&` も `<` などにする」となっていて、変換前と変換後が同じ文字で、意味が通りません(`<` と書くべき箇所と思われます)。 | JSON の方針を決めます。(a) JSON は機械が読むものとして、標準の動きを認め、要件の文言を「制御文字(0x20 未満)は `\u00XX`、`<`、`>`、`&` は `<`、`>`、`&`」に絞り、双方向の制御文字などが残ることを明記する。(b) 端末に出す場合を守るなら、DEL・C1・Cf を `\uXXXX` に変える後処理を足す。どちらでも、`<` の誤記を直し、合否の基準に U+202E、DEL、U+009B を含めた JSON の期待のバイト列を加えます。 |
| medium | `construction/u4-plan/nfr-requirements/security-requirements.md` > NFR8.16 | 次の2点が決まっていません。(1) 範囲: 「表のセルの中のコードの書式」としか書かれておらず、表の外に出るデータ由来の文字列(機能設計の形にある `## 対象` の行、`<summary>`、「消す候補」などの一覧、失敗したリージョンの理由、影響の finding のポリシー名やステートメント ID の文、scan の Markdown)の扱いが要件にありません。失敗したリージョンの理由には AWS の応答の文字列が入りえます。(2) 値の忠実さ: コードの書式(コードスパン)の中では文字の参照は解釈されないため、`a<b` が `a&lt;b` と表示され、コピーしたタグの値が元と変わります。コードスパンの中の `<` は CommonMark では安全なので、`<`、`>`、`&` の参照への変換は表示を壊しつつ、守る効果は(コードスパンが崩れない限り)ありません。合否の基準も、期待の形を「決めたエスケープの形にだけなっている」と述べるだけで、利用者が見る文字との対応を確かめません。 | 範囲を「Markdown の出力に出るデータ由来の文字列すべて」と書き、表の外の文字列は同じ関数を通すことを合否の基準に加えます(例: 失敗したリージョンの理由に `</details>` を含めるテスト)。参照への変換を残すなら、コードスパンの中で表示がどうなるかを許容する理由とともに書き、残さないなら、コードスパンの区切り(NFR8.16 の `` ` `` の数の規則)と `|`、改行だけに絞ります。 |
| medium | `construction/u4-plan/nfr-requirements/security-requirements.md` > NFR8.18、`nfr-requirements-questions.md` > Q2、`performance-requirements.md` | 読み込みの資源の上限が検証できません。(1) 256 MiB の根拠(「1万リソース・1リソース50個のタグがすべて最大の長さでも収まる」)は計算されていません。ASCII でも約 192 MiB(10,000 × 50 × 384 文字)が `beforeTags` だけで、変更後のタグや JSON の記号を足すと収まらない場合があります。一方、性能の条件(変更の対象5,000件、1リソース20個)の変更案は数 MiB です。上限が実際の大きさの数十倍で、U3 のルールファイルの上限(128 KiB)と比べても根拠が薄いままです。(2) 256 MiB を `encoding/json` で型に写すとメモリは数倍(数 GB の可能性)になりますが、メモリの要件も、読み込みの時間の要件(NFR1.14 は保存の時間だけ)もありません。(3) ファジングの「1秒を超える入力が見つからない」は、ファジングの道具が時間を失敗として扱わないため、そのままでは確かめられません。 | 上限を、実際の最大の変更案の大きさの計算(根拠つき)から決め直すか、256 MiB のままなら理由と、そのときのメモリの見積もりを書きます。1万リソース・変更5,000件の変更案の読み込みの時間(例: 3秒以内)とメモリの上限を、合否の基準つきで NFR1 系に加えます。ファジングの対象に「1秒を超えたら失敗」とする検査を含めると明記します。 |
| low | `construction/u4-plan/nfr-requirements/performance-requirements.md` > NFR9.6・NFR9.7、`nfr-requirements-questions.md` > 決まっていること(「リダイレクトしたファイルは UTF-8(BOM なし、LF)」) | Windows PowerShell 5.1 の `>` は UTF-16(BOM 付き)で書きます。ツールの出力は UTF-8 でも、`plan --format json > plan-view.json` のファイルがそうなることをツールは防げません。U3 は NFR9.5 でこの点を扱いましたが、U4 では扱っていません。また、保存した変更案を手で直した(BOM 付きで保存された)ファイルを読むときの動きが BR4.4 にも NFR8.18 にもなく、`encoding/json` は BOM があると「不正な文字」の誤りになります。 | 「リダイレクトしたファイルは UTF-8」を「ツールの出力は UTF-8。PowerShell 5.1 の `>` は変えてしまう」に直し、利用者への案内に `--out` を使うことを書きます。変更案の読み込みは、UTF-8 の BOM を読み飛ばすか、BOM や UTF-16 を「UTF-8 で保存し直す」案内つきの InvalidInput にするかを決め、種の入力に加えます。 |
| low | `construction/u4-plan/nfr-requirements/security-requirements.md` > NFR8.19、`inception/contract-design/contract-summary.md` の「U4 で足した項目」 | 「契約 C2・C3 の項目だけを書く」と書いていますが、BR3.4・BR4.2 は C2・C3 に `keyConflicts`、`planId`、`createdAt`、`regions`、`conflictResources`、`unsetRequiredResources` を足しています(契約の「U4 で足した項目」)。要件の文言と設計が合っていません。また、保存する「ルールファイルの場所」(絶対パスなら利用者名を含む)の扱いがありません。合否の基準の確かめは、環境変数の偽の認証情報が標準出力とファイルにないことだけです。 | 「契約(U4 で足した項目を含む)に書いた項目だけ」に直します。ルールファイルの場所は、入力のまま書くか、相対パスにするかを決めて書きます。標準エラーに出す警告とエラーにも偽の認証情報が含まれないことを、合否の基準に加えるか、含めない理由を書きます。 |
| low | `construction/u4-plan/nfr-requirements/security-requirements.md` > NFR8.17、`traceability.json` > NFR6 の N/A | NFR6(途中で壊れない保存)を N/A とし、変更案の保存の壊れにくさは NFR8.17 で扱うとしていますが、NFR8.17 の合否の基準は、権限、上書きしないこと、一時ファイルが残らないことだけです。保存先の名前に途中のファイルが現れないこと(書き込み中に止めても、保存先が「完全な変更案か、何もないか」のどちらかであること)が基準にありません。Windows では 0600 に相当する確かめがなく(`os.CreateTemp` は Unix の権限を使わない)、ハードリンクが使えない場合の「確かめてから名前を変える」代替の競合(確認と名前の変更の間に別のファイルができる)も扱っていません。 | 合否の基準に、書き込み中の中断のあとで保存先に部分的なファイルが残らないテストを加えます。Windows の権限は、既定の ACL に任せて確かめない旨を明記します。代替の名前変更の競合は、U3 と同じ限界として受け入れると書きます。 |
| low | `construction/u4-plan/nfr-requirements/performance-requirements.md` > NFR9.7、`tech-stack-decisions.md` > 言語の決め方 | 「3つの OS で同じ規則」は正確ではありません。Windows だけ、環境変数が3つとも空のときに `GetUserPreferredUILanguages` を使います(関数が `golang.org/x/sys/windows` にあることは確認しました)。合否の基準は、読む関数を差し替えるテストだけで、Windows の CI でも実際の API の呼び出しが確かめられません。その呼び出しの失敗時や空のときの動き(英語に戻す)も書かれていません。 | 「同じ規則(Windows だけ最後に OS の表示言語を見る)」に直します。Windows の CI で、実際の関数を呼んで失敗しない(空か有効な言語名を返す)簡単なテストを加え、失敗時は英語にすると書きます。 |
| low | `construction/u4-plan/nfr-requirements/performance-requirements.md` > NFR1.13・NFR1.14 | NFR1.13 の2秒に、U3 の NFR1.12(ルールの照会だけで1万件に2秒以内)と同じ上限があり、照会が上限を使い切る場合、変更案の作成の残りの余裕がありません。ピークのメモリと、すべての文字列に置き換えが必要な最悪の入力での出力の時間が条件に含まれていません。CI の実時間の上限は共有の実行環境で揺れます。 | 作成の上限は、NFR1.12 の測り方との関係(含める、含めない)を書いて決めます。置き換えが必要な値を半数含む入力も、NFR1.14 の条件に加えます。 |

### Validation Tool Results

| Tool | Result | Interpretation |
|---|---|---|
| `aidlc engine sensor-traceability --output-path .../u4-plan/nfr-requirements/traceability.json --stage-slug nfr-requirements` | pass: true、gaps・orphans・invalid_entries はすべて空、findings_count 0 | traceability.json の形は有効です。N/A の理由(NFR2、NFR4、NFR5、NFR7、NFR12)は妥当と判断しました。NFR6 の N/A は、上の NFR8.17 の指摘のとおり、根拠が部分的です。 |
| `go doc golang.org/x/sys/windows.GetUserPreferredUILanguages` | 関数が存在する | 「新しい依存は足さない」「`go.mod` の変更はない」の主張は正しいです(`x/sys` は直接の依存で、`terminal_windows.go` が `x/sys/windows` を既に使っています)。 |
| `internal/display/terminal_windows.go`、`display.go:55` の確認 | `enableVT` は標準エラーに対してだけ、`canRedraw` の中で呼ばれる | 色に関する技術の主張(U2 が有効にしている)は標準出力に当てはまらない。high の指摘の根拠です。 |

### Summary

セキュリティ、性能、移植性の要件はいずれも合否の基準が具体的で、U1〜U3 の番号の続きも正しく、新しい依存を足さないという主張も確認できました。ただし、色の出力に関する技術の根拠が事実と違い、Windows の標準出力で色の制御文字がそのまま見える設計になっているため、NOT-READY とします。そのほか、JSON の端末向けの安全性の主張と標準の動きの矛盾、Markdown の範囲と忠実さ、保存した変更案の読み込みの資源の上限の3点を要件として固めてから、次に進んでください。

### Findings

| ID | Severity | Location | Finding | Required action | Status |
|---|---|---|---|---|---|
| - | - | - | No findings | No action required | Resolved |
