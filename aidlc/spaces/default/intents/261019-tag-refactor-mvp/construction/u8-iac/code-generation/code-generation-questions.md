# Code Generation Plan Approval

AI-DLC writes this file when it asks you to approve the plan. To answer here
instead of in chat, write your answer after `[Answer]:` and say done.

## Plan Approval

Approve the code plan for u8-iac?

- Builds: `iac`(変更案を CloudFormation の YAML・JSON と Terraform の `.tf` に、変える範囲だけ差し替えて反映する。既定は差分だけ、`--write` は git の確かめのあと)、種類の表 `internal/restype`、`fileout` の置き換えとやり直しの部品、`verify.sh` の U8 の確かめと CI の2つの変更
- Touches: `internal/iac/`(新しい)、`internal/restype/`(新しい)、`internal/fileout/`、`internal/journal/`、`internal/cli/`、`internal/display/`、`internal/testutil/`、`internal/rules/rules_test.go`、`test/sandbox/...
- Tests: 受け入れ条件のテスト(AC7.1.1、AC7.1.2 と、差分だけ・`--write`・終了コード・ヘルプ。先に書く)、部品ごとの単体テスト(iac の各読み取り、Edit、引用符、差分、git、上限と深さ、書き出し、restype、fileout、display、cli で各5〜8件以上)、大きなテスト(1万リソース・2,000ファイル)、U1〜U7 の回帰。カバレッジ80%以上

Full plan: aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u8-iac/code-generation/code-generation-plan.md
Test instructions: aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u8-iac/code-generation/unit-test-instructions.md

[Approval Fingerprint]: sha256:v3:5c1669cb7c33af833af18199facd291a12dc2217f1600aadd7f7ba55971363f8
[Planned Source]: 21b38b026ee2272765120c5f1536d17d4f295a61d08abfe05b8f03cbc7dfae1a

- A. Approve Plan
- B. Request Changes
- C. I'll edit the files

[Answer]: A. Approve Plan
