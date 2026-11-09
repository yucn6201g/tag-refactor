# 規模の設計: U1 最小の一本

要件は `construction/u1-walking-skeleton/nfr-requirements/scalability-requirements.md`(NFR3.8、NFR3.9、NFR5.1)です。この道具は利用者の端末で1回ずつ動く CLI なので、横に増やす、負荷を分ける、データを分割する仕組みは要りません。ここでは、1回の実行で扱う量の設計だけを書きます。

| 要件 | 設計 |
|---|---|
| NFR3.8 U1 のアプリは1つ | `Discover` が見つけたアプリを、`--dir` からの相対のパスの名前の順に並べ、最初の1つだけを synth する。残りは `AppSkipped`(理由 `notInU1`。U3 で「複数のアプリ」を足すときに消す)で示し、そのアプリのリソースは結び付かなければ CloudFormation のテンプレートの探索に進む |
| NFR3.9 補助のプログラムのファイルの目安 | 補助のプログラムは、アプリの `tsconfig.json` の `include` と、cdk.json の app が名指すファイルから、`node_modules` と `cdk.out` を除いた `.ts` のファイルだけを読む。上限(2 分)の中で動かす |
| NFR5.1 今の iac の規模の扱い | 今の並列の読み取り(`forEach`)とメモリの予算(`budget`)を、CDK の候補のファイルにもそのまま使う。CDK の候補は補助のプログラムが名指したファイルだけなので、`walkRoot` の対象は増えない |
