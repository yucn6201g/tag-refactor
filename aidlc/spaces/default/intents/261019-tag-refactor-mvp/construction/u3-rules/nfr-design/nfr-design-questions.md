# 非機能の設計 — 質問(U3 ルール)

U3 の非機能要件(`construction/u3-rules/nfr-requirements/`)を、どう作るかを決めます。U1 と U2 の設計は、ここで変えない限りそのまま使います。

決まっていること(質問しません):

- 新しい依存は足さない。UTF-16 の変換は Go の標準(`unicode/utf16`)で行う。
- 下書きは YAML のライブラリを使わず、ツールが決まった形の文字列を組み立てる(`tech-stack-decisions.md`)。
- ファジングは Go の標準(`testing.F`)。CI では種の入力だけ、手元で5分走らせて結果をチェックポイントに残す。
- `rules suggest` の収集は U2 の `inventory` の収集をそのまま使い、一度もタグが付いていないリソースは探さない。

## Q1. rules suggest の置き場所

部品の設計(`inception/domain-design/components.md`)では、表記ゆれの候補は RuleEngine(`internal/rules`)が受け持ち、RuleEngine はほかの部品に依存しません。一方、キーのまとめ方(正規化と略語の辞書)は U2 で `internal/inventory` に作りました。下書きを作る処理を、どこに置きますか?

A. `internal/rules` に置く。入力は「リソースの ARN とタグ」の単純な一覧にし、キーと値をまとめる関数(`inventory` の正規化と略語の辞書)は、組み立てる側の `cli` から渡す。`rules` は `inventory` を import しない(部品の依存の向きを保つ)
B. 新しいパッケージ `internal/suggest` を作り、`inventory` と `rules` の両方を使う
C. `internal/inventory` に置く(まとめ方と同じ場所)
X. Other (please specify)

[Answer]: A

## Consolidated Summary Confirmation

回答のまとめ:

- 置き場所(Q1): 下書きを作る処理は `internal/rules` に置きます。入力は「リソースの ARN とタグ」の単純な一覧で、キーと値をまとめる関数(`inventory` の正規化と略語の辞書)は、組み立てる側の `cli` から渡します。`rules` は `inventory` を import せず、部品の依存の向きを保ちます。

Does this all look correct before I generate the artifact?

- Looks correct
- Request changes

[Answer]: Looks correct
