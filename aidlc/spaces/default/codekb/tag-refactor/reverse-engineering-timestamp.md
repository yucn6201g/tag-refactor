# Reverse Engineering Timestamp

## Run Record

- Date: 2026-11-07T15:37:08Z
- Commit: 9bf1fb6b83d5745e197116eb7a6392043493e1dd

この実行は、リポジトリ全体を対象にした初めての調査です(既存のコード知識ベースなし)。意図 `iac-cdk-pr`(iac への CDK の反映とプルリクエストの作成)に合わせ、`internal/iac/`、`internal/cli/`、`cmd/tag-refactor/`、`internal/planner/` の変更案の形式を深く読み、ほかのパッケージは説明のコメントと公開の API の単位で読みました(標準の深さ)。下の `shallow.paths` は、読み方が浅かった場所を人のために記したものです。

## 調べなかったもの

- `.gitignore` が無視するビルドとテストの出力は開いていません。
- AI-DLC の導入物と作業領域は調査の対象外です。
- カバレッジの値はこの調査では測っていません(CI の `coverage` のジョブが 80% の下限を確かめます)。

## Scope of Analysis

```yaml
scope_version: 1
kind: full
intent: iac-cdk-pr
fingerprint: b7e7ac757712f0b0e229956ac451b29119316002
analyzed:
  paths:
    - ./
  components:
    - cmd/tag-refactor
    - internal/cli
    - internal/iac
    - internal/planner
    - internal/impact
    - internal/inventory
    - internal/rules
    - internal/apply
    - internal/journal
    - internal/awsaccess
    - internal/display
    - internal/fileout
    - internal/errkind
    - internal/restype
    - internal/testutil
    - scripts
    - test/sandbox
    - docs/iam
    - .github
shallow:
  paths:
    - internal/apply/
    - internal/journal/
    - internal/awsaccess/
    - internal/inventory/
    - internal/impact/
    - internal/rules/
    - internal/display/
    - internal/testutil/
    - scripts/
    - test/sandbox/
    - docs/iam/
```
