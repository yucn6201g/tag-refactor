# 性能の要件: U1 最小の一本

元になる要件は `inception/requirements-analysis/requirements.md` の NFR3(時間の上限)と FR2.5、契約は `inception/contract-design/contract-summary.md` の C5 の timeouts です。出典の NQ-Q… は、このステージの `nfr-requirements-questions.md` の回答です。U1 の流れは `construction/u1-walking-skeleton/functional-design/functional-spec.md` にあります。

この道具は利用者が手で動かすもので、速さは「待てる長さで終わり、止まったままにならない」ことが大事です。応答時間の SLA は置きません。

## 要件

| ID | 要件 | 合否の基準 | 出典 |
|---|---|---|---|
| NFR3.1 | `cdk synth` の上限は 10 分。超えたら子のプロセスを止め、そのアプリを timeout の失敗にする | 終わらない差し替えの `cdk` で、上限(テストでは短くする)のあとに止まり、子のプロセスが残らない | NFR3、FR2.5 |
| NFR3.2 | 補助のプログラムの上限は、呼び出し(analyze、recheck)ごとに 2 分 | 終わらない差し替えの `node` で、上限のあとに止まる | NFR3、契約 C5 |
| NFR3.3 | `git` と `gh` の上限は、読むだけの `git` と `switch -c`、`add` が 30 秒、`commit` と `push` が 10 分、`gh` が 2 分 | 応答しない差し替えの `git` と `gh` で、上限のあとに止まる | NFR3、契約 C5 |
| NFR3.4 | 補助のプログラムの目安: TypeScript のファイルが 200 個までのアプリで、analyze と recheck が CI の機械でそれぞれ 30 秒以内に終わる | テスト用のアプリを 200 ファイルに増やしたベンチマークのテストで、時間を記録する(CI では記録だけで、合否にはしない) | NQ-Q1 |
| NFR3.5 | 補助のプログラムは、アプリごとに analyze を1回、書き換えがあれば recheck を1回だけ起動する(ファイルごとに起動しない) | テストで、起動の回数を数える | 契約 C4、FD-Q1 |
| NFR3.6 | CDK のアプリが無いとき、または変更案に CloudFormation のリソースが無いときは、synth も補助のプログラムも起動せず、今の `iac` と同じ時間で終わる | テストで、起動の回数が 0 であることを確かめる | NFR5、BR1.1 |
| NFR3.7 | CDK のテストで増える CI の時間は、OS ごとに 10 分以内を目安にする。npm と Go のキャッシュを使う | CI の仕事の時間を見て確かめる(合否にはしない) | NQ-Q5 |

## 資源

- メモリ: 今の `iac` のソフトなメモリの上限と解析の予算(`internal/iac` の budget)をそのまま使います。補助のプログラムの結果は、アプリごとに1つの JSON で、ソースの中身を含みません(キーと値の文字列だけ)。
- ディスク: synth の出力は一時的なフォルダーに置き、読み終えたら消します。
