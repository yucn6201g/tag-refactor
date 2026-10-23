# セキュリティの設計: U2 収集

U1 のセキュリティの設計(`construction/u1-walking-skeleton/nfr-design/security-design.md` の §1〜§7)は、U2 でもそのまま使います。ここでは、U2 で足す AWS の操作と、新しく読む応答についての設計を足します。

出典: `nfr-requirements/security-requirements.md`(NFR3.3〜NFR3.5、NFR8.9〜NFR8.11)、`nfr-requirements/tech-stack-decisions.md`、`nfr-design-questions.md` Q3・Q4、機能設計 `functional-spec.md`(BR3.1、BR3.2、BR7.1)、契約 C6・C12、AWS のドキュメント(`resource-explorer/latest/apireference/API_ListResources.html`、`config/latest/APIReference/API_SelectResourceConfig.html`、`config/latest/APIReference/API_RecordingGroup.html`、`config/latest/developerguide/querying-AWS-resources.html`)。

## 1. 操作の表を「API の操作」と「IAM の操作」に分ける(NFR3.3、NFR3.4)

U1 の操作の表(`internal/awsaccess` の `Operations`)は、1つの名前を API の操作と IAM の操作の両方に使っていました。U2 では両者が食い違う操作(`ListResources` には `resource-explorer-2:Search` が要る)があるため、表の1行に2つの名前を持たせます。

```go
type Operation struct {
    API             string // 呼ぶ API。例 "resource-explorer-2:ListResources"
    IAMAction       string // 必要な IAM の操作。例 "resource-explorer-2:Search"
    NeedsPermission bool
}
```

U2 で表に足す行:

| API(偽の AWS が記録する名前) | IAMAction(例のポリシーに書く名前) |
|---|---|
| `config:DescribeConfigurationRecorders` | `config:DescribeConfigurationRecorders` |
| `config:DescribeConfigurationRecorderStatus` | `config:DescribeConfigurationRecorderStatus` |
| `config:SelectResourceConfig` | `config:SelectResourceConfig` |
| `resource-explorer-2:ListIndexes` | `resource-explorer-2:ListIndexes` |
| `resource-explorer-2:GetDefaultView` | `resource-explorer-2:GetDefaultView` |
| `resource-explorer-2:GetView` | `resource-explorer-2:GetView` |
| `resource-explorer-2:ListResources` | `resource-explorer-2:Search` |

- `GetView` は、承認済みの NFR 要件の表(NFR3.3)と機能設計 BR7.1 の6つの操作にない操作で、ここで足す。既定のビューに絞り込みの条件(`Filters`)が付いていると、`ListResources` の結果が一部の種類に限られる。そこで既定のビューの条件を読み、条件があればそのリージョンの検出を `partial` の扱いにする(`reliability-design.md` §3)。上流の文書は、この設計と同時に直す(§5)。
- `ListResources` 以外の IAM の操作の名前が API と同じであることは、コードの生成で Service Authorization Reference と照らして確かめる。違っていれば表を直す。表を直せば、下のテストで例のポリシーとの食い違いが分かる。

### 守らせる仕組み

| テスト | 確かめること |
|---|---|
| 例のポリシーと表の照合(U1 のテストを変える) | 例の `docs/iam/readonly-policy.json` の操作の集まりが、表の `NeedsPermission` の行の `IAMAction` の集まりと一致する。確認用のテンプレートの読み取り用ロール R2 の操作も同じ集まりになる |
| 読み取りだけの検査(U1 のテストを変える) | 例の IAM の操作が `Get`、`List`、`Describe` で始まるか、名前で許す2つ(`config:SelectResourceConfig`、`resource-explorer-2:Search`)のどちらかである。`*` を含まない。誤りにする例として `config:SelectAggregateResourceConfig` と `resource-explorer-2:SearchViews`(仮の名前)をテストの入力に入れ、検査が誤りにすることを確かめる |
| 呼んだ操作の照合(NFR3.4) | 偽の AWS が記録した API の名前が、すべて表の `API` の列にある。API の `resource-explorer-2:Search` を記録したら、それだけでテストを失敗させる |

