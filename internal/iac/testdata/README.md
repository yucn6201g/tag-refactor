# テストのデータ

- `sandbox-plan.json`: AWS を呼ばずに手で作った変更案。確認用のスタック `tag-refactor-u6-fixtures` の論理 ID とスタック名のタグを持つ。CI の `lint` のジョブが、`test/sandbox/` の写しへの反映に使う。
- `terraform-show-example.json`: `terraform show -json` の形の例。**手で作ったもので、実物の出力ではない**(手元に terraform がない)。
  - 形は Terraform の公式の文書「JSON Output Format」(https://developer.hashicorp.com/terraform/internals/json-format 、v1.17.x の版を 2026-11-05 に確かめた)に合わせた: `format_version`、`terraform_version`、`values.root_module.resources` と `child_modules`、各リソースの `address`・`mode`・`type`・`name`・`provider_name`・`schema_version`・`values`・`sensitive_values`。
  - **各種類の `values.arn` は確かめていない。** Terraform Registry の AWS プロバイダーの文書は取得できず、`internal/restype` の表の種類ごとに `arn` を置いたのは、この作業の前提(種類が ARN の属性を持つ)によるもの。実物の出力が取れるようになったら置き換える。
