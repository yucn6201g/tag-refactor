# セキュリティの要件: U2 収集

U1 のセキュリティの要件(`construction/u1-walking-skeleton/nfr-requirements/security-requirements.md` の NFR3.1、NFR3.2、NFR8.1〜NFR8.8)は、U2 でもそのまま守ります。ここでは、U2 で新しく呼ぶ AWS の操作と、新しく表示する文字列についての要件を足します。

出典: 要件 NFR3、NFR8、機能設計 `construction/u2-scan/functional-design/`(BR3.1、BR3.2、BR4.1、BR4.3、BR7.1)、U1 の NFR の設計 `security-design.md` §1〜§4、`nfr-requirements-questions.md`、AWS のドキュメント。

## U2 で新しく呼ぶ操作(読み取りだけ)

API の操作の名前と、それを呼ぶのに必要な IAM の操作の名前は、同じとは限りません。例のポリシーには IAM の操作の名前を書きます。

| サービス | 呼ぶ API の操作 | 必要な IAM の操作 | 目的 |
|---|---|---|---|
| AWS Config | `DescribeConfigurationRecorders` | `config:DescribeConfigurationRecorders` | 記録する種類(すべてか一部か)を知る |
| AWS Config | `DescribeConfigurationRecorderStatus` | `config:DescribeConfigurationRecorderStatus` | 記録しているかを知る |
| AWS Config | `SelectResourceConfig` | `config:SelectResourceConfig` | 利用者のタグがないリソースを問い合わせる |
| Resource Explorer | `ListIndexes` | `resource-explorer-2:ListIndexes` | 索引があるか、集約の索引かを知る |
| Resource Explorer | `GetDefaultView` | `resource-explorer-2:GetDefaultView` | 一覧に使う既定のビューがあるかを知る |
| Resource Explorer | `GetView` | `resource-explorer-2:GetView` | 既定のビューに絞り込みの条件があるかを知る(NFR の設計で足した) |
| Resource Explorer | `ListResources` | `resource-explorer-2:Search`(AWS のドキュメント `API_ListResources.html` の最小の権限) | `tag:none` のリソースを件数の上限なく読む |

- `ListResources` の IAM の操作は、AWS のドキュメントで `resource-explorer-2:Search` と確かめた。ほかの操作の IAM の名前は、NFR の設計で Service Authorization Reference と照らして確かめ、違っていればこの表と例のポリシーを直す。
- API の `Search`(1,000件まで)、記録や索引を作る・変える操作は呼ばない。IAM の `resource-explorer-2:Search` は `ListResources` のためだけに許す。

## 要件

| ID | 要件 | 合否の基準 |
|---|---|---|
| NFR3.3 | 上の表の IAM の操作を、読み取り用の IAM ポリシーの例(`docs/iam/readonly-policy.json`)と、確認用のテンプレートの読み取り用ロール R2 に足す(BR7.1)。例の検査で許す IAM の操作の始まりを `Get`、`List`、`Describe` とし、例外として `config:SelectResourceConfig` と `resource-explorer-2:Search` の2つだけを名前で許す。それ以外の `Select` や `Search` で始まる操作は誤りにする。コードの操作の表は、API の操作と IAM の操作の対応を持ち、例と照らすのは IAM の操作の名前 | U1 の例の検査(`*` を含まない、コードの操作の表と一致、R2 と一致)が、足した操作を含めて通る。例外の扱いを確かめるテスト(許す2つ、誤りにするもの1つずつ)が通る |
| NFR3.4 | scan と plan は、上の表の API の操作だけを呼び、AWS Config や Resource Explorer の設定を変える操作(記録の開始、索引やビューの作成など)と、API の `Search` を呼ばない。使えないときは、使えないことと理由を示すだけにする(BR3.2) | HTTP の層の偽の AWS で、記録されたすべての操作がコードの操作の表に含まれることを確かめるテスト(U1 の NFR3.1 の仕組み)が通る。偽の AWS は、AWS Config の要求では操作を指す見出し(`X-Amz-Target`)から、Resource Explorer の要求では URL のパス(`/ListResources` など)から、操作の名前を取り出す |
| NFR3.5 | 検証用アカウントで、新しく足した API の操作がすべて、例だけを付けたロールで少なくとも1回成功する | U2 のチェックポイントの前に、人が検証用アカウントで Resource Explorer の索引と既定のビューを作る(費用はかからない)。AWS Config は記録を有効にしない(費用がかかるため)。そのため Config の3つの操作は、記録がない状態での成功の応答(記録なし、問い合わせの結果0件)を確かめる。いずれかの操作が記録なしの状態では呼べないと分かったときは、その操作の実機の確認をチェックポイントの記録に「未確認」と残し、偽の AWS だけで確かめたことを明記する |
| NFR8.9 | AWS Config と Resource Explorer の応答から作る文字列(ARN、種類の名前、リージョン、検出できなかった理由)と、進み具合の行に出す文字列は、U1 の NFR の設計 `security-design.md` §4 の置き換え(Unicode の制御文字 Cc(C1 の 0x80〜0x9F を含む)、書式の文字 Cf(双方向の制御を含む)、行と段落の区切り Zl・Zp、不正な UTF-8、バックスラッシュ)を通してから出す | 0x9B、U+202E、改行、不正な UTF-8 を含む種類名と ARN を返す偽の AWS のテストで、標準出力と標準エラーにそのまま含まれない |
| NFR8.10 | 検出できなかった理由やリージョンの失敗の理由には、エラーの種類と操作の名前だけを出し、AWS の SDK のエラー文は出さない(U1 の NFR8.1) | 偽の認証情報と、拒否・時間切れ・想定外の応答を返す Config と Resource Explorer の偽の AWS で、認証情報の文字列とエラー文が出力に含まれない |
| NFR8.11 | 収集の途中で認証情報の期限が切れたときは、それまでに読んだ結果を標準出力に出さずに終える。部分的な結果を、完全な結果と取り違えさせないため(信頼性の NFR5.8 と同じ振る舞いを、セキュリティの面から求める)。期限切れとして扱う AWS のエラーの種類の一覧(タグの取得、Config、Resource Explorer、IAM のそれぞれ)は、NFR の設計で AWS のドキュメントから決め、コードの表として持つ | NFR5.8 のテストで確かめる。テストは一覧のエラーの種類をすべて使う |

## 脅威と対策

| 脅威 | 対策 |
|---|---|
| U2 で増えた操作のために、利用者が広すぎる権限を渡す | NFR3.3(操作の名前を固定し、例のポリシーを最小に保つ) |
| ツールが Config や Resource Explorer を勝手に有効にして、費用や設定の変化を起こす | NFR3.4 |
| 権限の不足に、検証で気付かない | NFR3.5 |
| 他人が付けたリソース名やタグに仕込んだ制御文字で、端末の表示を書き換える | NFR8.9 |
| エラー文から認証情報や内部の情報が漏れる | NFR8.10 |
| 途中で止まった結果を、利用者が完全な結果と思い込む | NFR8.11 |