`internal/testutil` の `IsReadOperation`(API の名前を見る)は、`Get`、`List`、`Describe` に加えて `config:SelectResourceConfig` だけを名前で許すように変える。

### 偽の AWS が操作の名前を取り出す方法(NFR3.4)

| サービス | 要求の形 | 操作の名前の取り出し方 |
|---|---|---|
| タグの取得、AWS Config | JSON 1.1。見出し `X-Amz-Target` がある | 見出しの最後の `.` より後(U1 のまま) |
| EC2、IAM | クエリーの形。本文に `Action=` | 本文の `Action`(U1 のまま) |
| Resource Explorer | REST と JSON。`POST /ListResources` のように URL のパスで操作が決まる | 見出しも `Action` もないとき、URL のパスの最後の部分 |

サービスの名前は、U1 と同じく署名の範囲(`resource-explorer-2`、`config`)から取る。テストでは、3つの形それぞれで操作の名前が取り出せることを確かめる。

## 2. 検証用アカウントでの確認(NFR3.5)

- 確認用のテンプレート(`test/sandbox/`)に、Resource Explorer の索引と既定のビューを足す(Q4)。中身は Infrastructure Design で決める。読み取り用ロール R2 には、§1 の表の IAM の操作を足す。
- AWS Config の記録は有効にしない(費用がかかるため)。Config の3つの操作は、記録がない状態で次を確かめる。
  - `DescribeConfigurationRecorders`、`DescribeConfigurationRecorderStatus`: 成功し、記録がないという応答になる。
  - `SelectResourceConfig`: ツールが使う問い合わせの文で呼び、AccessDenied にも、問い合わせの文の誤り(`InvalidExpressionException`)にもならない。これで、IAM の権限と、問い合わせの文が AWS に受け付けられることの両方を確かめる。
- 実機で呼べなかった操作があれば、チェックポイントの記録に「未確認」と書き、偽の AWS だけで確かめたことを明記する。
- 結合テストは、U1 と同じくビルドタグ `integration` と、プロファイル名の環境変数があるときだけ動き、CI では動かない。

## 3. AWS の応答を境目で確かめる(NFR8.9、NFR8.10)

| 応答 | 確かめ方 |
|---|---|
| Config の問い合わせの結果 | 1件ずつ JSON の文字列で返る。JSON として読めない、または `resourceType` がないものが1件でもあれば、そのリージョンの Config の経路を「失敗」(想定外の応答)にして、BR3.1 の次の方法に進む。1件の大きさの上限は 256 KiB とし、超えたら同じく失敗にする |
| Config の `arn` | ない、または `arn:` で始まらないものは、ARN のないリソースとして数える(BR3.3) |
| Config の `tags` | キーと値の組の配列として読む。`aws:` で始まるキーは利用者のタグとして数えない |
| Resource Explorer の `Arn`、`ResourceType`、`Region` | U1 のタグの取得の応答と同じく、文字列として受け取り、表示の直前に置き換える |
| Resource Explorer の既定のビューの ARN | 応答の ARN を、次の `GetView` にそのまま渡す。利用者の入力からは作らない |

- 置き換え: Config と Resource Explorer の応答から作る文字列(ARN、種類の名前、リージョン、検出できなかった理由)と、進み具合の行は、すべて `internal/display` の出力の口を通る。U1 の置き換え(Cc、C1、Cf、Zl、Zp、不正な UTF-8、バックスラッシュ)がかかる。U1 の「`display` と `cmd` の外から標準出力と標準エラーを使っていない」の検査も、そのまま U2 のコードに効く。
- エラー文を出さない: 新しい操作のエラーも、`internal/awsaccess` の `classify` で種類つきのエラーに変える。利用者に出すのは種類、操作の名前、リージョンだけ。AccessDenied のときは、利用者が例のポリシーを直せるよう、表の `IAMAction` の名前を出す(例 `AccessDenied resource-explorer-2:Search`)。`--debug` には `API` の名前を出す。
- テスト: 0x9B、U+202E、改行、不正な UTF-8 を含む ARN と種類の名前を返す偽の Config と Resource Explorer で、標準出力と標準エラーにそれらがそのまま出ないことを確かめる。見分けやすい偽の認証情報を使い、拒否、時間切れ、想定外の応答で、認証情報と SDK のエラー文が出ないことを確かめる。

