# 規模の要件: U1 最小の一本

元になる要件は `inception/requirements-analysis/requirements.md` の NFR3 と NFR5、U1 の流れは `construction/u1-walking-skeleton/functional-design/functional-spec.md` です。出典の NQ-Q… は、このステージの `nfr-requirements-questions.md` の回答です。

この道具は利用者の端末で1回ずつ動く CLI なので、同時に多くの利用者を受ける意味での規模はありません。ここで扱うのは、1回の実行で扱う量です。

| ID | 要件 | 合否の基準 | 出典 |
|---|---|---|---|
| NFR3.8 | U1 で扱う CDK のアプリは、`--dir` の下の1つ。2つ以上あるときの扱い(すべてを対象にする、絞り込む)は U3 で足す。U1 では、2つ以上見つかったら、最初の1つ(フォルダーの名前の順)だけを synth し、残りを外して理由を示す | 2つのアプリがあるテストで、1つだけが synth され、もう1つが外したと示される | NFR3、unit-of-work.md の U1 |
| NFR3.9 | 補助のプログラムが1回に扱うファイルの目安は 200 個まで(NFR3.4)。それを超えても上限(2 分)までは動かす | ベンチマークのテスト(記録だけ) | NQ-Q1 |
| NFR5.1 | 変更案のリソースの数と CloudFormation、Terraform のファイルの数についての今の `iac` の振る舞い(並列の読み取り、メモリの予算)は変えない | 前の intent の `iac` のテスト(規模のテストを含む)が、すべてそのまま通る | NFR5 |
