# コードの要約: U3 CDK の synth と対応づけ

## 作ったもの

- `--dir` の下のすべての CDK のアプリを見つけ(`node_modules`、`cdk.out`、`.venv`、`venv` に入らない)、名前の順に1つずつ synth する。`--cdk-app` で絞り込み、`--synth-timeout`(`time.ParseDuration`)で1回の synth の上限を変える。
- Python のアプリ: アプリの直下の `.venv` の Python を先に使い(`PATH` の先頭と `VIRTUAL_ENV`、`--app` の置き換え)、synth の前に `--version` で版を確かめる(上限 30 秒)。古ければ新しい理由 `toolTooOld`、読めなければ `toolMissing`。
- 一部のアプリが失敗しても、ほかのアプリと Terraform、CloudFormation の処理を続ける。どのアプリのものか分からないリソースは、synth に失敗したアプリがちょうど1つのときだけ、その道具の名前で理由を示す。
- 作成の記録(`cdk synth --debug`)での補い: 補助のプログラム `ts-helper.js` が `from: "trace"` の箇所を返し、Recheck にも同じ記録を渡す。
- 進み具合: 端末ではアプリごとの行を書き直す表と経過時間、端末でないときは状態が変わるたびと1分ごとに1行(`time.Ticker`)。`--debug` の行はホームのフォルダーを `~` にする。
- CI: `ci.yml` の test、race、coverage と `release.yml` の build に Python 3.13 の準備(`.venv`、`pip install --only-binary=:all:`、`pip check`、`npm ci --ignore-scripts`)。Dependabot に py-minimal の pip と npm。
- テスト用のアプリ8つ(`py-minimal`、`py-v1`、`ts-trace`、`ts-lookup`、`ts-profile`、`ts-v1`、`other-lang`、`unknown-version`)。`.gitignore` に `.venv` と `__pycache__`。
- README の `iac` の節と「開発」の節(py-minimal の `.venv` を作る3行)。契約の「Construction での直し」の表に 11 行。

## 主な判断

- **AppSkipped の送り手**: Synthesize(契約 C2 のまま)。外す判断と知らせを同じ `internal/cdk` に置き、二重送りと送り漏れを防ぐ。
- **`__pycache__`**: `.gitignore` に足す(git status の確かめは、書いたことを見つけるだけで止められないため)。
- **Python の最小の版は 3.10**: 設計は「見込み 3.9、Code Generation で py-minimal に固定した `aws-cdk-lib` の `Requires-Python` に合わせて決める」としていた。固定した `aws-cdk-lib` 2.276.0 は 3.10 以上を求めるので 3.10 にした(`TestMinPythonMatchesTheTestApp` で確かめる)。
- **`windowsQuotedApp` は false**: 手元の Windows で、引用符つきの `--app` が `cdk.cmd` に届かなかった。フォルダーの名前に空白などがある Python のアプリでは置き換えをせず、README に回避策(cdk.json の app を `.venv\Scripts\python.exe app.py` のように書く)を書いた。
- **作成の記録の形**: `additionalMetadataFile` の `aws:cdk:creationStack` の `data` にある。needsAWS の8つの文は直さなくて済んだ。

## テスト

- 受け入れ条件のテスト 14 本(`internal/cli/acceptance_v02u3_test.go`、`TestV02U3…`)を実装の前に書き、実装のあとで部品ごとの単体テスト(`internal/cdk` に新しく6つのファイル、`internal/iac/cdk_u3_test.go`、`internal/display/iaccdk_test.go` など)を足した。実際の `cdk synth` を動かすテスト(`realsynth_test.go`)は、CI の3つの OS で動く。
- 開発担当の報告: `GOTOOLCHAIN=local CGO_ENABLED=0 go test ./...` は 19 パッケージすべて通過。文のカバレッジは全体で 95.2%(cdk 93.7%、iac 94.8%、display 98.2%、cli 97.1%、testutil 90.6%)。`cmd/tag-refactor` は 71.4% で、この Unit では変えていない(全体の下限 80% は満たす)。gofmt、go vet、golangci-lint は問題なし。govulncheck は、呼ばれているモジュールの脆弱性 0 件(手元の go1.27.0 の標準ライブラリには、新しいパッチの版で直った指摘が残る)。
- 手元の Windows で飛ばしたもの: `TestQuotedArgReachesCmd`(`windowsQuotedApp` が false のため)、Unix だけのテスト(`TestHomeOfARealLink`、空白のフォルダーの `$` の場合)。`-race` は手元に cgo が無いので CI で確かめる。

## 計画からのずれ

1. 止め方のテストの時間の上限を 1〜2 秒から 8 秒にし、設計の `TAG_REFACTOR_CDK_CHILD` の代わりに testutil の偽物(`PythonHang`)を使った。全体を並べて動かすと、偽の Python が孫のプロセスを起こす前に 2 秒の上限が来て落ちたため。
2. `ProcessAlive` は procexec から移さず、同じ見方を testutil に足した(`alive_unix.go`、`alive_windows.go`)。procexec は変えない。
3. `requirements.txt` の `aws-cdk-lib @ https://…` の行を捨てていた不具合を単体テストで見つけて直した(`@` より前に `://` があるときだけ捨てる)。
4. 機能の設計の BR2.3 の「profile の指定だけで synth は失敗する」は実際と違ったので、rules.md の文を直した(lookup など AWS の値を引くときに失敗する)。テスト用のアプリ `ts-profile` は AWS の値を引く形にした。

## レビュー(1回目、READY)の Minor の扱い

- R-01(相対のパスの Python)、R-02(Windows で `--app` を置き換えないときの Python の食い違い)、R-04(README の profile の文)、R-07(TestQuotedArgReachesCmd を「届かない」ことの確かめに)、R-09(pyproject.toml の依存の読み方)、R-03(契約の本文を「Construction での直し」の表に合わせる)は、判定の記録のあとで直した。
- R-05: `source-manifest.json` はアプリのソースの一覧なので、`aidlc/` の下の契約と設計の文書は入れない(直した文書は「計画からのずれ」と契約の表に書いた)。
- R-06: 計画の「最初の確かめの結果」の BR2.3 の文は、承認した計画の本文なので直さず、ここ(計画からのずれ 4)を正とする。Step 17 はチェックした。
- R-08: 下の「残っていること」で確かめる。

## 残っていること

- 構成の検証コマンド(`test/verify/v02-u1.sh`、AWS の認証情報が要る)と、PR の CI(3つの OS の py-minimal、release の build、CI の時間)の確認。
- 公開の Go の下限(今は 1.27.3)を、標準ライブラリの直しが入った版に上げるかの判断(U3 の外)。