## 4. 認証情報の期限切れで、部分的な結果を出さない(NFR8.11)

- 期限切れとして扱うエラーの種類は、`internal/awsaccess` の `errorCodes` の表に持つ。U1 の表(`ExpiredToken`、`ExpiredTokenException`、`RequestExpired`、`TokenRefreshRequired` と、拒否の `InvalidClientTokenId`、`UnrecognizedClientException` など)は、タグの取得、IAM、AWS Config、Resource Explorer に共通の名前を含む。コードの生成で、各サービスの API のドキュメントの共通のエラーと照らし、足りない名前があれば足す。
- 呼び出しの途中で SDK が認証情報を取り直して失敗したとき(SSO の期限切れなど)も、`CredentialsMissing` にする。どの SDK のエラーの型で分かるかは、コードの生成で確かめてテストにする。
- Resource Explorer の `ListResources` が返す 401(`UnauthorizedException`)は、認証情報の誤りではない。AWS のドキュメントでは、既定のビューがない場合と、ビューを使う権限がない場合の両方で返る。この名前は共通の表に入れず(入れると、ビューのないリージョンが1つあるだけで全体が止まるため)、Resource Explorer の読み取りの中で次のように分ける。
  - `GetDefaultView` の `ViewArn` が空: 「既定のビューがない」(種類 `AccessDenied`、理由 `no-default-view`、契約 C12 の理由への追加)。`ListResources` は呼ばない。
  - `ViewArn` はあるのに `ListResources` が 401: 「既定のビューを使えない」(種類 `AccessDenied`、理由なし)。表示では IAM の操作の名前 `resource-explorer-2:Search` を添え、権限の不足の可能性を示す。
  - 2つの場合で理由の文言が分かれることを、偽の AWS のテストで確かめる。
- 部分的な結果を出さない仕組みは `reliability-design.md` §4。テストは、上の表の期限切れの名前をすべて、3つの段階(タグの取得、検出、plan の IAM の読み取り)で返して確かめる。

## 5. 上流と下流への引き継ぎ(`GetView` を足すこと)

`GetView` を足すことは、承認済みの上流の文書への変更になる。この設計と同時に、次の場所を直す。

| 場所 | 直すこと | 担当 |
|---|---|---|
| NFR 要件 `security-requirements.md` の「U2 で新しく呼ぶ操作」の表 | `GetView` / `resource-explorer-2:GetView` の行を足す | この設計の段階(同時に直す) |
| 機能設計 `rules.md` の BR7.1 | 足す IAM の操作に `resource-explorer-2:GetView` を足す | この設計の段階(同時に直す) |
| 機能設計 `functional-spec.md` の操作の表 | Resource Explorer のビューの行に `GetView` を足す | この設計の段階(同時に直す) |
| 例のポリシー `docs/iam/readonly-policy.json`、`internal/awsaccess` の操作の表 | 7つの操作(§1 の表)を足す | コードの生成 |
| 確認用のテンプレートの読み取り用ロール R2 | 7つの IAM の操作を足す | Infrastructure Design |

§1 のテスト(例のポリシー、R2、操作の表の照合)が、どこか1か所の直し忘れを見つける。

## 6. 依存を足すときの確かめ(U1 の §6 に足す)

- 足す依存は `service/configservice`、`service/resourceexplorer2`、`golang.org/x/sync`、`golang.org/x/term` に限る(`tech-stack-decisions.md`)。いずれも通信の先は AWS の API だけで、U1 の §7 の通信の範囲を変えない。
- `govulncheck`、`go mod verify`、`go mod tidy` の差分の確認、Dependabot の対象に入ることは、U1 の CI のまま。
