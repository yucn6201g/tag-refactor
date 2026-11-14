# 信頼性の設計: U1 最小の一本

要件は `construction/u1-walking-skeleton/nfr-requirements/reliability-requirements.md` と `performance-requirements.md`(NFR3.x、NFR4.x、NFR5.x、NFR6.1、NFR7〜NFR9)、流れは `construction/u1-walking-skeleton/functional-design/functional-spec.md` です。出典の ND-Q… は、このステージの `nfr-design-questions.md` の回答です。

この道具は利用者が手で動かす CLI なので、再試行、サーキットブレーカー、フェイルオーバーは置きません。外のプログラムの失敗は、その場で止めて理由を示し、利用者がやり直します。

## 1. 外のプログラムの起動と停止(NFR3.1〜NFR3.3、NFR3.10、ND-Q3)

外のプログラムは、`internal/procexec`(新しい小さなパッケージ。`internal/cdk` と `internal/pullrequest` が使う)で起動します。起動の仕方は、端末とやり取りしないもの(非対話)と、端末とやり取りするもの(対話)の2つに分けます。

| 起動の仕方 | 使うもの | 標準入力 | 標準出力、標準エラー | プロセスのまとめ方 | 止め方 |
|---|---|---|---|---|---|
| 非対話(`Run`) | cdk synth、補助のプログラム、読むだけの git、git switch -c、git add、gh | 空か、要求の JSON | 取り込む(それぞれ 64 KiB まで。超えた分は捨て、捨てたことを --debug に出す) | Unix は新しいプロセスのグループ(Setpgid)。Windows はジョブオブジェクト | グループ全体に SIGKILL、またはジョブオブジェクトを閉じる |
| 対話(`RunInteractive`) | git commit、git push | 端末 | 標準出力は読み捨て、標準エラーは端末 | Unix は tag-refactor と同じ前面のプロセスのグループのまま(端末を読んでも SIGTTIN で止まらない)。Windows はジョブオブジェクト | Unix は git のプロセスだけに SIGKILL(フックが起動した孫は残りうる。受け入れる残るリスク)。Windows はジョブオブジェクトを閉じる |

共通の手順は次のとおりです。

1. 文脈(context)に上限を付けて `exec.CommandContext` で起動する。シェルは通さない。
2. Windows では、子のプロセスを止めた状態で作り(`SysProcAttr.CreationFlags` に `CREATE_SUSPENDED`)、ジョブオブジェクト(閉じると中のプロセスをすべて止める設定)に入れてから、`ntdll.dll` の `NtResumeProcess` で動かし始める。入れる前に孫が作られる隙を無くすため。どれも `golang.org/x/sys/windows`(今の依存)で呼べる。
3. `cmd.Cancel` に上の止め方を入れ、`cmd.WaitDelay` を 5 秒にする。止めたあと、出力のパイプを待ち続けない。
4. 上限で止まったら、`errkind` の timeout で返す。

テストは、終わらない差し替えのプログラム(テストのバイナリ自身を子として動かし、孫を起動して眠るもの)で、上限のあと 10 秒の中で戻り(WaitDelay の 5 秒に余裕を足した値)、非対話では孫も残らないことを3つの OS で確かめます。対話の起動は、Unix でプロセスのグループを変えていないことをテストで確かめます。

## 2. 後片付け(NFR4.3)

| 一時的なもの | 作る場所 | 消す場所 |
|---|---|---|
| synth の出力先 | `CdkSynthesizer.Synthesize` の始め | 同じ関数の `defer` |
| 認証情報の遮断の空のファイル | 環境を作る関数 | 呼び出し側の `defer` |
| 補助のプログラムの書き出し先 | `CdkSourceAnalyzer` の呼び出しの始め | 同じ関数の `defer` |
| プルリクエストの本文のファイル | `Publish` の create の段階の前 | 成功したら消す。失敗したら残して場所を示す |

Ctrl+C の中断では、今の `iac` と同じく文脈が取り消され、上の `defer` がすべて走ります。テストは、取り消した文脈で呼び、一時的なフォルダーが残らないことを確かめます。

## 3. 書き出しの失敗と作成(NFR4.1、NFR4.2、NFR4.4)

- 書き出しは今の `iac` の `write`(一時ファイルに書いて名前を変える、`fileout.Replace`)をそのまま使う。
- `internal/cli` は、`Result.WriteErr` か `Result.Interrupted` があれば `Publish` を呼ばない。`Result.WrittenFiles` が空でも呼ばない。
- `Publish` の段階(branch、commit、push、create)は順に行い、失敗した段階で止める。済んだ段階を `PublishOutcome.CompletedSteps` に入れ、元に戻さない。

## 4. 今の振る舞いを壊さない(NFR5.1、NFR5.2、NFR3.6)

- 変更案に `aws:cloudformation:stack-name` のタグのリソースが無いときは、`internal/cdk` を一度も呼ばない。テストで、差し替えた実行の口の起動の回数が 0 であることを確かめる。
- 前の intent の `iac` の受け入れ条件のテスト(`internal/cli` の acceptance_u8 など)は変えずにそのまま通す。

## 5. テストの仕組み(NFR7.1、NFR8.1、NFR8.2、NFR9.1、ND-Q4)

- 外のプログラムの呼び出しは、`internal/procexec` の実行の口(インタフェース)を差し替えてテストする。カバレッジの計測から外さない。
- 実際の `cdk synth` と node を動かすテストは、テスト用のアプリ `internal/cdk/testdata/apps/ts-minimal` を一時的なフォルダーに写して使う。`node_modules` は写さず、写した先から元の `node_modules` を指す。Unix では `os.Symlink`、Windows では `cmd /c mklink /J`(ディレクトリのジャンクション。管理者の権限が要らない)で作る。後片付けでは、`os.Lstat` でリンクかジャンクションであることを確かめてから `os.Remove` でリンクだけを消し、そのあとで写しのフォルダーを消す。リンクの先(元の `node_modules`)を消す `os.RemoveAll` は、リンクを消す前には呼ばない。
- 道具が無いときは、環境変数 `CI` が空でなければ `t.Fatal`、空なら `t.Skip` に理由を書く。
- ベンチマークのテストは、テスト用のアプリを 200 ファイルに増やした写しで analyze と recheck の時間を測って記録し、60 秒(目安の2倍)を超えたら失敗にする。要件 NFR3.4 は「記録だけ」としていたが、このステージの人の回答(ND-Q4)で、目安の2倍を失敗の線にすることに改めた。
- CI の仕事の上限(timeout-minutes)は 30 分にする。CI の時間の目安(10 分)は合否にしない。
- 実際の GitHub の結合テストの片付けは `t.Cleanup` で行い、失敗したら残したプルリクエストの番号とブランチの名前を出して失敗にする(トークンは出さない)。
