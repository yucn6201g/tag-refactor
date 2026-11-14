# 性能の設計: U1 最小の一本

要件は `construction/u1-walking-skeleton/nfr-requirements/performance-requirements.md`(NFR3.1〜NFR3.7)です。出典の ND-Q… は、このステージの `nfr-design-questions.md` の回答です。

キャッシュ、接続のプール、非同期の処理は置きません。速さは、起動の回数を減らすことと、時間の上限で決まります。

| 要件 | 設計 |
|---|---|
| NFR3.1 synth の上限 10 分 | `internal/procexec` の上限つきの起動(reliability-design.md の1節)。値は `cdk.Options.Timeout`(U1 は既定値の 10 分だけ、`--synth-timeout` は U3) |
| NFR3.2 補助のプログラムの上限 2 分 | 同じ起動の口で、呼び出しごとに 2 分 |
| NFR3.3 git と gh の上限 | 同じ起動の口で、段階ごとの値を `internal/pullrequest` の定数に置く(30 秒、10 分、2 分) |
| NFR3.4 補助のプログラムの目安 | analyze はアプリのソースを1回だけ読み、ファイルごとに `createSourceFile` を1回だけ呼ぶ。型の検査(`createProgram`)はしない。recheck は書き換えたファイルだけを読み直し、ほかのファイルは analyze の結果を使う。ベンチマークのテストで 60 秒を超えたら失敗(ND-Q4) |
| NFR3.5 アプリごとに1回の起動 | `CdkSourceAnalyzer` は、アプリの対象のコンストラクトのパスをまとめて1つの要求にする。recheck も書き換えたファイルをまとめて1回にする。テストで起動の回数を数える |
| NFR3.6 CDK が無いときは起動しない | 照合の前に、変更案に stack-name のタグのリソースがあるかを見る(BR1.1) |
| NFR3.7 CI の時間 | `actions/setup-node` の npm のキャッシュと、今の Go のキャッシュを使う。CDK の synth を動かすテストは、ほかのテストと並べて動かせるよう `t.Parallel` にし、1つのアプリの synth の結果をテストの中で使い回す |

## 時間の配分(U1 のデモのアプリ)

| 段階 | 目安 |
|---|---|
| `cdk synth`(ts-node でアプリを動かす) | 20〜60 秒 |
| 補助のプログラムの analyze | 2 秒以内 |
| 書き換えと recheck | 2 秒以内 |
| git と gh(ブランチから作成まで) | 10 秒前後(ネットワークによる) |
