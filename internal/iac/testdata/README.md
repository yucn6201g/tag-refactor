# テストのデータ

- `sandbox-plan.json`: AWS を呼ばずに手で作った変更案。確認用のスタック `tag-refactor-u6-fixtures` の論理 ID とスタック名のタグを持つ。CI の `lint` のジョブが、`test/sandbox/` の写しへの反映に使う。
- `terraform-show-example.json`: `terraform show -json` の形の例。**手元に terraform がないので、実物の出力ではない。** 形は Terraform の公式の文書「JSON Output Format」の State Representation(`format_version`、`terraform_version`、`values.root_module.resources`、`child_modules`、各リソースの `address`・`mode`・`type`・`name`・`provider_name`・`schema_version`・`values`・`sensitive_values`)に合わせ、中のリソースは `internal/restype` の表の Terraform の種類ごとに1つずつ置いた。`values.arn` を持つことは、Terraform Registry の AWS プロバイダーの文書の各リソースの「Attribute Reference」の `arn` に合わせた(文書の版: 2026-10 時点の hashicorp/aws v6 系。実物で確かめたものではないので、実物が取れるようになったら置き換える)。
